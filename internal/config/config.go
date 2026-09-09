package config

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Interfaces []InterfaceConfig `yaml:"interfaces"`
	API        APIConfig         `yaml:"api"`
}

type InterfaceConfig struct {
	Name     string     `yaml:"name"`
	ServerIP string     `yaml:"server_ip"`
	DHCP     DHCPConfig `yaml:"dhcp"`
	DNS      []string   `yaml:"dns"`
	Gateway  string     `yaml:"gateway"`
}

type DHCPConfig struct {
	RangeStart    string `yaml:"range_start"`
	RangeEnd      string `yaml:"range_end"`
	SubnetMask    string `yaml:"subnet_mask"`
	LeaseDuration int    `yaml:"lease_duration"`
}

type APIConfig struct {
	Listen string `yaml:"listen"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	return &cfg, nil
}

func (c *Config) validate() error {
	if len(c.Interfaces) == 0 {
		return fmt.Errorf("no interfaces configured")
	}

	if c.API.Listen == "" {
		c.API.Listen = ":8080"
	}

	for i, ifc := range c.Interfaces {
		if ifc.Name == "" {
			return fmt.Errorf("interface %d: name is required", i)
		}

		if _, err := net.InterfaceByName(ifc.Name); err != nil {
			return fmt.Errorf("interface %q: %w", ifc.Name, err)
		}

		if net.ParseIP(ifc.ServerIP) == nil {
			return fmt.Errorf("interface %q: invalid server_ip %q", ifc.Name, ifc.ServerIP)
		}

		if net.ParseIP(ifc.Gateway) == nil {
			return fmt.Errorf("interface %q: invalid gateway %q", ifc.Name, ifc.Gateway)
		}

		for j, dns := range ifc.DNS {
			if net.ParseIP(dns) == nil {
				return fmt.Errorf("interface %q: invalid dns[%d] %q", ifc.Name, j, dns)
			}
		}

		start := net.ParseIP(ifc.DHCP.RangeStart)
		if start == nil {
			return fmt.Errorf("interface %q: invalid range_start %q", ifc.Name, ifc.DHCP.RangeStart)
		}

		end := net.ParseIP(ifc.DHCP.RangeEnd)
		if end == nil {
			return fmt.Errorf("interface %q: invalid range_end %q", ifc.Name, ifc.DHCP.RangeEnd)
		}

		s4 := start.To4()
		e4 := end.To4()
		if s4 == nil || e4 == nil {
			return fmt.Errorf("interface %q: only IPv4 ranges are supported", ifc.Name)
		}

		if binary.BigEndian.Uint32(s4) > binary.BigEndian.Uint32(e4) {
			return fmt.Errorf("interface %q: range_start %s is after range_end %s", ifc.Name, ifc.DHCP.RangeStart, ifc.DHCP.RangeEnd)
		}

		if net.ParseIP(ifc.DHCP.SubnetMask) == nil {
			return fmt.Errorf("interface %q: invalid subnet_mask %q", ifc.Name, ifc.DHCP.SubnetMask)
		}

		if ifc.DHCP.LeaseDuration <= 0 {
			return fmt.Errorf("interface %q: lease_duration must be positive", ifc.Name)
		}
	}

	return nil
}
