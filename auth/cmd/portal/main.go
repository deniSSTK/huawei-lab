package main

import (
	"bytes"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"university-lab/auth/internal/control"
)

const iterations = 600000
const cookieName = "lab_session"

type app struct {
	db              *sql.DB
	client          *http.Client
	gateway, secret string
	ttl             int
	secure          bool
	mu              sync.Mutex // Serialize session replacement and gateway changes.
	failures        map[string][]time.Time
}

type session struct {
	Token, IP, Role, User string
	Until                 int64
}

func token() string { return hex.EncodeToString(randomBytes(32)) }
func randomBytes(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return b
}
func tokenHash(s string) string { sum := sha256.Sum256([]byte(s)); return hex.EncodeToString(sum[:]) }
func passwordHash(password string, salt []byte) []byte {
	hash, err := pbkdf2.Key(sha256.New, password, salt, iterations, 32)
	if err != nil {
		panic(err)
	}
	return hash
}

func openDB(path string, users map[string]string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", path+"?_busy_timeout=5000&_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS users (username TEXT PRIMARY KEY, role TEXT NOT NULL, salt BLOB NOT NULL, password_hash BLOB NOT NULL);
 CREATE TABLE IF NOT EXISTS sessions (token_hash TEXT PRIMARY KEY, username TEXT NOT NULL REFERENCES users(username), ip TEXT UNIQUE NOT NULL, expires INTEGER NOT NULL);`)
	if err != nil {
		db.Close()
		return nil, err
	}
	for _, role := range []string{"student", "teacher", "admin"} {
		name := role + ".test"
		password := users[name]
		if len(password) < 16 {
			db.Close()
			return nil, fmt.Errorf("missing strong seed password for %s", name)
		}
		salt := randomBytes(16)
		// Seed is local and reproducible across recreation. Credentials exist only
		// in the ignored secret file and as salted hashes in SQLite.
		_, err = db.Exec(`INSERT INTO users(username,role,salt,password_hash) VALUES(?,?,?,?) ON CONFLICT(username) DO UPDATE SET role=excluded.role,salt=excluded.salt,password_hash=excluded.password_hash`, name, role, salt, passwordHash(password, salt))
		if err != nil {
			db.Close()
			return nil, err
		}
	}
	return db, nil
}

func peerIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return ""
	}
	return host
}

func (a *app) grant(ip, role string, ttl int) error {
	data, _ := json.Marshal(control.Grant{IP: ip, Role: role, TTL: ttl})
	request, err := http.NewRequest(http.MethodPost, a.gateway+"/grant", bytes.NewReader(data))
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+a.secret)
	request.Header.Set("Content-Type", "application/json")
	response, err := a.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	io.Copy(io.Discard, io.LimitReader(response.Body, 2048))
	if response.StatusCode != 200 {
		return fmt.Errorf("gateway returned %d", response.StatusCode)
	}
	return nil
}

func (a *app) current(r *http.Request) (session, error) {
	var s session
	c, err := r.Cookie(cookieName)
	if err != nil {
		return s, err
	}
	err = a.db.QueryRow(`SELECT s.token_hash,s.ip,u.role,u.username,s.expires FROM sessions s JOIN users u ON s.username=u.username WHERE s.token_hash=?`, tokenHash(c.Value)).Scan(&s.Token, &s.IP, &s.Role, &s.User, &s.Until)
	if err != nil {
		return s, err
	}
	if s.IP != peerIP(r) || time.Now().Unix() >= s.Until {
		return s, errors.New("session expired or wrong client")
	}
	return s, nil
}

func sameOrigin(r *http.Request) bool {
	if origin := r.Header.Get("Origin"); origin != "" {
		parsed, err := url.Parse(origin)
		return err == nil && parsed.Host == r.Host && (parsed.Scheme == "http" || parsed.Scheme == "https")
	}
	return r.Header.Get("Sec-Fetch-Site") != "cross-site"
}

func (a *app) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: a.secure, MaxAge: -1})
}

func (a *app) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	if !sameOrigin(r) {
		http.Error(w, "cross-origin request rejected", 403)
		return
	}
	ip := peerIP(r)
	if !control.ClientIP(ip) {
		http.Error(w, "Use a simulated guest device for network login", 403)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", 400)
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	var recent []time.Time
	for _, failure := range a.failures[ip] {
		if time.Since(failure) < time.Minute {
			recent = append(recent, failure)
		}
	}
	a.failures[ip] = recent
	if len(recent) >= 10 {
		http.Error(w, "too many attempts", 429)
		return
	}
	var role string
	var salt, hash []byte
	err := a.db.QueryRow(`SELECT role,salt,password_hash FROM users WHERE username=?`, r.Form.Get("username")).Scan(&role, &salt, &hash)
	// Keep the expensive password check for unknown accounts as well.
	if err != nil {
		salt = make([]byte, 16)
		hash = make([]byte, 32)
	}
	valid := subtle.ConstantTimeCompare(passwordHash(r.Form.Get("password"), salt), hash) == 1
	if err != nil || !valid {
		a.failures[ip] = append(recent, time.Now())
		http.Error(w, "invalid credentials", 401)
		return
	}
	// First revoke the previous device grant. If replacement cannot complete,
	// no old high-privilege access remains and no successful session is issued.
	if err := a.grant(ip, "", 0); err != nil {
		http.Error(w, "gateway unavailable", 503)
		return
	}
	if _, err := a.db.Exec(`DELETE FROM sessions WHERE ip=? OR expires<=?`, ip, time.Now().Unix()); err != nil {
		http.Error(w, "session storage unavailable", 503)
		return
	}
	raw := token()
	until := time.Now().Unix() + int64(a.ttl)
	if _, err := a.db.Exec(`INSERT INTO sessions(token_hash,username,ip,expires) VALUES(?,?,?,?)`, tokenHash(raw), r.Form.Get("username"), ip, until); err != nil {
		http.Error(w, "session storage unavailable", 503)
		return
	}
	// Floor to remaining whole seconds so gateway access never outlives SQLite.
	remaining := int(time.Until(time.Unix(until, 0)).Seconds())
	if err := a.grant(ip, role, remaining); err != nil {
		a.db.Exec(`DELETE FROM sessions WHERE ip=?`, ip)
		http.Error(w, "gateway unavailable", 503)
		return
	}
	delete(a.failures, ip)
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: raw, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: a.secure, MaxAge: a.ttl, Expires: time.Unix(until, 0)})
	if r.Header.Get("Accept") == "application/json" {
		json.NewEncoder(w).Encode(map[string]any{"role": role, "expires": until})
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *app) logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	if !sameOrigin(r) {
		http.Error(w, "cross-origin request rejected", 403)
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	s, err := a.current(r)
	if err != nil {
		a.clearCookie(w)
		http.Error(w, "no active session", 401)
		return
	}
	if err := a.grant(s.IP, "", 0); err != nil {
		http.Error(w, "gateway unavailable; access expires by TTL", 503)
		return
	}
	if _, err := a.db.Exec(`DELETE FROM sessions WHERE token_hash=?`, s.Token); err != nil {
		http.Error(w, "session storage unavailable", 503)
		return
	}
	a.clearCookie(w)
	if r.Header.Get("Accept") == "application/json" {
		w.WriteHeader(204)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (a *app) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if a.db.Ping() != nil {
			http.Error(w, "storage unavailable", 503)
			return
		}
		fmt.Fprintln(w, "ok")
	})
	mux.HandleFunc("/api/login", a.login)
	mux.HandleFunc("/api/logout", a.logout)
	mux.HandleFunc("/api/me", func(w http.ResponseWriter, r *http.Request) {
		s, err := a.current(r)
		if err != nil {
			http.Error(w, "unauthorized", 401)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"username": s.User, "role": s.Role, "expires": s.Until})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", 405)
			return
		}
		if r.URL.Path != "/" && r.URL.Path != "/login" {
			http.NotFound(w, r)
			return
		}
		s, _ := a.current(r)
		if err := page.Execute(w, map[string]any{"Session": s, "Active": s.User != "" && time.Now().Unix() < s.Until && s.IP == peerIP(r), "TTL": a.ttl}); err != nil {
			log.Print("page render failed")
		}
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'unsafe-inline'; form-action 'self'; frame-ancestors 'none'")
		mux.ServeHTTP(w, r)
	})
}

var page = template.Must(template.New("portal").Parse(`<!doctype html><html lang="uk"><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>University Access</title><style>body{font:17px system-ui;background:#f5f6fa;color:#172133;max-width:650px;margin:7vh auto;padding:24px}main{background:white;padding:36px;border-radius:16px}h1{font-size:32px;margin-top:0}label{display:block;margin:18px 0}input{display:block;padding:12px;border:1px solid #aab3c2;border-radius:6px;width:90%;font:inherit}button{padding:12px 22px;background:#234ac4;color:white;border:0;border-radius:6px;font:inherit}a{color:#234ac4}li{margin:12px 0}small{color:#536076}</style><main><h1>Університетська мережа</h1>{{if .Active}}<p>Ви увійшли як <strong>{{.Session.User}}</strong>. Роль: <strong>{{.Session.Role}}</strong>.</p><p>Доступ надано IP вашого Guest пристрою до завершення сесії. Інші пристрої залишаються ізольованими.</p><ul><li><a href="http://172.28.20.10/">Навчальні матеріали</a></li>{{if ne .Session.Role "student"}}<li><a href="http://172.28.20.10:8081/">Сервіс викладача</a></li>{{end}}{{if eq .Session.Role "admin"}}<li><a href="http://172.28.20.10:8082/">Адміністративний сервіс</a></li>{{end}}</ul><form method="post" action="/api/logout"><button>Вийти</button></form>{{else}}<p>Увійдіть з Guest пристрою, щоб отримати доступ за своєю роллю.</p><form method="post" action="/api/login"><label>Логін<input name="username" required autocomplete="username"></label><label>Пароль<input name="password" type="password" required autocomplete="current-password"></label><button>Увійти</button></form><p><a href="https://example.com/">Продовжити як гість</a></p><small>Доступ гостя до інтернету зберігається. Сесія діє {{.TTL}} секунд.</small>{{end}}</main></html>`))

func main() {
	secret, err := control.Secret("/run/secrets/gateway_token")
	if err != nil || len(secret) < 32 {
		log.Fatal("missing management secret")
	}
	data, err := os.ReadFile("/run/secrets/test_users")
	if err != nil {
		log.Fatal("missing seed users")
	}
	users := map[string]string{}
	if json.Unmarshal(data, &users) != nil {
		log.Fatal("invalid seed users")
	}
	ttl := 300
	if value := os.Getenv("SESSION_TTL_SECONDS"); value != "" {
		ttl, err = strconv.Atoi(value)
		if err != nil || ttl < 5 || ttl > 3600 {
			log.Fatal("TTL must be between 5 and 3600 seconds")
		}
	}
	db, err := openDB("/data/portal.db", users)
	if err != nil {
		log.Fatal("database initialization failed: ", err)
	}
	defer db.Close()
	a := &app{db: db, client: &http.Client{Timeout: 3 * time.Second}, gateway: "http://172.28.50.254:9090", secret: secret, ttl: ttl, secure: strings.EqualFold(os.Getenv("COOKIE_SECURE"), "true"), failures: make(map[string][]time.Time)}
	log.Printf("Auth Portal ready; session TTL=%ds", ttl)
	server := &http.Server{Addr: ":8080", Handler: a.handler(), ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 8 * time.Second, WriteTimeout: 8 * time.Second, IdleTimeout: 20 * time.Second}
	log.Fatal(server.ListenAndServe())
}
