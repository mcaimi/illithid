package dhcp

import (
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv4/server4"

	"github.com/mcaimi/illithid/internal/config"
	"github.com/mcaimi/illithid/internal/intercept"
	"github.com/mcaimi/illithid/internal/lease"
	"github.com/mcaimi/illithid/internal/pool"
)

type Handler struct {
	ifName     string
	pool       *pool.IPPool
	leases     *lease.Store
	intercepts *intercept.Store
	serverIP   net.IP
	gateway    net.IP
	dns        []net.IP
	mask       net.IPMask
	duration   time.Duration
	logger     *slog.Logger
}

func NewHandler(cfg config.InterfaceConfig, p *pool.IPPool, ls *lease.Store, is *intercept.Store, logger *slog.Logger) *Handler {
	dns := make([]net.IP, len(cfg.DNS))
	for i, d := range cfg.DNS {
		dns[i] = net.ParseIP(d).To4()
	}

	maskIP := net.ParseIP(cfg.DHCP.SubnetMask).To4()
	mask := net.IPMask(maskIP)

	return &Handler{
		ifName:     cfg.Name,
		pool:       p,
		leases:     ls,
		intercepts: is,
		serverIP:   net.ParseIP(cfg.ServerIP).To4(),
		gateway:    net.ParseIP(cfg.Gateway).To4(),
		dns:        dns,
		mask:       mask,
		duration:   time.Duration(cfg.DHCP.LeaseDuration) * time.Second,
		logger:     logger.With("interface", cfg.Name),
	}
}

func (h *Handler) ServeDHCP(conn net.PacketConn, peer net.Addr, msg *dhcpv4.DHCPv4) {
	h.logger.Debug("received", "type", msg.MessageType(), "mac", msg.ClientHWAddr)

	switch msg.MessageType() {
	case dhcpv4.MessageTypeDiscover:
		h.handleDiscover(conn, peer, msg)
	case dhcpv4.MessageTypeRequest:
		h.handleRequest(conn, peer, msg)
	case dhcpv4.MessageTypeRelease:
		h.handleRelease(msg)
	default:
		h.logger.Debug("ignoring message type", "type", msg.MessageType())
	}
}

func (h *Handler) interceptParams(il *intercept.Lease) (ip, gw net.IP, dns []net.IP) {
	ip = net.ParseIP(il.IP).To4()
	gw = net.ParseIP(il.Gateway).To4()
	dns = make([]net.IP, len(il.DNS))
	for i, d := range il.DNS {
		dns[i] = net.ParseIP(d).To4()
	}
	return
}

func (h *Handler) handleDiscover(conn net.PacketConn, peer net.Addr, msg *dhcpv4.DHCPv4) {
	mac := msg.ClientHWAddr.String()

	if il, ok := h.intercepts.Lookup(mac); ok && il.Interface == h.ifName {
		h.sendInterceptOffer(conn, peer, msg, il)
		return
	}

	ip, err := h.pool.Allocate(mac)
	if err != nil {
		h.logger.Warn("pool exhausted", "mac", mac)
		return
	}

	reply, err := dhcpv4.NewReplyFromRequest(msg,
		dhcpv4.WithMessageType(dhcpv4.MessageTypeOffer),
		dhcpv4.WithYourIP(ip),
		dhcpv4.WithServerIP(h.serverIP),
		dhcpv4.WithOption(dhcpv4.OptSubnetMask(h.mask)),
		dhcpv4.WithOption(dhcpv4.OptRouter(h.gateway)),
		dhcpv4.WithOption(dhcpv4.OptDNS(h.dns...)),
		dhcpv4.WithOption(dhcpv4.OptIPAddressLeaseTime(h.duration)),
		dhcpv4.WithOption(dhcpv4.OptServerIdentifier(h.serverIP)),
	)
	if err != nil {
		h.logger.Error("failed to build OFFER", "error", err)
		return
	}

	if _, err := conn.WriteTo(reply.ToBytes(), peer); err != nil {
		h.logger.Error("failed to send OFFER", "error", err)
		return
	}

	h.logger.Info("OFFER sent", "mac", mac, "ip", ip)
}

