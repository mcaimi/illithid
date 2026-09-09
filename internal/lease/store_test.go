package lease

import (
	"sync"
	"testing"
	"time"
)

func newTestLease(mac, ip, hostname, iface string) *Lease {
	return &Lease{
		MAC:       mac,
		IP:        ip,
		Hostname:  hostname,
		Interface: iface,
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now(),
	}
}

func TestAddAndList(t *testing.T) {
	s := NewStore()

	s.Add(newTestLease("aa:bb:cc:dd:ee:01", "10.0.0.1", "host1", "eth0"))
	s.Add(newTestLease("aa:bb:cc:dd:ee:02", "10.0.0.2", "host2", "eth0"))

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

	s.Add(newTestLease("aa:bb:cc:dd:ee:01", "10.0.0.1", "host1", "eth0"))
	s.Add(newTestLease("aa:bb:cc:dd:ee:01", "10.0.0.5", "host1-new", "eth0"))

	leases := s.List()
	if len(leases) != 1 {
		t.Fatalf("expected 1 lease after overwrite, got %d", len(leases))
	}
	if leases[0].IP != "10.0.0.5" {
		t.Errorf("expected overwritten IP 10.0.0.5, got %s", leases[0].IP)
	}
}

func TestRemove(t *testing.T) {
	s := NewStore()
	s.Add(newTestLease("aa:bb:cc:dd:ee:01", "10.0.0.1", "host1", "eth0"))

	removed, err := s.Remove("aa:bb:cc:dd:ee:01")
	if err != nil {
		t.Fatal(err)
	}
	if removed.IP != "10.0.0.1" {
		t.Errorf("expected removed IP 10.0.0.1, got %s", removed.IP)
	}

	if len(s.List()) != 0 {
		t.Error("expected empty store after remove")
	}
}

func TestRemoveNotFound(t *testing.T) {
	s := NewStore()

	_, err := s.Remove("ff:ff:ff:ff:ff:ff")
	if err == nil {
		t.Fatal("expected error removing non-existent MAC")
	}
}

func TestLookup(t *testing.T) {
	s := NewStore()
	s.Add(newTestLease("aa:bb:cc:dd:ee:01", "10.0.0.1", "host1", "eth0"))

	l, found := s.Lookup("aa:bb:cc:dd:ee:01")
	if !found {
		t.Fatal("expected found")
	}
	if l.IP != "10.0.0.1" {
		t.Errorf("expected IP 10.0.0.1, got %s", l.IP)
	}

	_, found = s.Lookup("ff:ff:ff:ff:ff:ff")
	if found {
		t.Error("expected not found for unknown MAC")
	}
}

func TestMACNormalization(t *testing.T) {
	s := NewStore()
	s.Add(newTestLease("AA:BB:CC:DD:EE:01", "10.0.0.1", "host1", "eth0"))

	l, found := s.Lookup("aa:bb:cc:dd:ee:01")
	if !found {
		t.Fatal("expected to find lease with lowercase MAC lookup")
	}
	if l.MAC != "aa:bb:cc:dd:ee:01" {
		t.Errorf("expected normalized MAC aa:bb:cc:dd:ee:01, got %s", l.MAC)
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
			s.Add(newTestLease(mac, "10.0.0.1", "host", "eth0"))
			s.List()
			s.Lookup(mac)
		}(i)
	}

	wg.Wait()
}
