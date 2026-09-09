package api

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mcaimi/illithid/internal/intercept"
	"github.com/mcaimi/illithid/internal/lease"
	"github.com/mcaimi/illithid/internal/pool"
)

func setupTestServer(t *testing.T) (*Server, *lease.Store, *intercept.Store, *pool.IPPool) {
	t.Helper()

	ls := lease.NewStore()
	is := intercept.NewStore()
	p, err := pool.New(net.ParseIP("10.0.0.1"), net.ParseIP("10.0.0.100"))
	if err != nil {
		t.Fatal(err)
	}

	pools := map[string]*pool.IPPool{"eth0": p}
	ifNames := []string{"eth0"}
	srv := NewServer(ls, is, pools, ifNames)
	return srv, ls, is, p
}

func TestDocs(t *testing.T) {
	srv, _, _, _ := setupTestServer(t)

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
}

func TestOpenAPISpec(t *testing.T) {
	srv, _, _, _ := setupTestServer(t)

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

	paths, ok := spec["paths"].(map[string]any)
	if !ok {
		t.Fatal("missing paths object")
	}
	for _, path := range []string{"/leases", "/leases/{mac}", "/intercepts", "/intercepts/{mac}", "/clients"} {
		if _, ok := paths[path]; !ok {
			t.Errorf("missing %s path", path)
		}
	}
}

