package app

import (
	"database/sql"
	"log"
	"os"
	"time"
)

var db *sql.DB

func initDB(path string) {
	var err error
	db, err = sql.Open("sqlite", path)
	if err != nil {
		log.Fatal(err)
	}
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

	// setupSchema costs a dozen sequential round trips to the remote database,
	// which every serverless cold start would otherwise pay before serving a
	// byte. The schema is already in place there, so only run it when asked.
	if os.Getenv("VERCEL") == "" || os.Getenv("RUN_MIGRATIONS") == "1" {
		setupSchema()
	}
	log.Println("Turso DB 연결:", url)
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
		accountID      int64
		typ, category  string
		amount         int64
		memo           string
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

func listAccounts() []Account {
	rows, err := db.Query(`SELECT id, name, icon, balance FROM accounts ORDER BY sort_order, id`)
	if err != nil {
		log.Println(err)
		return nil
	}
	defer rows.Close()
	var out []Account
	for rows.Next() {
		var a Account
		if err := rows.Scan(&a.ID, &a.Name, &a.Icon, &a.Balance); err == nil {
			out = append(out, a)
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

func listTemplates() []Template {
	rows, err := db.Query(`SELECT t.id, t.account_id, a.name, a.icon, t.type, t.category, t.amount, t.memo
		FROM templates t JOIN accounts a ON a.id = t.account_id ORDER BY t.sort_order, t.id`)
	if err != nil {
		log.Println(err)
		return nil
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
	return out
}
