// Пакет api — взаимодействие с Bybit V5 API.
// trading.go — приватные торговые эндпоинты: ордера, позиции, баланс, плечо.
package api

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// PlaceOrder размещает новый ордер.
func (c *Client) PlaceOrder(req OrderRequest) (*OrderResponse, error) {
	body := map[string]interface{}{
		"category": "linear", "symbol": req.Symbol,
		"side": req.Side, "orderType": req.OrderType,
		"qty": req.Qty, "timeInForce": req.TimeInForce,
		"positionIdx": req.PositionIdx,
	}
	if req.Price != "" {
		body["price"] = req.Price
	}
	if req.StopLoss != "" {
		body["stopLoss"] = req.StopLoss
	}
	if req.TakeProfit != "" {
		body["takeProfit"] = req.TakeProfit
	}
	if req.ReduceOnly {
		body["reduceOnly"] = true
	}
	data, err := c.AuthPost("/v5/order/create", body)
	if err != nil {
		return nil, err
	}
	var resp struct {
		BaseResponse
		Result struct {
			OrderID     string `json:"orderId"`
			OrderLinkID string `json:"orderLinkId"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	if resp.RetCode != 0 {
		return nil, fmt.Errorf("place order: %s (%d)", resp.RetMsg, resp.RetCode)
	}
	return &OrderResponse{
		OrderID: resp.Result.OrderID, OrderLinkID: resp.Result.OrderLinkID,
		Symbol: req.Symbol, Side: req.Side, OrderType: req.OrderType,
	}, nil
}

// CancelOrder отменяет ордер по ID.
func (c *Client) CancelOrder(symbol, orderID string) error {
	data, err := c.AuthPost("/v5/order/cancel", map[string]interface{}{
		"category": "linear", "symbol": symbol, "orderId": orderID,
	})
	if err != nil {
		return err
	}
	var resp struct{ BaseResponse }
	if err := json.Unmarshal(data, &resp); err != nil {
		return err
	}
	if resp.RetCode != 0 {
		return fmt.Errorf("cancel: %s", resp.RetMsg)
	}
	return nil
}

// SetLeverage устанавливает плечо для символа.
func (c *Client) SetLeverage(symbol string, leverage int) error {
	lev := strconv.Itoa(leverage)
	data, err := c.AuthPost("/v5/position/set-leverage", map[string]interface{}{
		"category": "linear", "symbol": symbol,
		"buyLeverage": lev, "sellLeverage": lev,
	})
	if err != nil {
		return err
	}
	var resp struct{ BaseResponse }
	if err := json.Unmarshal(data, &resp); err != nil {
		return err
	}
	if resp.RetCode != 0 && resp.RetCode != 110043 {
		return fmt.Errorf("set leverage: %s", resp.RetMsg)
	}
	return nil
}

// GetPositions возвращает открытые позиции.
func (c *Client) GetPositions(symbol string) ([]Position, error) {
	params := map[string]string{"category": "linear", "limit": "200"}
	if symbol != "" {
		params["symbol"] = symbol
	}
	data, err := c.AuthGet("/v5/position/list", params)
	if err != nil {
		return nil, err
	}
	var resp struct {
		BaseResponse
		Result struct {
			List []map[string]string `json:"list"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	if resp.RetCode != 0 {
		return nil, fmt.Errorf("positions: %s", resp.RetMsg)
	}
	result := make([]Position, 0)
	for _, p := range resp.Result.List {
		if D(p["size"]).IsZero() {
			continue
		}
		ct, _ := strconv.ParseInt(p["createdTime"], 10, 64)
		ut, _ := strconv.ParseInt(p["updatedTime"], 10, 64)
		result = append(result, Position{
			Symbol: p["symbol"], Side: p["side"], Size: D(p["size"]),
			AvgPrice: D(p["avgPrice"]), MarkPrice: D(p["markPrice"]),
			UnrealizedPnl: D(p["unrealisedPnl"]), RealizedPnl: D(p["cumRealisedPnl"]),
			StopLoss: D(p["stopLoss"]), TakeProfit: D(p["takeProfit"]),
			Leverage: D(p["leverage"]), LiquidationPrice: D(p["liqPrice"]),
			CreatedTime: ct, UpdatedTime: ut,
		})
	}
	return result, nil
}

// GetBalance возвращает баланс унифицированного аккаунта.
func (c *Client) GetBalance() (*AccountBalance, error) {
	data, err := c.AuthGet("/v5/account/wallet-balance", map[string]string{"accountType": "UNIFIED"})
	if err != nil {
		return nil, err
	}
	var resp struct {
		BaseResponse
		Result struct {
			List []struct {
				TotalEquity            string `json:"totalEquity"`
				TotalWalletBalance     string `json:"totalWalletBalance"`
				TotalAvailableBalance  string `json:"totalAvailableBalance"`
				TotalInitialMargin     string `json:"totalInitialMargin"`
				TotalMaintenanceMargin string `json:"totalMaintenanceMargin"`
				Coin []struct {
					Coin                string `json:"coin"`
					Equity              string `json:"equity"`
					WalletBalance       string `json:"walletBalance"`
					AvailableToWithdraw string `json:"availableToWithdraw"`
					UnrealisedPnl       string `json:"unrealisedPnl"`
				} `json:"coin"`
			} `json:"list"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	if resp.RetCode != 0 {
		return nil, fmt.Errorf("balance: %s", resp.RetMsg)
	}
	if len(resp.Result.List) == 0 {
		return &AccountBalance{}, nil
	}
	acc := resp.Result.List[0]
	bal := &AccountBalance{
		TotalEquity: D(acc.TotalEquity), TotalWalletBalance: D(acc.TotalWalletBalance),
		TotalAvailableBalance: D(acc.TotalAvailableBalance),
		TotalInitialMargin: D(acc.TotalInitialMargin), TotalMaintMargin: D(acc.TotalMaintenanceMargin),
	}
	for _, coin := range acc.Coin {
		bal.Coins = append(bal.Coins, CoinBalance{
			Coin: coin.Coin, Equity: D(coin.Equity), WalletBalance: D(coin.WalletBalance),
			AvailableToWithdraw: D(coin.AvailableToWithdraw), UnrealizedPnl: D(coin.UnrealisedPnl),
		})
	}
	return bal, nil
}
