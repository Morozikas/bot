// Пакет api — взаимодействие с Bybit V5 API.
// orderbook.go — эндпоинты стакана заявок.
package api

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// GetOrderBook возвращает снимок стакана (depth: 1-500).
func (c *Client) GetOrderBook(symbol string, depth int) (*OrderBook, error) {
	data, err := c.Get("/v5/market/orderbook", map[string]string{
		"category": "linear", "symbol": symbol, "limit": strconv.Itoa(depth),
	})
	if err != nil {
		return nil, err
	}
	var resp struct {
		BaseResponse
		Result struct {
			S  string     `json:"s"`
			B  [][]string `json:"b"`
			A  [][]string `json:"a"`
			Ts int64      `json:"ts"`
			U  int64      `json:"u"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	if resp.RetCode != 0 {
		return nil, fmt.Errorf("orderbook: %s", resp.RetMsg)
	}
	ob := &OrderBook{Symbol: resp.Result.S, Timestamp: resp.Result.Ts, UpdateID: resp.Result.U}
	for _, b := range resp.Result.B {
		ob.Bids = append(ob.Bids, OrderBookLevel{Price: D(b[0]), Quantity: D(b[1])})
	}
	for _, a := range resp.Result.A {
		ob.Asks = append(ob.Asks, OrderBookLevel{Price: D(a[0]), Quantity: D(a[1])})
	}
	return ob, nil
}
