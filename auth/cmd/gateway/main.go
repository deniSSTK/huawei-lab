package main

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"university-lab/auth/internal/control"
)

type permission struct {
	Role  string
	Until time.Time
}
type controller struct {
	mu     sync.Mutex
	grants map[string]permission
	secret string
}

func (c *controller) apply(next map[string]permission) error {
	// One atomic transaction replaces the small lab grant set. Absolute expiry
	// is preserved for other clients; a change cannot prolong their sessions.
	var elements []string
	for ip, grant := range next {
		remaining := time.Until(grant.Until).Milliseconds()
		if remaining <= 0 {
			continue
		}
		for _, port := range control.Ports[grant.Role] {
			elements = append(elements, fmt.Sprintf("%s . %d timeout %dms", ip, port, remaining))
		}
	}
	sort.Strings(elements)
	script := "flush set inet lab_filter role_access\n"
	if len(elements) > 0 {
		script += "add element inet lab_filter role_access { " + strings.Join(elements, ", ") + " }\n"
	}
	cmd := exec.Command("nft", "-f", "-")
	cmd.Stdin = strings.NewReader(script)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("nft apply: %w: %s", err, out)
	}
	c.grants = next
	return nil
}

func (c *controller) serve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+c.secret)) != 1 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodPost || r.URL.Path != "/grant" {
		http.NotFound(w, r)
		return
	}
	var request control.Grant
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, "invalid JSON", 400)
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		http.Error(w, "trailing JSON", 400)
		return
	}
	if !control.ClientIP(request.IP) {
		http.Error(w, "invalid client IP", 400)
		return
	}
	if request.TTL < 0 || request.TTL > 3600 {
		http.Error(w, "invalid TTL", 400)
		return
	}
	if request.TTL > 0 && len(control.Ports[request.Role]) == 0 {
		http.Error(w, "invalid role", 400)
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	next := make(map[string]permission)
	for ip, grant := range c.grants {
		if time.Now().Before(grant.Until) {
			next[ip] = grant
		}
	}
	delete(next, request.IP)
	if request.TTL > 0 {
		next[request.IP] = permission{request.Role, time.Now().Add(time.Duration(request.TTL) * time.Second)}
	}
	if err := c.apply(next); err != nil {
		log.Printf("grant update failed: %v", err)
		http.Error(w, "gateway unavailable", 503)
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{"applied": true})
}

func main() {
	secret, err := control.Secret("/run/secrets/gateway_token")
	if err != nil || len(secret) < 32 {
		log.Fatal("missing management secret")
	}
	c := &controller{grants: make(map[string]permission), secret: secret}
	// Binding only the management IP keeps this API off Guest/IoT interfaces.
	server := &http.Server{Addr: "172.28.50.254:9090", Handler: http.HandlerFunc(c.serve), ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, IdleTimeout: 15 * time.Second}
	log.Print("gateway management ready on isolated management network")
	log.Fatal(server.ListenAndServe())
}
