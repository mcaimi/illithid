package lease

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

type Lease struct {
	MAC       string    `json:"mac"`
	IP        string    `json:"ip"`
	Hostname  string    `json:"hostname"`
	Interface string    `json:"interface"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

type Store struct {
	mu     sync.RWMutex
	leases map[string]*Lease
}

func NewStore() *Store {
	return &Store{
		leases: make(map[string]*Lease),
	}
}

func (s *Store) Add(l *Lease) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := normalizeMAC(l.MAC)
	l.MAC = key
	s.leases[key] = l
}

func (s *Store) Remove(mac string) (*Lease, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := normalizeMAC(mac)
	l, ok := s.leases[key]
	if !ok {
		return nil, fmt.Errorf("lease not found for mac %s", mac)
	}

	delete(s.leases, key)
	return l, nil
}

func (s *Store) List() []*Lease {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Lease, 0, len(s.leases))
	for _, l := range s.leases {
		result = append(result, l)
	}
	return result
}

func (s *Store) Lookup(mac string) (*Lease, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	l, ok := s.leases[normalizeMAC(mac)]
	return l, ok
}

func normalizeMAC(mac string) string {
	return strings.ToLower(mac)
}
