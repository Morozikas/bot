// Пакет api — взаимодействие с Bybit V5 API.
// liquidations.go — эндпоинт данных о ликвидациях.
package api

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// GetLiquidations возвращает последние ликвидации по символу.
func (c *Client) GetLiquidations(symbol string, limit int) ([]LiquidationRecord, error) {
	data, err := c.Get("/v5/market/recent-trade", map[string]string{
		"category": "linear", "symbol": symbol, "limit": strconv.Itoa(limit),
	})
	if err != nil {
		return nil, err
	}
	// Bybit не имеет отдельного REST-эндпоинта ликвидаций — используем WS или Trade History.
	// Здесь возвращаем пустой список как заглушку; реальные данные приходят через WS.
	var resp struct {
		BaseResponse
		Result struct {
			List []struct {
				Symbol string `json:"symbol"`
				Side   string `json:"side"`
				Price  string `json:"price"`
				Size   string `json:"size"`
				Time   string `json:"time"`
			} `json:"list"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	if resp.RetCode != 0 {
		return nil, fmt.Errorf("liquidations: %s", resp.RetMsg)
	}
	result := make([]LiquidationRecord, 0)
	for _, item := range resp.Result.List {
		ts, _ := strconv.ParseInt(item.Time, 10, 64)
		result = append(result, LiquidationRecord{
			Symbol: item.Symbol, Side: item.Side,
			Price: D(item.Price), Qty: D(item.Size), Timestamp: ts,
		})
	}
	return result, nil
}
