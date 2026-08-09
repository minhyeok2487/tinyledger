package app

import (
	"database/sql"
	"errors"
	"log"
	"time"
)

var db *sql.DB

func initDB(path string) {
	var err error
	// WAL lets the dashboard's parallel reads proceed while a write is in
	// flight, and busy_timeout makes any remaining contention wait rather
	// than fail the query outright.
	db, err = sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		log.Fatal(err)
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	setupSchema()
}

// initTursoDB connects to a remote Turso (libSQL) database instead of a local file.
func initTursoDB(url, token string) {
	var err error
	db, err = sql.Open("libsql", url+"?authToken="+token)
	if err != nil {
		log.Fatal(err)
	}
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(8)
	db.SetConnMaxIdleTime(5 * time.Minute)
	db.SetConnMaxLifetime(30 * time.Minute)

	// setupSchema costs a dozen sequential round trips, which every serverless
	// cold start would otherwise pay before serving a byte. Check a stored
	// version first: one round trip when the schema is already current.
	if schemaCurrent() {
		return
	}
	setupSchema()
	recordSchemaVersion()
	log.Println("Turso DB 스키마 갱신:", schemaVersion)
}

// schemaVersion is bumped whenever setupSchema gains a table, column, or
// migration, so remote databases pick the change up on their next cold start.
const schemaVersion = 2

func schemaCurrent() bool {
	var v int
	// Errors here mean the table doesn't exist yet (pre-versioning database),
	// which is exactly the case that needs the full setup to run.
	if err := db.QueryRow(`SELECT COALESCE(MAX(version),0) FROM schema_meta`).Scan(&v); err != nil {
		return false
	}
	return v >= schemaVersion
}

func recordSchemaVersion() {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_meta (version INTEGER NOT NULL)`); err != nil {
		log.Println("schema_meta:", err)
		return
	}
	if _, err := db.Exec(`INSERT INTO schema_meta(version) VALUES (?)`, schemaVersion); err != nil {
		log.Println("schema_meta insert:", err)
	}
}

func setupSchema() {
	if _, err := db.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		log.Println("PRAGMA foreign_keys:", err)
	}

	stmts := []string{
		`CREATE TABLE IF NOT EXISTS accounts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			icon TEXT NOT NULL DEFAULT '💳',
			sort_order INTEGER NOT NULL DEFAULT 0,
			balance INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS transactions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			account_id INTEGER NOT NULL DEFAULT 1,
			date TEXT NOT NULL,
			type TEXT NOT NULL,
			category TEXT NOT NULL,
			amount INTEGER NOT NULL,
			memo TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_tx_date ON transactions(date)`,
		`CREATE INDEX IF NOT EXISTS idx_tx_account ON transactions(account_id)`,
		`CREATE TABLE IF NOT EXISTS budgets (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			month TEXT NOT NULL,
			category TEXT NOT NULL,
			amount INTEGER NOT NULL,
			UNIQUE(month, category)
		)`,
		`CREATE TABLE IF NOT EXISTS templates (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			account_id INTEGER NOT NULL DEFAULT 1,
			type TEXT NOT NULL,
			category TEXT NOT NULL,
			amount INTEGER NOT NULL,
			memo TEXT,
			sort_order INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS notes (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			content TEXT NOT NULL DEFAULT ''
		)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			log.Fatal(err)
		}
	}

	migrateRecurringToTemplates()
	ensureColumn("accounts", "balance", "INTEGER NOT NULL DEFAULT 0")
	ensureColumn("accounts", "excluded", "INTEGER NOT NULL DEFAULT 0")

	// seed default account
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM accounts`).Scan(&count); err != nil {
		log.Fatal(err)
	}
	if count == 0 {
		if _, err := db.Exec(`INSERT INTO accounts(name, icon, sort_order) VALUES ('기본', '💳', 0)`); err != nil {
			log.Fatal(err)
		}
	}
}

// migrateRecurringToTemplates carries over rows from the old auto-generating
// "recurring" table (day/active based) into the new manual "templates" table,
// then drops it. No-op if there's nothing to migrate.
func migrateRecurringToTemplates() {
	var hasOld int
	db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='recurring'`).Scan(&hasOld)
	if hasOld == 0 {
		return
	}
	rows, err := db.Query(`SELECT account_id, type, category, amount, memo FROM recurring`)
	type oldRec struct {
		accountID     int64
		typ, category string
		amount        int64
		memo          string
	}
	var recs []oldRec
	if err != nil {
		log.Println(err)
	} else {
		for rows.Next() {
			var rec oldRec
			var memo sql.NullString
			if rows.Scan(&rec.accountID, &rec.typ, &rec.category, &rec.amount, &memo) == nil {
				rec.memo = memo.String
				recs = append(recs, rec)
			}
		}
		rows.Close()
	}
	for i, rec := range recs {
		if _, err := db.Exec(`INSERT INTO templates(account_id, type, category, amount, memo, sort_order) VALUES (?, ?, ?, ?, ?, ?)`,
			rec.accountID, rec.typ, rec.category, rec.amount, rec.memo, i); err != nil {
			log.Println("template migration insert failed:", err)
		}
	}
	db.Exec(`DROP TABLE recurring`)
}

