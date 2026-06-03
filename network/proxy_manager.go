// Пакет network — работа с сетью.
// proxy_manager.go — настройка HTTP-прокси для всех исходящих запросов к бирже.
package network

import (
	"net/http"
	"net/url"
)

// ProxyManager управляет конфигурацией HTTP-прокси.
type ProxyManager struct {
	proxyURL string
}

// NewProxyManager создаёт ProxyManager.
func NewProxyManager(proxyURL string) *ProxyManager {
	return &ProxyManager{proxyURL: proxyURL}
}

// Transport возвращает http.Transport с настроенным прокси (если задан).
func (p *ProxyManager) Transport() *http.Transport {
	t := &http.Transport{}
	if p.proxyURL != "" {
		if pu, err := url.Parse(p.proxyURL); err == nil {
			t.Proxy = http.ProxyURL(pu)
		}
	}
	return t
}

// IsConfigured возвращает true, если прокси URL задан.
func (p *ProxyManager) IsConfigured() bool {
	return p.proxyURL != ""
}

// URL возвращает строку прокси URL.
func (p *ProxyManager) URL() string {
	return p.proxyURL
}
