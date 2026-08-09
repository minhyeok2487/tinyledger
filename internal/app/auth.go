package app

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	sessionCookie   = "gagyebu_session"
	sessionDuration = 30 * 24 * time.Hour
)

// sitePassword is the single shared password. Empty means auth is off, which
// is only allowed outside Vercel so a missing env var can't silently expose
// the deployed ledger.
func sitePassword() string {
	return os.Getenv("GAGYEBU_PASSWORD")
}

func authEnabled() bool {
	return sitePassword() != ""
}

// signSession returns "<expiry>.<hmac>". The password itself is the signing
// key, so no extra secret is needed and changing the password invalidates
// every existing session. Every serverless instance derives the same key,
// so sessions survive cold starts without a session store.
func signSession(exp int64, pw string) string {
	mac := hmac.New(sha256.New, []byte(pw))
	mac.Write([]byte("gagyebu-session-v1:" + strconv.FormatInt(exp, 10)))
	return strconv.FormatInt(exp, 10) + "." + hex.EncodeToString(mac.Sum(nil))
}

func validSession(value, pw string) bool {
	dot := strings.IndexByte(value, '.')
	if dot < 0 {
		return false
	}
	exp, err := strconv.ParseInt(value[:dot], 10, 64)
	if err != nil || time.Now().Unix() > exp {
		return false
	}
	return hmac.Equal([]byte(signSession(exp, pw)), []byte(value))
}

// safeNext keeps the post-login redirect on this site — an attacker-supplied
// ?next= must not be able to bounce the user to another host.
func safeNext(next string) string {
	if next == "" || !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") {
		return "/"
	}
	return next
}

func isHTTPS(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, pw string) {
	exp := time.Now().Add(sessionDuration)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    signSession(exp.Unix(), pw),
		Path:     "/",
		Expires:  exp,
		MaxAge:   int(sessionDuration.Seconds()),
		HttpOnly: true,
		Secure:   isHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	})
}

// requireAuth gates every route except the login page and static assets.
func requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pw := sitePassword()
		if pw == "" {
			// Fail closed when deployed: an unset password there means the
			// ledger would otherwise be readable and writable by anyone.
			if os.Getenv("VERCEL") != "" {
				http.Error(w, "GAGYEBU_PASSWORD 환경변수가 설정되지 않았습니다.", http.StatusServiceUnavailable)
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		if r.URL.Path == "/login" || strings.HasPrefix(r.URL.Path, "/static/") {
			next.ServeHTTP(w, r)
			return
		}

		c, err := r.Cookie(sessionCookie)
		if err != nil || !validSession(c.Value, pw) {
			if r.Method == http.MethodGet {
				http.Redirect(w, r, "/login?next="+url.QueryEscape(r.URL.RequestURI()), http.StatusSeeOther)
			} else {
				// A POST from an expired tab shouldn't silently redirect and
				// look like it worked.
				http.Error(w, "세션이 만료되었습니다. 다시 로그인해주세요.", http.StatusUnauthorized)
			}
			return
		}
		next.ServeHTTP(w, r)
	})
}

func handleLoginPage(w http.ResponseWriter, r *http.Request) {
	if !authEnabled() {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	renderLogin(w, r, "")
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	pw := sitePassword()
	if pw == "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	// Constant-time compare so the response time doesn't leak the password.
	if subtle.ConstantTimeCompare([]byte(r.FormValue("password")), []byte(pw)) != 1 {
		// Slow down guessing a little without holding the request open.
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusUnauthorized)
		renderLogin(w, r, "비밀번호가 틀렸습니다.")
		return
	}
	setSessionCookie(w, r, pw)
	http.Redirect(w, r, safeNext(r.FormValue("next")), http.StatusSeeOther)
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   isHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func renderLogin(w http.ResponseWriter, r *http.Request, errMsg string) {
	data := LoginData{Next: safeNext(r.FormValue("next")), Error: errMsg}
	if err := tpl.ExecuteTemplate(w, "login.html", data); err != nil {
		log.Println(err)
	}
}
