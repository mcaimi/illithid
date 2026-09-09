package intercept

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

type Lease struct {
	MAC       string    `json:"mac"`
	IP        string    `json:"ip"`
	Gateway   string    `json:"gateway"`
	DNS       []string  `json:"dns"`
	Interface string    `json:"interface"`
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
	if l.CreatedAt.IsZero() {
		l.CreatedAt = time.Now()
	}
	s.leases[key] = l
}

func (s *Store) Update(l *Lease) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := normalizeMAC(l.MAC)
	existing, ok := s.leases[key]
	if !ok {
		return fmt.Errorf("interception lease not found for mac %s", l.MAC)
	}

	l.MAC = key
	l.CreatedAt = existing.CreatedAt
	s.leases[key] = l
	return nil
}

func (s *Store) Remove(mac string) (*Lease, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := normalizeMAC(mac)
	l, ok := s.leases[key]
	if !ok {
		return nil, fmt.Errorf("interception lease not found for mac %s", mac)
	}

	delete(s.leases, key)
	return l, nil
}

func (s *Store) Lookup(mac string) (*Lease, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	l, ok := s.leases[normalizeMAC(mac)]
	return l, ok
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

func normalizeMAC(mac string) string {
	return strings.ToLower(mac)
}
