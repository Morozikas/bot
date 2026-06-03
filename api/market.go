// Пакет api — взаимодействие с Bybit V5 API.
// market.go — рыночные эндпоинты: свечи, тикеры, инструменты, последние сделки.
package api

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// GetCandles возвращает OHLCV-свечи.
// interval: "1" "5" "15" "30" "60" "240" "D" "W"
func (c *Client) GetCandles(symbol, interval string, limit int) ([]Candle, error) {
	data, err := c.Get("/v5/market/kline", map[string]string{
		"category": "linear", "symbol": symbol,
		"interval": interval, "limit": strconv.Itoa(limit),
	})
	if err != nil {
		return nil, err
	}
	var resp struct {
		BaseResponse
		Result struct {
			List [][]string `json:"list"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	if resp.RetCode != 0 {
		return nil, fmt.Errorf("candles: %s", resp.RetMsg)
	}
	items := make([]Candle, 0, len(resp.Result.List))
	for _, row := range resp.Result.List {
		if len(row) < 7 {
			continue
		}
		ts, _ := strconv.ParseInt(row[0], 10, 64)
		items = append(items, Candle{
			StartTime: ts,
			Open:  D(row[1]), High: D(row[2]), Low: D(row[3]),
			Close: D(row[4]), Volume: D(row[5]), Turnover: D(row[6]),
		})
	}
	return items, nil
}

// GetTickers возвращает тикеры (пустой symbol = все линейные бессрочные).
func (c *Client) GetTickers(symbol string) ([]Ticker, error) {
	params := map[string]string{"category": "linear"}
	if symbol != "" {
		params["symbol"] = symbol
	}
	data, err := c.Get("/v5/market/tickers", params)
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
		return nil, fmt.Errorf("tickers: %s", resp.RetMsg)
	}
	tickers := make([]Ticker, 0, len(resp.Result.List))
	for _, t := range resp.Result.List {
		nft, _ := strconv.ParseInt(t["nextFundingTime"], 10, 64)
		tickers = append(tickers, Ticker{
			Symbol: t["symbol"], LastPrice: D(t["lastPrice"]),
			MarkPrice: D(t["markPrice"]), IndexPrice: D(t["indexPrice"]),
			OpenInterest: D(t["openInterest"]), OpenInterestVal: D(t["openInterestValue"]),
			FundingRate: D(t["fundingRate"]), NextFundingTime: nft,
			Volume24h: D(t["volume24h"]), Turnover24h: D(t["turnover24h"]),
			Price24hPct: D(t["price24hPcnt"]),
			High24h: D(t["highPrice24h"]), Low24h: D(t["lowPrice24h"]),
			Bid1Price: D(t["bid1Price"]), Ask1Price: D(t["ask1Price"]),
		})
	}
	return tickers, nil
}

// GetRecentTrades возвращает последние публичные сделки.
func (c *Client) GetRecentTrades(symbol string, limit int) ([]Trade, error) {
	data, err := c.Get("/v5/market/recent-trade", map[string]string{
		"category": "linear", "symbol": symbol, "limit": strconv.Itoa(limit),
	})
	if err != nil {
		return nil, err
	}
	var resp struct {
		BaseResponse
		Result struct {
			List []struct {
				ExecID string `json:"execId"`
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
		return nil, fmt.Errorf("trades: %s", resp.RetMsg)
	}
	trades := make([]Trade, 0, len(resp.Result.List))
	for _, t := range resp.Result.List {
		ts, _ := strconv.ParseInt(t.Time, 10, 64)
		trades = append(trades, Trade{
			TradeID: t.ExecID, Symbol: t.Symbol, Side: t.Side,
			Price: D(t.Price), Quantity: D(t.Size),
			Timestamp: ts, IsBuyer: t.Side == "Buy",
		})
	}
	return trades, nil
}

// GetInstruments возвращает все торгующиеся USDT-бессрочные контракты.
func (c *Client) GetInstruments() ([]InstrumentInfo, error) {
	data, err := c.Get("/v5/market/instruments-info", map[string]string{
		"category": "linear", "limit": "1000",
	})
	if err != nil {
		return nil, err
	}
	var resp2 struct {
		BaseResponse
		Result struct {
			List []struct {
				Symbol    string `json:"symbol"`
				BaseCoin  string `json:"baseCoin"`
				QuoteCoin string `json:"quoteCoin"`
				Status    string `json:"status"`
				PriceFilter struct {
					MinPrice string `json:"minPrice"`
					MaxPrice string `json:"maxPrice"`
					TickSize string `json:"tickSize"`
				} `json:"priceFilter"`
				LotSizeFilter struct {
					MaxOrderQty string `json:"maxOrderQty"`
					MinOrderQty string `json:"minOrderQty"`
					QtyStep     string `json:"qtyStep"`
				} `json:"lotSizeFilter"`
				LeverageFilter struct {
					MinLeverage  string `json:"minLeverage"`
					MaxLeverage  string `json:"maxLeverage"`
					LeverageStep string `json:"leverageStep"`
				} `json:"leverageFilter"`
			} `json:"list"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &resp2); err != nil {
		return nil, err
	}
	result := make([]InstrumentInfo, 0)
	for _, i := range resp2.Result.List {
		if i.Status != "Trading" || i.QuoteCoin != "USDT" {
			continue
		}
		result = append(result, InstrumentInfo{
			Symbol: i.Symbol, BaseCoin: i.BaseCoin, QuoteCoin: i.QuoteCoin, Status: i.Status,
			PriceFilter:   PriceFilter{MinPrice: D(i.PriceFilter.MinPrice), MaxPrice: D(i.PriceFilter.MaxPrice), TickSize: D(i.PriceFilter.TickSize)},
			LotSizeFilter: LotSizeFilter{MaxOrderQty: D(i.LotSizeFilter.MaxOrderQty), MinOrderQty: D(i.LotSizeFilter.MinOrderQty), QtyStep: D(i.LotSizeFilter.QtyStep)},
			LeverageFilter: LeverageFilter{MinLeverage: D(i.LeverageFilter.MinLeverage), MaxLeverage: D(i.LeverageFilter.MaxLeverage), LeverageStep: D(i.LeverageFilter.LeverageStep)},
		})
	}
	return result, nil
}
