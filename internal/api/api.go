package api

import (
	"encoding/json"
	"net"
	"net/http"
	"regexp"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/mcaimi/illithid/internal/intercept"
	"github.com/mcaimi/illithid/internal/lease"
	"github.com/mcaimi/illithid/internal/pool"
)

var macRegex = regexp.MustCompile(`^([0-9a-fA-F]{2}:){5}[0-9a-fA-F]{2}$`)

type Server struct {
	leases     *lease.Store
	intercepts *intercept.Store
	pools      map[string]*pool.IPPool
	ifNames    map[string]struct{}
	router     chi.Router
}

func NewServer(leases *lease.Store, intercepts *intercept.Store, pools map[string]*pool.IPPool, ifNames []string) *Server {
	ifSet := make(map[string]struct{}, len(ifNames))
	for _, name := range ifNames {
		ifSet[name] = struct{}{}
	}

	s := &Server{
		leases:     leases,
		intercepts: intercepts,
		pools:      pools,
		ifNames:    ifSet,
		router:     chi.NewRouter(),
	}

	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)
	s.router.Use(middleware.SetHeader("Content-Type", "application/json"))

	s.router.Get("/api/docs", s.serveDocs)

	s.router.Route("/api/v1", func(r chi.Router) {
		r.Get("/openapi.json", s.openAPISpec)

		r.Get("/leases", s.listLeases)
		r.Delete("/leases/{mac}", s.deleteLease)

		r.Get("/intercepts", s.listIntercepts)
		r.Post("/intercepts", s.createIntercept)
		r.Put("/intercepts/{mac}", s.updateIntercept)
		r.Delete("/intercepts/{mac}", s.deleteIntercept)

		r.Get("/clients", s.listClients)
	})

	return s
}

func (s *Server) Handler() http.Handler {
	return s.router
}

func (s *Server) serveDocs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(docsHTML))
}

func (s *Server) openAPISpec(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(openapiSpec))
}

func (s *Server) listLeases(w http.ResponseWriter, r *http.Request) {
	leases := s.leases.List()
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(leases)
}

func (s *Server) deleteLease(w http.ResponseWriter, r *http.Request) {
	mac := chi.URLParam(r, "mac")

	removed, err := s.leases.Remove(mac)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "lease not found"})
		return
	}

	if p, ok := s.pools[removed.Interface]; ok {
		p.Release(removed.MAC)
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listIntercepts(w http.ResponseWriter, r *http.Request) {
	intercepts := s.intercepts.List()
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(intercepts)
}

type interceptRequest struct {
	MAC       string   `json:"mac"`
	IP        string   `json:"ip"`
	Gateway   string   `json:"gateway"`
	DNS       []string `json:"dns"`
	Interface string   `json:"interface"`
}

func (s *Server) validateInterceptRequest(req *interceptRequest, macFromURL string) (string, error) {
	mac := req.MAC
	if macFromURL != "" {
		mac = macFromURL
	}
	if !macRegex.MatchString(mac) {
		return "", &apiError{"invalid mac address format"}
	}
	if net.ParseIP(req.IP) == nil {
		return "", &apiError{"invalid ip address"}
	}
	if net.ParseIP(req.Gateway) == nil {
		return "", &apiError{"invalid gateway address"}
	}
	if len(req.DNS) == 0 {
		return "", &apiError{"at least one dns server is required"}
	}
	for _, d := range req.DNS {
		if net.ParseIP(d) == nil {
			return "", &apiError{"invalid dns address: " + d}
		}
	}
	if _, ok := s.ifNames[req.Interface]; !ok {
		return "", &apiError{"unknown interface: " + req.Interface}
	}
	return mac, nil
}

type apiError struct {
	msg string
}

func (e *apiError) Error() string {
	return e.msg
}

func (s *Server) createIntercept(w http.ResponseWriter, r *http.Request) {
	var req interceptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid request body"})
		return
	}

	mac, err := s.validateInterceptRequest(&req, "")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	if _, exists := s.intercepts.Lookup(mac); exists {
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{"error": "interception lease already exists for this mac"})
		return
	}

	il := &intercept.Lease{
		MAC:       mac,
		IP:        req.IP,
		Gateway:   req.Gateway,
		DNS:       req.DNS,
		Interface: req.Interface,
		CreatedAt: time.Now(),
	}
	s.intercepts.Add(il)

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(il)
}

func (s *Server) updateIntercept(w http.ResponseWriter, r *http.Request) {
	macParam := chi.URLParam(r, "mac")

	var req interceptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid request body"})
		return
	}

	mac, err := s.validateInterceptRequest(&req, macParam)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	il := &intercept.Lease{
		MAC:       mac,
		IP:        req.IP,
		Gateway:   req.Gateway,
		DNS:       req.DNS,
		Interface: req.Interface,
	}
	if err := s.intercepts.Update(il); err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "interception lease not found"})
		return
	}

	updated, _ := s.intercepts.Lookup(mac)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(updated)
}

func (s *Server) deleteIntercept(w http.ResponseWriter, r *http.Request) {
	mac := chi.URLParam(r, "mac")

	_, err := s.intercepts.Remove(mac)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "interception lease not found"})
		return
	}

	s.leases.Remove(mac)

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listClients(w http.ResponseWriter, r *http.Request) {
	clients := s.leases.List()
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(clients)
}