func (h *Handler) sendInterceptOffer(conn net.PacketConn, peer net.Addr, msg *dhcpv4.DHCPv4, il *intercept.Lease) {
	ip, gw, dns := h.interceptParams(il)

	reply, err := dhcpv4.NewReplyFromRequest(msg,
		dhcpv4.WithMessageType(dhcpv4.MessageTypeOffer),
		dhcpv4.WithYourIP(ip),
		dhcpv4.WithServerIP(h.serverIP),
		dhcpv4.WithOption(dhcpv4.OptSubnetMask(h.mask)),
		dhcpv4.WithOption(dhcpv4.OptRouter(gw)),
		dhcpv4.WithOption(dhcpv4.OptDNS(dns...)),
		dhcpv4.WithOption(dhcpv4.OptIPAddressLeaseTime(h.duration)),
		dhcpv4.WithOption(dhcpv4.OptServerIdentifier(h.serverIP)),
	)
	if err != nil {
		h.logger.Error("failed to build OFFER (intercepted)", "error", err)
		return
	}

	if _, err := conn.WriteTo(reply.ToBytes(), peer); err != nil {
		h.logger.Error("failed to send OFFER (intercepted)", "error", err)
		return
	}

	h.logger.Info("OFFER sent (intercepted)", "mac", msg.ClientHWAddr, "ip", ip, "gateway", gw)
}

func (h *Handler) handleRequest(conn net.PacketConn, peer net.Addr, msg *dhcpv4.DHCPv4) {
	mac := msg.ClientHWAddr.String()

	if il, ok := h.intercepts.Lookup(mac); ok && il.Interface == h.ifName {
		h.handleInterceptRequest(conn, peer, msg, il)
		return
	}

	var requestedIP net.IP
	if opt := msg.Options.Get(dhcpv4.OptionRequestedIPAddress); opt != nil {
		requestedIP = net.IP(opt)
	} else if !msg.ClientIPAddr.IsUnspecified() {
		requestedIP = msg.ClientIPAddr
	}

	allocatedIP, ok := h.pool.Lookup(mac)
	if !ok || (requestedIP != nil && !allocatedIP.Equal(requestedIP)) {
		h.sendNAK(conn, peer, msg, mac, requestedIP)
		return
	}

	hostname := ""
	if opt := msg.Options.Get(dhcpv4.OptionHostName); opt != nil {
		hostname = string(opt)
	}

	h.leases.Add(&lease.Lease{
		MAC:       mac,
		IP:        allocatedIP.String(),
		Hostname:  hostname,
		Interface: h.ifName,
		ExpiresAt: time.Now().Add(h.duration),
		CreatedAt: time.Now(),
	})

	ack, err := dhcpv4.NewReplyFromRequest(msg,
		dhcpv4.WithMessageType(dhcpv4.MessageTypeAck),
		dhcpv4.WithYourIP(allocatedIP),
		dhcpv4.WithServerIP(h.serverIP),
		dhcpv4.WithOption(dhcpv4.OptSubnetMask(h.mask)),
		dhcpv4.WithOption(dhcpv4.OptRouter(h.gateway)),
		dhcpv4.WithOption(dhcpv4.OptDNS(h.dns...)),
		dhcpv4.WithOption(dhcpv4.OptIPAddressLeaseTime(h.duration)),
		dhcpv4.WithOption(dhcpv4.OptServerIdentifier(h.serverIP)),
	)
	if err != nil {
		h.logger.Error("failed to build ACK", "error", err)
		return
	}

	if _, err := conn.WriteTo(ack.ToBytes(), peer); err != nil {
		h.logger.Error("failed to send ACK", "error", err)
		return
	}

	h.logger.Info("ACK sent", "mac", mac, "ip", allocatedIP, "hostname", hostname)
}

