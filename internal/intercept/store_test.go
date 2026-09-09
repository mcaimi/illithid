package intercept

import (
	"sync"
	"testing"
	"time"
)

func newTestLease(mac, ip, gw, iface string, dns []string) *Lease {
	return &Lease{
		MAC:       mac,
		IP:        ip,
		Gateway:   gw,
		DNS:       dns,
		Interface: iface,
		CreatedAt: time.Now(),
	}
}

func TestAddAndList(t *testing.T) {
	s := NewStore()

	s.Add(newTestLease("aa:bb:cc:dd:ee:01", "10.0.0.50", "10.0.0.254", "eth0", []string{"8.8.8.8"}))
	s.Add(newTestLease("aa:bb:cc:dd:ee:02", "10.0.0.51", "10.0.0.254", "eth0", []string{"8.8.8.8"}))

	leases := s.List()
	if len(leases) != 2 {
		t.Errorf("expected 2 leases, got %d", len(leases))
	}
}

func TestListEmpty(t *testing.T) {
	s := NewStore()

	leases := s.List()
	if leases == nil {
		t.Fatal("List() returned nil, expected empty slice")
	}
	if len(leases) != 0 {
		t.Errorf("expected 0 leases, got %d", len(leases))
	}
}

func TestAddOverwrite(t *testing.T) {
	s := NewStore()

	s.Add(newTestLease("aa:bb:cc:dd:ee:01", "10.0.0.50", "10.0.0.254", "eth0", []string{"8.8.8.8"}))
	s.Add(newTestLease("aa:bb:cc:dd:ee:01", "10.0.0.99", "10.0.0.254", "eth0", []string{"1.1.1.1"}))

	leases := s.List()
	if len(leases) != 1 {
		t.Fatalf("expected 1 lease after overwrite, got %d", len(leases))
	}
	if leases[0].IP != "10.0.0.99" {
		t.Errorf("expected overwritten IP 10.0.0.99, got %s", leases[0].IP)
	}
}

func TestUpdate(t *testing.T) {
	s := NewStore()
	s.Add(newTestLease("aa:bb:cc:dd:ee:01", "10.0.0.50", "10.0.0.254", "eth0", []string{"8.8.8.8"}))

	original, _ := s.Lookup("aa:bb:cc:dd:ee:01")
	originalCreated := original.CreatedAt

	updated := &Lease{
		MAC:     "aa:bb:cc:dd:ee:01",
		IP:      "10.0.0.99",
		Gateway: "10.0.0.1",
		DNS:     []string{"1.1.1.1"},
		Interface: "eth0",
	}
	err := s.Update(updated)
	if err != nil {
		t.Fatal(err)
	}

	l, ok := s.Lookup("aa:bb:cc:dd:ee:01")
	if !ok {
		t.Fatal("expected to find updated lease")
	}
	if l.IP != "10.0.0.99" {
		t.Errorf("expected updated IP 10.0.0.99, got %s", l.IP)
	}
	if l.Gateway != "10.0.0.1" {
		t.Errorf("expected updated gateway 10.0.0.1, got %s", l.Gateway)
	}
	if !l.CreatedAt.Equal(originalCreated) {
		t.Error("expected CreatedAt to be preserved after update")
	}
}

func TestUpdateNotFound(t *testing.T) {
	s := NewStore()

	err := s.Update(&Lease{MAC: "ff:ff:ff:ff:ff:ff"})
	if err == nil {
		t.Fatal("expected error updating non-existent lease")
	}
}

func TestRemove(t *testing.T) {
	s := NewStore()
	s.Add(newTestLease("aa:bb:cc:dd:ee:01", "10.0.0.50", "10.0.0.254", "eth0", []string{"8.8.8.8"}))

	removed, err := s.Remove("aa:bb:cc:dd:ee:01")
	if err != nil {
		t.Fatal(err)
	}
	if removed.IP != "10.0.0.50" {
		t.Errorf("expected removed IP 10.0.0.50, got %s", removed.IP)
	}

	if len(s.List()) != 0 {
		t.Error("expected empty store after remove")
	}
}

func TestRemoveNotFound(t *testing.T) {
	s := NewStore()

	_, err := s.Remove("ff:ff:ff:ff:ff:ff")
	if err == nil {
		t.Fatal("expected error removing non-existent lease")
	}
}

func TestLookup(t *testing.T) {
	s := NewStore()
	s.Add(newTestLease("aa:bb:cc:dd:ee:01", "10.0.0.50", "10.0.0.254", "eth0", []string{"8.8.8.8"}))

	l, found := s.Lookup("aa:bb:cc:dd:ee:01")
	if !found {
		t.Fatal("expected found")
	}
	if l.IP != "10.0.0.50" {
		t.Errorf("expected IP 10.0.0.50, got %s", l.IP)
	}

	_, found = s.Lookup("ff:ff:ff:ff:ff:ff")
	if found {
		t.Error("expected not found for unknown MAC")
	}
}

func TestMACNormalization(t *testing.T) {
	s := NewStore()
	s.Add(newTestLease("AA:BB:CC:DD:EE:01", "10.0.0.50", "10.0.0.254", "eth0", []string{"8.8.8.8"}))

	l, found := s.Lookup("aa:bb:cc:dd:ee:01")
	if !found {
		t.Fatal("expected to find lease with lowercase MAC lookup")
	}
	if l.MAC != "aa:bb:cc:dd:ee:01" {
		t.Errorf("expected normalized MAC, got %s", l.MAC)
	}
}

func TestConcurrentAccess(t *testing.T) {
	s := NewStore()
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			mac := "aa:bb:cc:dd:ee:" + string(rune('a'+i%26))
			s.Add(newTestLease(mac, "10.0.0.1", "10.0.0.254", "eth0", []string{"8.8.8.8"}))
			s.List()
			s.Lookup(mac)
		}(i)
	}

	wg.Wait()
}
