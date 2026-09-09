package pool

import (
	"encoding/binary"
	"fmt"
	"net"
	"sync"
)

type IPPool struct {
	mu        sync.Mutex
	free      map[string]struct{}
	allocated map[string]string
	ipToMAC   map[string]string
}

func New(rangeStart, rangeEnd net.IP) (*IPPool, error) {
	ips, err := enumerateIPs(rangeStart, rangeEnd)
	if err != nil {
		return nil, err
	}

	free := make(map[string]struct{}, len(ips))
	for _, ip := range ips {
		free[ip.String()] = struct{}{}
	}

	return &IPPool{
		free:      free,
		allocated: make(map[string]string),
		ipToMAC:   make(map[string]string),
	}, nil
}

func (p *IPPool) Allocate(mac string) (net.IP, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if ip, ok := p.allocated[mac]; ok {
		return net.ParseIP(ip), nil
	}

	if len(p.free) == 0 {
		return nil, fmt.Errorf("pool exhausted")
	}

	var ipStr string
	for ipStr = range p.free {
		break
	}

	delete(p.free, ipStr)
	p.allocated[mac] = ipStr
	p.ipToMAC[ipStr] = mac

	return net.ParseIP(ipStr), nil
}

func (p *IPPool) Release(mac string) (net.IP, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	ipStr, ok := p.allocated[mac]
	if !ok {
		return nil, fmt.Errorf("mac %s not found in pool", mac)
	}

	delete(p.allocated, mac)
	delete(p.ipToMAC, ipStr)
	p.free[ipStr] = struct{}{}

	return net.ParseIP(ipStr), nil
}

func (p *IPPool) Lookup(mac string) (net.IP, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	ipStr, ok := p.allocated[mac]
	if !ok {
		return nil, false
	}

	return net.ParseIP(ipStr), true
}

func (p *IPPool) Stats() (total, free, allocated int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	free = len(p.free)
	allocated = len(p.allocated)
	total = free + allocated
	return
}

func enumerateIPs(start, end net.IP) ([]net.IP, error) {
	s4 := start.To4()
	e4 := end.To4()
	if s4 == nil || e4 == nil {
		return nil, fmt.Errorf("only IPv4 addresses are supported")
	}

	startInt := binary.BigEndian.Uint32(s4)
	endInt := binary.BigEndian.Uint32(e4)

	if startInt > endInt {
		return nil, fmt.Errorf("start %s is after end %s", start, end)
	}

	count := endInt - startInt + 1
	if count > 65534 {
		return nil, fmt.Errorf("range too large: %d addresses", count)
	}

	ips := make([]net.IP, 0, count)
	for i := startInt; i <= endInt; i++ {
		ip := make(net.IP, 4)
		binary.BigEndian.PutUint32(ip, i)
		ips = append(ips, ip)
	}

	return ips, nil
}
