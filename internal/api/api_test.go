package api

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mcaimi/illithid/internal/lease"
	"github.com/mcaimi/illithid/internal/pool"
)

func setupTestServer(t *testing.T) (*Server, *lease.Store, *pool.IPPool) {
	t.Helper()

	ls := lease.NewStore()
	p, err := pool.New(net.ParseIP("10.0.0.1"), net.ParseIP("10.0.0.100"))
	if err != nil {
		t.Fatal(err)
	}

	pools := map[string]*pool.IPPool{"eth0": p}
	srv := NewServer(ls, pools)
	return srv, ls, p
}

func TestDocs(t *testing.T) {
	srv, _, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/docs", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Errorf("expected text/html content-type, got %q", ct)
	}

	body := w.Body.String()
	if !strings.Contains(body, "swagger-ui") {
		t.Error("expected swagger-ui in HTML body")
	}
	if !strings.Contains(body, "/api/v1/openapi.json") {
		t.Error("expected openapi.json URL in HTML body")
	}
}

func TestOpenAPISpec(t *testing.T) {
	srv, _, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/openapi.json", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var spec map[string]any
	if err := json.NewDecoder(w.Body).Decode(&spec); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}

	if spec["openapi"] != "3.0.3" {
		t.Errorf("expected openapi version 3.0.3, got %v", spec["openapi"])
	}

	info, ok := spec["info"].(map[string]any)
	if !ok {
		t.Fatal("missing info object")
	}
	if info["title"] != "Illithid DHCP Server API" {
		t.Errorf("unexpected title: %v", info["title"])
	}

	paths, ok := spec["paths"].(map[string]any)
	if !ok {
		t.Fatal("missing paths object")
	}
	if _, ok := paths["/leases"]; !ok {
		t.Error("missing /leases path")
	}
	if _, ok := paths["/leases/{mac}"]; !ok {
		t.Error("missing /leases/{mac} path")
	}
}

func TestListLeasesEmpty(t *testing.T) {
	srv, _, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/leases", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var leases []*lease.Lease
	if err := json.NewDecoder(w.Body).Decode(&leases); err != nil {
		t.Fatal(err)
	}
	if len(leases) != 0 {
		t.Errorf("expected 0 leases, got %d", len(leases))
	}
}

func TestListLeasesWithData(t *testing.T) {
	srv, ls, _ := setupTestServer(t)

	ls.Add(&lease.Lease{
		MAC:       "aa:bb:cc:dd:ee:01",
		IP:        "10.0.0.1",
		Hostname:  "host1",
		Interface: "eth0",
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now(),
	})
	ls.Add(&lease.Lease{
		MAC:       "aa:bb:cc:dd:ee:02",
		IP:        "10.0.0.2",
		Hostname:  "host2",
		Interface: "eth0",
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now(),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/leases", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var leases []*lease.Lease
	if err := json.NewDecoder(w.Body).Decode(&leases); err != nil {
		t.Fatal(err)
	}
	if len(leases) != 2 {
		t.Errorf("expected 2 leases, got %d", len(leases))
	}
}

func TestDeleteLease(t *testing.T) {
	srv, ls, p := setupTestServer(t)

	p.Allocate("aa:bb:cc:dd:ee:01")
	ls.Add(&lease.Lease{
		MAC:       "aa:bb:cc:dd:ee:01",
		IP:        "10.0.0.1",
		Hostname:  "host1",
		Interface: "eth0",
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now(),
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/leases/aa:bb:cc:dd:ee:01", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}

	leases := ls.List()
	if len(leases) != 0 {
		t.Errorf("expected 0 leases after delete, got %d", len(leases))
	}

	_, free, _ := p.Stats()
	if free != 100 {
		t.Errorf("expected all IPs free after delete, got free=%d", free)
	}
}

func TestDeleteLeaseNotFound(t *testing.T) {
	srv, _, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/leases/ff:ff:ff:ff:ff:ff", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["error"] != "lease not found" {
		t.Errorf("expected 'lease not found' error, got %q", resp["error"])
	}
}
