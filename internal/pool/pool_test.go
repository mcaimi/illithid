package pool

import (
	"net"
	"sync"
	"testing"
)

func TestNewPool(t *testing.T) {
	p, err := New(net.ParseIP("192.168.1.100"), net.ParseIP("192.168.1.110"))
	if err != nil {
		t.Fatal(err)
	}

	total, free, allocated := p.Stats()
	if total != 11 {
		t.Errorf("expected total=11, got %d", total)
	}
	if free != 11 {
		t.Errorf("expected free=11, got %d", free)
	}
	if allocated != 0 {
		t.Errorf("expected allocated=0, got %d", allocated)
	}
}

func TestNewPoolInvalidRange(t *testing.T) {
	_, err := New(net.ParseIP("192.168.1.110"), net.ParseIP("192.168.1.100"))
	if err == nil {
		t.Fatal("expected error for reversed range")
	}
}

func TestAllocate(t *testing.T) {
	p, _ := New(net.ParseIP("10.0.0.1"), net.ParseIP("10.0.0.3"))

	ip1, err := p.Allocate("aa:bb:cc:dd:ee:01")
	if err != nil {
		t.Fatal(err)
	}
	if ip1 == nil {
		t.Fatal("expected non-nil IP")
	}

	ip2, err := p.Allocate("aa:bb:cc:dd:ee:02")
	if err != nil {
		t.Fatal(err)
	}
	if ip1.Equal(ip2) {
		t.Error("expected different IPs for different MACs")
	}
}

func TestAllocateIdempotent(t *testing.T) {
	p, _ := New(net.ParseIP("10.0.0.1"), net.ParseIP("10.0.0.3"))

	ip1, _ := p.Allocate("aa:bb:cc:dd:ee:01")
	ip2, _ := p.Allocate("aa:bb:cc:dd:ee:01")

	if !ip1.Equal(ip2) {
		t.Errorf("expected same IP for same MAC, got %s and %s", ip1, ip2)
	}

	_, free, _ := p.Stats()
	if free != 2 {
		t.Errorf("expected free=2 after idempotent allocate, got %d", free)
	}
}

func TestAllocateExhaustion(t *testing.T) {
	p, _ := New(net.ParseIP("10.0.0.1"), net.ParseIP("10.0.0.2"))

	p.Allocate("aa:bb:cc:dd:ee:01")
	p.Allocate("aa:bb:cc:dd:ee:02")

	_, err := p.Allocate("aa:bb:cc:dd:ee:03")
	if err == nil {
		t.Fatal("expected error on exhausted pool")
	}
}

func TestRelease(t *testing.T) {
	p, _ := New(net.ParseIP("10.0.0.1"), net.ParseIP("10.0.0.2"))

	ip, _ := p.Allocate("aa:bb:cc:dd:ee:01")
	released, err := p.Release("aa:bb:cc:dd:ee:01")
	if err != nil {
		t.Fatal(err)
	}
	if !ip.Equal(released) {
		t.Errorf("released IP %s doesn't match allocated %s", released, ip)
	}

	_, free, allocated := p.Stats()
	if free != 2 || allocated != 0 {
		t.Errorf("expected free=2 allocated=0 after release, got free=%d allocated=%d", free, allocated)
	}
}

func TestReleaseUnknownMAC(t *testing.T) {
	p, _ := New(net.ParseIP("10.0.0.1"), net.ParseIP("10.0.0.2"))

	_, err := p.Release("ff:ff:ff:ff:ff:ff")
	if err == nil {
		t.Fatal("expected error releasing unknown MAC")
	}
}

func TestReleaseAndReallocate(t *testing.T) {
	p, _ := New(net.ParseIP("10.0.0.1"), net.ParseIP("10.0.0.1"))

	p.Allocate("aa:bb:cc:dd:ee:01")
	p.Release("aa:bb:cc:dd:ee:01")

	ip, err := p.Allocate("aa:bb:cc:dd:ee:02")
	if err != nil {
		t.Fatal(err)
	}
	if ip == nil {
		t.Fatal("expected non-nil IP after release and reallocate")
	}
}

func TestLookup(t *testing.T) {
	p, _ := New(net.ParseIP("10.0.0.1"), net.ParseIP("10.0.0.3"))

	_, found := p.Lookup("aa:bb:cc:dd:ee:01")
	if found {
		t.Error("expected not found for unallocated MAC")
	}

	allocated, _ := p.Allocate("aa:bb:cc:dd:ee:01")
	ip, found := p.Lookup("aa:bb:cc:dd:ee:01")
	if !found {
		t.Fatal("expected found after allocation")
	}
	if !ip.Equal(allocated) {
		t.Errorf("lookup returned %s, expected %s", ip, allocated)
	}
}

func TestConcurrentAllocate(t *testing.T) {
	p, _ := New(net.ParseIP("10.0.0.1"), net.ParseIP("10.0.0.100"))

	var wg sync.WaitGroup
	errs := make(chan error, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			mac := net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, byte(i >> 8), byte(i)}.String()
			_, err := p.Allocate(mac)
			if err != nil {
				errs <- err
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent allocate error: %v", err)
	}

	_, free, allocated := p.Stats()
	if free != 0 || allocated != 100 {
		t.Errorf("expected free=0 allocated=100, got free=%d allocated=%d", free, allocated)
	}
}
