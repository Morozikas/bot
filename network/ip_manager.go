// Пакет network — работа с сетью: IP, прокси, rate-limit, безопасность, файрвол.
// ip_manager.go — управление IP-адресами: ротация, мониторинг, блокировки.
package network

import (
	"fmt"
	"net"
	"sync"
	"time"
)

// IPEntry — один IP-адрес с метаданными использования.
type IPEntry struct {
	Address    string
	LastUsed   time.Time
	ErrorCount int
	Blocked    bool
}

// IPManager управляет пулом IP-адресов / прокси.
type IPManager struct {
	mu      sync.Mutex
	entries []*IPEntry
	current int
}

// NewIPManager создаёт менеджер с заданным списком адресов.
func NewIPManager(addresses []string) *IPManager {
	entries := make([]*IPEntry, 0, len(addresses))
	for _, addr := range addresses {
		entries = append(entries, &IPEntry{Address: addr})
	}
	return &IPManager{entries: entries}
}

// Next возвращает следующий доступный IP по round-robin.
func (m *IPManager) Next() (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.entries) == 0 {
		return "", fmt.Errorf("нет доступных IP-адресов")
	}
	for i := 0; i < len(m.entries); i++ {
		idx := (m.current + i) % len(m.entries)
		e := m.entries[idx]
		if !e.Blocked {
			e.LastUsed = time.Now()
			m.current = (idx + 1) % len(m.entries)
			return e.Address, nil
		}
	}
	return "", fmt.Errorf("все IP-адреса заблокированы")
}

// ReportError увеличивает счётчик ошибок; блокирует адрес при > 5 ошибках.
func (m *IPManager) ReportError(address string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range m.entries {
		if e.Address == address {
			e.ErrorCount++
			if e.ErrorCount > 5 {
				e.Blocked = true
			}
			return
		}
	}
}

// Unblock снимает блокировку с адреса.
func (m *IPManager) Unblock(address string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range m.entries {
		if e.Address == address {
			e.Blocked = false
			e.ErrorCount = 0
			return
		}
	}
}

// GetLocalIP возвращает основной локальный IP.
func GetLocalIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String()
}