// ensureColumn adds a column to table if it doesn't already exist (simple migration helper).
func ensureColumn(table, column, def string) {
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		log.Println(err)
		return
	}
	exists := false
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk) == nil && name == column {
			exists = true
		}
	}
	rows.Close()
	if !exists {
		db.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + column + ` ` + def)
	}
}

func listAccounts() ([]Account, error) {
	rows, err := db.Query(`SELECT id, name, icon, balance, COALESCE(excluded,0) FROM accounts ORDER BY sort_order, id`)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	defer rows.Close()
	var out []Account
	for rows.Next() {
		var a Account
		if err := rows.Scan(&a.ID, &a.Name, &a.Icon, &a.Balance, &a.Excluded); err == nil {
			out = append(out, a)
		}
	}
	return out, rows.Err()
}

// accountNetTotals returns income minus expense per account, in one query
// rather than one per account.
func accountNetTotals() map[int64]int64 {
	out := map[int64]int64{}
	rows, err := db.Query(`SELECT account_id,
			COALESCE(SUM(CASE WHEN type='income'  THEN amount END),0),
			COALESCE(SUM(CASE WHEN type='expense' THEN amount END),0)
		FROM transactions GROUP BY account_id`)
	if err != nil {
		log.Println(err)
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var id, income, expense int64
		if rows.Scan(&id, &income, &expense) == nil {
			out[id] = income - expense
		}
	}
	return out
}

func getNote() string {
	var content string
	err := db.QueryRow(`SELECT content FROM notes WHERE id = 1`).Scan(&content)
	if err != nil {
		return ""
	}
	return content
}

func saveNote(content string) {
	db.Exec(`INSERT INTO notes(id, content) VALUES (1, ?) ON CONFLICT(id) DO UPDATE SET content = excluded.content`, content)
}

func transferBetweenAccounts(fromID, toID, amount int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE accounts SET balance = balance - ? WHERE id = ?`, amount, fromID); err != nil {
		tx.Rollback()
		return err
	}
	if _, err := tx.Exec(`UPDATE accounts SET balance = balance + ? WHERE id = ?`, amount, toID); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// errLastAccount means the delete was refused because it would leave the
// ledger with no accounts at all.
var errLastAccount = errors.New("cannot delete the last account")

// deleteAccountReassign moves an account's transactions and templates onto
// another account before removing it, all in one transaction so a mid-way
// failure can't orphan them.
func deleteAccountReassign(id string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var count int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM accounts`).Scan(&count); err != nil {
		return err
	}
	if count <= 1 {
		return errLastAccount
	}

	var first int64
	if err := tx.QueryRow(`SELECT id FROM accounts WHERE id != ? ORDER BY sort_order, id LIMIT 1`, id).Scan(&first); err != nil {
		return err
	}
	for _, stmt := range []string{
		`UPDATE transactions SET account_id = ? WHERE account_id = ?`,
		`UPDATE templates SET account_id = ? WHERE account_id = ?`,
	} {
		if _, err := tx.Exec(stmt, first, id); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`DELETE FROM accounts WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func listTemplates() ([]Template, error) {
	rows, err := db.Query(`SELECT t.id, t.account_id, a.name, a.icon, t.type, t.category, t.amount, t.memo
		FROM templates t JOIN accounts a ON a.id = t.account_id ORDER BY t.sort_order, t.id`)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	defer rows.Close()
	var out []Template
	for rows.Next() {
		var t Template
		var memo sql.NullString
		if rows.Scan(&t.ID, &t.AccountID, &t.AccountName, &t.AccountIcon, &t.Type, &t.Category, &t.Amount, &memo) == nil {
			t.Memo = memo.String
			out = append(out, t)
		}
	}
	return out, rows.Err()
}