func TestListLeasesEmpty(t *testing.T) {
	srv, _, _, _ := setupTestServer(t)

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
	srv, ls, _, _ := setupTestServer(t)

	ls.Add(&lease.Lease{
		MAC: "aa:bb:cc:dd:ee:01", IP: "10.0.0.1", Hostname: "host1",
		Interface: "eth0", ExpiresAt: time.Now().Add(time.Hour), CreatedAt: time.Now(),
	})
	ls.Add(&lease.Lease{
		MAC: "aa:bb:cc:dd:ee:02", IP: "10.0.0.2", Hostname: "host2",
		Interface: "eth0", ExpiresAt: time.Now().Add(time.Hour), CreatedAt: time.Now(),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/leases", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	var leases []*lease.Lease
	json.NewDecoder(w.Body).Decode(&leases)
	if len(leases) != 2 {
		t.Errorf("expected 2 leases, got %d", len(leases))
	}
}

func TestDeleteLease(t *testing.T) {
	srv, ls, _, p := setupTestServer(t)

	p.Allocate("aa:bb:cc:dd:ee:01")
	ls.Add(&lease.Lease{
		MAC: "aa:bb:cc:dd:ee:01", IP: "10.0.0.1", Hostname: "host1",
		Interface: "eth0", ExpiresAt: time.Now().Add(time.Hour), CreatedAt: time.Now(),
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/leases/aa:bb:cc:dd:ee:01", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}

	_, free, _ := p.Stats()
	if free != 100 {
		t.Errorf("expected all IPs free, got free=%d", free)
	}
}

func TestDeleteLeaseNotFound(t *testing.T) {
	srv, _, _, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/leases/ff:ff:ff:ff:ff:ff", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// --- Intercept endpoints ---

func TestListInterceptsEmpty(t *testing.T) {
	srv, _, _, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/intercepts", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var intercepts []*intercept.Lease
	json.NewDecoder(w.Body).Decode(&intercepts)
	if len(intercepts) != 0 {
		t.Errorf("expected 0 intercepts, got %d", len(intercepts))
	}
}

func TestCreateIntercept(t *testing.T) {
	srv, _, _, _ := setupTestServer(t)

	body := `{"mac":"aa:bb:cc:dd:ee:01","ip":"192.168.1.50","gateway":"192.168.1.254","dns":["10.0.0.53"],"interface":"eth0"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/intercepts", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var il intercept.Lease
	json.NewDecoder(w.Body).Decode(&il)
	if il.MAC != "aa:bb:cc:dd:ee:01" {
		t.Errorf("expected mac aa:bb:cc:dd:ee:01, got %s", il.MAC)
	}
	if il.IP != "192.168.1.50" {
		t.Errorf("expected ip 192.168.1.50, got %s", il.IP)
	}
	if il.Gateway != "192.168.1.254" {
		t.Errorf("expected gateway 192.168.1.254, got %s", il.Gateway)
	}
}

func TestCreateInterceptDuplicate(t *testing.T) {
	srv, _, is, _ := setupTestServer(t)

	is.Add(&intercept.Lease{
		MAC: "aa:bb:cc:dd:ee:01", IP: "192.168.1.50", Gateway: "192.168.1.254",
		DNS: []string{"10.0.0.53"}, Interface: "eth0",
	})

	body := `{"mac":"aa:bb:cc:dd:ee:01","ip":"192.168.1.60","gateway":"192.168.1.254","dns":["10.0.0.53"],"interface":"eth0"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/intercepts", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", w.Code)
	}
}

func TestCreateInterceptValidation(t *testing.T) {
	srv, _, _, _ := setupTestServer(t)

	tests := []struct {
		name string
		body string
	}{
		{"invalid mac", `{"mac":"not-a-mac","ip":"1.2.3.4","gateway":"1.2.3.1","dns":["8.8.8.8"],"interface":"eth0"}`},
		{"invalid ip", `{"mac":"aa:bb:cc:dd:ee:01","ip":"bad","gateway":"1.2.3.1","dns":["8.8.8.8"],"interface":"eth0"}`},
		{"invalid gateway", `{"mac":"aa:bb:cc:dd:ee:01","ip":"1.2.3.4","gateway":"bad","dns":["8.8.8.8"],"interface":"eth0"}`},
		{"empty dns", `{"mac":"aa:bb:cc:dd:ee:01","ip":"1.2.3.4","gateway":"1.2.3.1","dns":[],"interface":"eth0"}`},
		{"invalid dns", `{"mac":"aa:bb:cc:dd:ee:01","ip":"1.2.3.4","gateway":"1.2.3.1","dns":["bad"],"interface":"eth0"}`},
		{"unknown interface", `{"mac":"aa:bb:cc:dd:ee:01","ip":"1.2.3.4","gateway":"1.2.3.1","dns":["8.8.8.8"],"interface":"eth99"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/intercepts", bytes.NewBufferString(tt.body))
			w := httptest.NewRecorder()
			srv.Handler().ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestUpdateIntercept(t *testing.T) {
	srv, _, is, _ := setupTestServer(t)

	is.Add(&intercept.Lease{
		MAC: "aa:bb:cc:dd:ee:01", IP: "192.168.1.50", Gateway: "192.168.1.254",
		DNS: []string{"10.0.0.53"}, Interface: "eth0",
	})

	body := `{"ip":"192.168.1.99","gateway":"192.168.1.1","dns":["1.1.1.1"],"interface":"eth0"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/intercepts/aa:bb:cc:dd:ee:01", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var il intercept.Lease
	json.NewDecoder(w.Body).Decode(&il)
	if il.IP != "192.168.1.99" {
		t.Errorf("expected updated ip 192.168.1.99, got %s", il.IP)
	}
}

func TestUpdateInterceptNotFound(t *testing.T) {
	srv, _, _, _ := setupTestServer(t)

	body := `{"ip":"192.168.1.99","gateway":"192.168.1.1","dns":["1.1.1.1"],"interface":"eth0"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/intercepts/ff:ff:ff:ff:ff:ff", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestDeleteIntercept(t *testing.T) {
	srv, ls, is, _ := setupTestServer(t)

	is.Add(&intercept.Lease{
		MAC: "aa:bb:cc:dd:ee:01", IP: "192.168.1.50", Gateway: "192.168.1.254",
		DNS: []string{"10.0.0.53"}, Interface: "eth0",
	})
	ls.Add(&lease.Lease{
		MAC: "aa:bb:cc:dd:ee:01", IP: "192.168.1.50", Interface: "eth0",
		Intercepted: true, ExpiresAt: time.Now().Add(time.Hour), CreatedAt: time.Now(),
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/intercepts/aa:bb:cc:dd:ee:01", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}

	if len(is.List()) != 0 {
		t.Error("expected intercept removed")
	}
	if len(ls.List()) != 0 {
		t.Error("expected tracking lease also removed")
	}
}

func TestDeleteInterceptNotFound(t *testing.T) {
	srv, _, _, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/intercepts/ff:ff:ff:ff:ff:ff", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// --- Clients endpoint ---

func TestListClients(t *testing.T) {
	srv, ls, _, _ := setupTestServer(t)

	ls.Add(&lease.Lease{
		MAC: "aa:bb:cc:dd:ee:01", IP: "10.0.0.1", Hostname: "normal-host",
		Interface: "eth0", ExpiresAt: time.Now().Add(time.Hour), CreatedAt: time.Now(),
	})
	ls.Add(&lease.Lease{
		MAC: "aa:bb:cc:dd:ee:02", IP: "192.168.1.50", Hostname: "intercepted-host",
		Interface: "eth0", Intercepted: true, ExpiresAt: time.Now().Add(time.Hour), CreatedAt: time.Now(),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/clients", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var clients []map[string]any
	json.NewDecoder(w.Body).Decode(&clients)
	if len(clients) != 2 {
		t.Fatalf("expected 2 clients, got %d", len(clients))
	}

	interceptedCount := 0
	for _, c := range clients {
		if c["intercepted"] == true {
			interceptedCount++
		}
	}
	if interceptedCount != 1 {
		t.Errorf("expected 1 intercepted client, got %d", interceptedCount)
	}
}
