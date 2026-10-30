package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"university-lab/auth/internal/control"
)

func fixture(t *testing.T) (*app, *[]control.Grant, *bool) {
	t.Helper()
	users := map[string]string{"student.test": "student-lab-test-password", "teacher.test": "teacher-lab-test-password", "admin.test": "admin-lab-test-password"}
	db, err := openDB(filepath.Join(t.TempDir(), "portal.db"), users)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	grants := []control.Grant{}
	available := true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !available {
			http.Error(w, "offline", 503)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-secret" {
			t.Error("missing management authentication")
		}
		var g control.Grant
		if json.NewDecoder(r.Body).Decode(&g) != nil {
			t.Error("invalid grant")
		}
		grants = append(grants, g)
	}))
	t.Cleanup(server.Close)
	return &app{db: db, client: server.Client(), gateway: server.URL, secret: "test-secret", ttl: 20, failures: make(map[string][]time.Time)}, &grants, &available
}

func request(a *app, method, path, body, ip string, cookie *http.Cookie) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://portal"+path, strings.NewReader(body))
	r.RemoteAddr = ip + ":1234"
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Accept", "application/json")
	if cookie != nil {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	a.handler().ServeHTTP(w, r)
	return w
}
func login(a *app, role, ip string) *httptest.ResponseRecorder {
	return request(a, "POST", "/api/login", url.Values{"username": {role + ".test"}, "password": {role + "-lab-test-password"}, "role": {"admin"}}.Encode(), ip, nil)
}

func TestLoginUsesStoredRoleAndBindsSessionToDevice(t *testing.T) {
	a, grants, _ := fixture(t)
	w := login(a, "student", "172.28.10.10")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	if (*grants)[1].Role != "student" {
		t.Fatal("client role override was trusted")
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatal("missing cookie protections")
	}
	if request(a, "GET", "/api/me", "", "172.28.10.11", cookies[0]).Code != 401 {
		t.Fatal("copied cookie worked on another device")
	}
	if request(a, "GET", "/api/me", "", "172.28.10.10", cookies[0]).Code != 200 {
		t.Fatal("session rejected")
	}
	var stored string
	if err := a.db.QueryRow("SELECT token_hash FROM sessions").Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored == cookies[0].Value {
		t.Fatal("raw session token stored")
	}
	var salt, hash []byte
	a.db.QueryRow("SELECT salt,password_hash FROM users WHERE username='student.test'").Scan(&salt, &hash)
	if len(salt) != 16 || len(hash) != 32 || strings.Contains(string(hash), "lab-test-password") {
		t.Fatal("password hashing failed")
	}
}

func TestWrongPasswordAndGatewayFailureDoNotCreateSession(t *testing.T) {
	a, grants, available := fixture(t)
	w := request(a, "POST", "/api/login", "username=admin.test&password=wrong", "172.28.10.10", nil)
	if w.Code != 401 || len(*grants) != 0 {
		t.Fatal("wrong password created a grant")
	}
	*available = false
	w = login(a, "admin", "172.28.10.10")
	if w.Code != 503 || len(w.Result().Cookies()) != 0 {
		t.Fatal("gateway failure accepted login")
	}
	var count int
	a.db.QueryRow("SELECT count(*) FROM sessions").Scan(&count)
	if count != 0 {
		t.Fatal("gateway failure persisted session")
	}
}

func TestRoleReplacementAndLogoutRevokeAccess(t *testing.T) {
	a, grants, _ := fixture(t)
	first := login(a, "admin", "172.28.10.10").Result().Cookies()[0]
	second := login(a, "student", "172.28.10.10").Result().Cookies()[0]
	if (*grants)[2].TTL != 0 || (*grants)[3].Role != "student" {
		t.Fatal("role replacement did not revoke first")
	}
	if request(a, "GET", "/api/me", "", "172.28.10.10", first).Code != 401 {
		t.Fatal("old session survived replacement")
	}
	w := request(a, "POST", "/api/logout", "", "172.28.10.10", second)
	if w.Code != 204 || (*grants)[4].TTL != 0 {
		t.Fatal("logout did not revoke")
	}
	if request(a, "GET", "/api/me", "", "172.28.10.10", second).Code != 401 {
		t.Fatal("logout session still active")
	}
}

func TestExpiryAndCSRF(t *testing.T) {
	a, grants, _ := fixture(t)
	c := login(a, "teacher", "172.28.10.10").Result().Cookies()[0]
	a.db.Exec("UPDATE sessions SET expires=?", time.Now().Unix()-1)
	if request(a, "GET", "/api/me", "", "172.28.10.10", c).Code != 401 {
		t.Fatal("expired session accepted")
	}
	r := httptest.NewRequest("POST", "http://portal/api/logout", nil)
	r.RemoteAddr = "172.28.10.10:1234"
	r.Header.Set("Origin", "https://other.example")
	r.AddCookie(c)
	w := httptest.NewRecorder()
	a.handler().ServeHTTP(w, r)
	if w.Code != 403 || len(*grants) != 2 {
		t.Fatal("cross-origin state change accepted")
	}
}

func TestFailedGrantAfterRevocationClearsNewSession(t *testing.T) {
	a, _, _ := fixture(t)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 2 {
			http.Error(w, "failed", 503)
		}
	}))
	defer server.Close()
	a.gateway = server.URL
	w := login(a, "student", "172.28.10.10")
	if w.Code != 503 {
		t.Fatal(w.Code)
	}
	var count int
	if err := a.db.QueryRow("SELECT count(*) FROM sessions").Scan(&count); err != nil && err != sql.ErrNoRows {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("failed grant left a session")
	}
}
