// Пакет network — работа с сетью.
// firewall.go — белый список IP/доменов для исходящих подключений; блокировка нежелательных адресов.
package network

import (
	"fmt"
	"net"
	"strings"
)

// Firewall управляет разрешёнными исходящими подключениями.
type Firewall struct {
	allowedHosts []string
	blockedCIDRs []*net.IPNet
}

// NewFirewall создаёт Firewall с заданными разрешёнными хостами.
func NewFirewall(allowedHosts []string) *Firewall {
	return &Firewall{allowedHosts: allowedHosts}
}

// BlockCIDR добавляет CIDR-блок в список заблокированных.
func (f *Firewall) BlockCIDR(cidr string) error {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("неверный CIDR: %w", err)
	}
	f.blockedCIDRs = append(f.blockedCIDRs, ipnet)
	return nil
}

// IsAllowed проверяет, разрешён ли хост.
func (f *Firewall) IsAllowed(host string) bool {
	if len(f.allowedHosts) == 0 {
		return true
	}
	for _, allowed := range f.allowedHosts {
		if strings.HasSuffix(host, allowed) {
			return true
		}
	}
	return false
}

// IsIPBlocked проверяет, заблокирован ли IP-адрес.
func (f *Firewall) IsIPBlocked(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	for _, cidr := range f.blockedCIDRs {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

// DefaultBybitFirewall создаёт файрвол, разрешающий только домены Bybit.
func DefaultBybitFirewall() *Firewall {
	return NewFirewall([]string{"bybit.com", "stream.bybit.com"})
}
