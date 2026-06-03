// Пакет trading — автоматическое исполнение сделок.
// users.go — менеджер пользователей: API-клиенты, балансы, лимиты позиций.
package trading

import (
	"fmt"
	"sync"

	"bybit-elite-signal/api"
	"bybit-elite-signal/config"
)

// ManagedUser — один торговый аккаунт с собственным клиентом и состоянием.
type ManagedUser struct {
	Config         config.UserConfig
	Client         *api.Client
	OpenPositions  int
	DailyLoss      float64
	InitialBalance float64
	mu             sync.Mutex
}

// UserManager хранит всех активных пользователей системы.
type UserManager struct {
	users     []*ManagedUser
	mu        sync.RWMutex
	globalCfg *config.AppConfig
}

// NewUserManager создаёт менеджер и инициализирует клиентов для всех пользователей.
func NewUserManager(cfg *config.AppConfig, users []config.UserConfig) (*UserManager, error) {
	um := &UserManager{globalCfg: cfg}
	for _, u := range users {
		if !u.Active {
			continue
		}
		baseURL := "https://api.bybit.com"
		if u.Testnet {
			baseURL = "https://api-testnet.bybit.com"
		}
		client, err := api.NewClient(baseURL, u.APIKey, u.APISecret, cfg.Network.ProxyURL)
		if err != nil {
			return nil, fmt.Errorf("пользователь %s: %w", u.ID, err)
		}
		um.users = append(um.users, &ManagedUser{Config: u, Client: client})
	}
	return um, nil
}

// GetUsers возвращает снимок списка пользователей.
func (um *UserManager) GetUsers() []*ManagedUser {
	um.mu.RLock()
	defer um.mu.RUnlock()
	cp := make([]*ManagedUser, len(um.users))
	copy(cp, um.users)
	return cp
}

// AddUser добавляет нового пользователя во время работы системы.
func (um *UserManager) AddUser(u config.UserConfig) error {
	baseURL := "https://api.bybit.com"
	if u.Testnet {
		baseURL = "https://api-testnet.bybit.com"
	}
	client, err := api.NewClient(baseURL, u.APIKey, u.APISecret, um.globalCfg.Network.ProxyURL)
	if err != nil {
		return err
	}
	um.mu.Lock()
	defer um.mu.Unlock()
	um.users = append(um.users, &ManagedUser{Config: u, Client: client})
	return nil
}

// RemoveUser удаляет пользователя по ID.
func (um *UserManager) RemoveUser(id string) {
	um.mu.Lock()
	defer um.mu.Unlock()
	filtered := um.users[:0]
	for _, u := range um.users {
		if u.Config.ID != id {
			filtered = append(filtered, u)
		}
	}
	um.users = filtered
}

// RefreshBalances обновляет начальные балансы всех пользователей.
func (um *UserManager) RefreshBalances() {
	for _, u := range um.GetUsers() {
		if bal, err := u.Client.GetBalance(); err == nil {
			u.mu.Lock()
			u.InitialBalance, _ = bal.TotalEquity.Float64()
			u.mu.Unlock()
		}
	}
}

func (u *ManagedUser) IncrementPositions() {
	u.mu.Lock()
	u.OpenPositions++
	u.mu.Unlock()
}

func (u *ManagedUser) DecrementPositions() {
	u.mu.Lock()
	if u.OpenPositions > 0 {
		u.OpenPositions--
	}
	u.mu.Unlock()
}

func (u *ManagedUser) AddDailyLoss(amount float64) {
	u.mu.Lock()
	u.DailyLoss += amount
	u.mu.Unlock()
}

// GetRiskPct возвращает эффективный % риска на сделку для пользователя.
func (u *ManagedUser) GetRiskPct(globalDefault float64) float64 {
	if u.Config.RiskPct > 0 {
		return u.Config.RiskPct / 100
	}
	return globalDefault / 100
}

// GetLeverage возвращает плечо для символа с учётом пользовательских настроек.
func (u *ManagedUser) GetLeverage(symbol string, globalDefault int) int {
	if lev, ok := u.Config.SymbolLeverage[symbol]; ok && lev > 0 {
		return lev
	}
	if u.Config.MaxLeverage > 0 {
		return u.Config.MaxLeverage
	}
	return globalDefault
}
