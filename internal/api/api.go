package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/mcaimi/illithid/internal/lease"
	"github.com/mcaimi/illithid/internal/pool"
)

type Server struct {
	leases *lease.Store
	pools  map[string]*pool.IPPool
	router chi.Router
}

func NewServer(leases *lease.Store, pools map[string]*pool.IPPool) *Server {
	s := &Server{
		leases: leases,
		pools:  pools,
		router: chi.NewRouter(),
	}

	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)
	s.router.Use(middleware.SetHeader("Content-Type", "application/json"))

	s.router.Get("/api/docs", s.serveDocs)

	s.router.Route("/api/v1", func(r chi.Router) {
		r.Get("/openapi.json", s.openAPISpec)
		r.Get("/leases", s.listLeases)
		r.Delete("/leases/{mac}", s.deleteLease)
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
