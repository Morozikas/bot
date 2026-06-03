// Пакет api — взаимодействие с Bybit V5 API.
// derivatives.go — деривативные эндпоинты: OI, ставка финансирования, соотношение L/S.
package api

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// GetOpenInterest возвращает историю открытого интереса.
func (c *Client) GetOpenInterest(symbol, interval string, limit int) ([]OpenInterest, error) {
	data, err := c.Get("/v5/market/open-interest", map[string]string{
		"category": "linear", "symbol": symbol,
		"intervalTime": interval, "limit": strconv.Itoa(limit),
	})
	if err != nil {
		return nil, err
	}
	var resp struct {
		BaseResponse
		Result struct {
			List []struct {
				OI string `json:"openInterest"`
				Ts string `json:"timestamp"`
			} `json:"list"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	if resp.RetCode != 0 {
		return nil, fmt.Errorf("OI: %s", resp.RetMsg)
	}
	result := make([]OpenInterest, 0, len(resp.Result.List))
	for _, item := range resp.Result.List {
		ts, _ := strconv.ParseInt(item.Ts, 10, 64)
		result = append(result, OpenInterest{Symbol: symbol, OpenInterest: D(item.OI), Timestamp: ts})
	}
	return result, nil
}

// GetFundingHistory возвращает историю ставок финансирования.
func (c *Client) GetFundingHistory(symbol string, limit int) ([]FundingRate, error) {
	data, err := c.Get("/v5/market/funding/history", map[string]string{
		"category": "linear", "symbol": symbol, "limit": strconv.Itoa(limit),
	})
	if err != nil {
		return nil, err
	}
	var resp struct {
		BaseResponse
		Result struct {
			List []struct {
				Symbol string `json:"symbol"`
				Rate   string `json:"fundingRate"`
				Ts     string `json:"fundingRateTimestamp"`
			} `json:"list"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	if resp.RetCode != 0 {
		return nil, fmt.Errorf("funding: %s", resp.RetMsg)
	}
	result := make([]FundingRate, 0, len(resp.Result.List))
	for _, item := range resp.Result.List {
		ts, _ := strconv.ParseInt(item.Ts, 10, 64)
		result = append(result, FundingRate{Symbol: item.Symbol, FundingRate: D(item.Rate), FundingRateTime: ts})
	}
	return result, nil
}

// GetLongShortRatio возвращает соотношение лонг/шорт позиций.
func (c *Client) GetLongShortRatio(symbol, period string, limit int) ([]LongShortRatio, error) {
	data, err := c.Get("/v5/market/account-ratio", map[string]string{
		"category": "linear", "symbol": symbol,
		"period": period, "limit": strconv.Itoa(limit),
	})
	if err != nil {
		return nil, err
	}
	var resp struct {
		BaseResponse
		Result struct {
			List []struct {
				Symbol    string `json:"symbol"`
				BuyRatio  string `json:"buyRatio"`
				SellRatio string `json:"sellRatio"`
				Timestamp string `json:"timestamp"`
			} `json:"list"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	if resp.RetCode != 0 {
		return nil, fmt.Errorf("ls ratio: %s", resp.RetMsg)
	}
	result := make([]LongShortRatio, 0)
	for _, item := range resp.Result.List {
		ts, _ := strconv.ParseInt(item.Timestamp, 10, 64)
		result = append(result, LongShortRatio{
			Symbol: item.Symbol, BuyRatio: D(item.BuyRatio),
			SellRatio: D(item.SellRatio), Timestamp: ts,
		})
	}
	return result, nil
}