func (h *Handler) handleInterceptRequest(conn net.PacketConn, peer net.Addr, msg *dhcpv4.DHCPv4, il *intercept.Lease) {
	mac := msg.ClientHWAddr.String()
	interceptIP, gw, dns := h.interceptParams(il)

	var requestedIP net.IP
	if opt := msg.Options.Get(dhcpv4.OptionRequestedIPAddress); opt != nil {
		requestedIP = net.IP(opt)
	} else if !msg.ClientIPAddr.IsUnspecified() {
		requestedIP = msg.ClientIPAddr
	}

	if requestedIP != nil && !interceptIP.Equal(requestedIP) {
		h.sendNAK(conn, peer, msg, mac, requestedIP)
		return
	}

	hostname := ""
	if opt := msg.Options.Get(dhcpv4.OptionHostName); opt != nil {
		hostname = string(opt)
	}

	h.leases.Add(&lease.Lease{
		MAC:         mac,
		IP:          il.IP,
		Hostname:    hostname,
		Interface:   h.ifName,
		Intercepted: true,
		ExpiresAt:   time.Now().Add(h.duration),
		CreatedAt:   time.Now(),
	})

	ack, err := dhcpv4.NewReplyFromRequest(msg,
		dhcpv4.WithMessageType(dhcpv4.MessageTypeAck),
		dhcpv4.WithYourIP(interceptIP),
		dhcpv4.WithServerIP(h.serverIP),
		dhcpv4.WithOption(dhcpv4.OptSubnetMask(h.mask)),
		dhcpv4.WithOption(dhcpv4.OptRouter(gw)),
		dhcpv4.WithOption(dhcpv4.OptDNS(dns...)),
		dhcpv4.WithOption(dhcpv4.OptIPAddressLeaseTime(h.duration)),
		dhcpv4.WithOption(dhcpv4.OptServerIdentifier(h.serverIP)),
	)
	if err != nil {
		h.logger.Error("failed to build ACK (intercepted)", "error", err)
		return
	}

	if _, err := conn.WriteTo(ack.ToBytes(), peer); err != nil {
		h.logger.Error("failed to send ACK (intercepted)", "error", err)
		return
	}

	h.logger.Info("ACK sent (intercepted)", "mac", mac, "ip", interceptIP, "gateway", gw, "hostname", hostname)
}

func (h *Handler) sendNAK(conn net.PacketConn, peer net.Addr, msg *dhcpv4.DHCPv4, mac string, requestedIP net.IP) {
	nak, err := dhcpv4.NewReplyFromRequest(msg,
		dhcpv4.WithMessageType(dhcpv4.MessageTypeNak),
		dhcpv4.WithServerIP(h.serverIP),
		dhcpv4.WithOption(dhcpv4.OptServerIdentifier(h.serverIP)),
	)
	if err != nil {
		h.logger.Error("failed to build NAK", "error", err)
		return
	}

	if _, err := conn.WriteTo(nak.ToBytes(), peer); err != nil {
		h.logger.Error("failed to send NAK", "error", err)
		return
	}

	h.logger.Warn("NAK sent", "mac", mac, "requested", requestedIP)
}

func (h *Handler) handleRelease(msg *dhcpv4.DHCPv4) {
	mac := msg.ClientHWAddr.String()

	if il, ok := h.intercepts.Lookup(mac); ok && il.Interface == h.ifName {
		h.leases.Remove(mac)
		h.logger.Info("RELEASE processed (intercepted)", "mac", mac)
		return
	}

	if _, err := h.pool.Release(mac); err != nil {
		h.logger.Warn("release for unknown MAC", "mac", mac)
		return
	}

	h.leases.Remove(mac)
	h.logger.Info("RELEASE processed", "mac", mac)
}

func StartServer(ifname string, handler server4.Handler, logger *slog.Logger) (*server4.Server, error) {
	addr := &net.UDPAddr{
		IP:   net.IPv4zero,
		Port: 67,
	}

	srv, err := server4.NewServer(ifname, addr, handler)
	if err != nil {
		return nil, fmt.Errorf("creating DHCP server on %s: %w", ifname, err)
	}

	return srv, nil
}
