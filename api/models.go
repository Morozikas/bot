// Пакет api — взаимодействие с Bybit V5 API.
// models.go — все типы данных: свечи, стакан, сделки, деривативы, ордера, позиции.
package api

import "github.com/shopspring/decimal"

type BaseResponse struct {
	RetCode int    `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Time    int64  `json:"time"`
}

// Candle — одна OHLCV свеча.
type Candle struct {
	StartTime int64
	Open      decimal.Decimal
	High      decimal.Decimal
	Low       decimal.Decimal
	Close     decimal.Decimal
	Volume    decimal.Decimal
	Turnover  decimal.Decimal
}

// OrderBookLevel — один уровень стакана.
type OrderBookLevel struct {
	Price    decimal.Decimal
	Quantity decimal.Decimal
}

// OrderBook — снимок стакана заявок.
type OrderBook struct {
	Symbol    string
	Bids      []OrderBookLevel
	Asks      []OrderBookLevel
	Timestamp int64
	UpdateID  int64
}

// Trade — публичная сделка из ленты.
type Trade struct {
	TradeID   string
	Symbol    string
	Side      string
	Price     decimal.Decimal
	Quantity  decimal.Decimal
	Timestamp int64
	IsBuyer   bool
}

// Ticker — агрегированные данные символа.
type Ticker struct {
	Symbol          string
	LastPrice       decimal.Decimal
	MarkPrice       decimal.Decimal
	IndexPrice      decimal.Decimal
	OpenInterest    decimal.Decimal
	OpenInterestVal decimal.Decimal
	FundingRate     decimal.Decimal
	NextFundingTime int64
	Volume24h       decimal.Decimal
	Turnover24h     decimal.Decimal
	Price24hPct     decimal.Decimal
	High24h         decimal.Decimal
	Low24h          decimal.Decimal
	Bid1Price       decimal.Decimal
	Ask1Price       decimal.Decimal
}

// InstrumentInfo — спецификация инструмента.
type InstrumentInfo struct {
	Symbol         string
	BaseCoin       string
	QuoteCoin      string
	Status         string
	PriceFilter    PriceFilter
	LotSizeFilter  LotSizeFilter
	LeverageFilter LeverageFilter
}

type PriceFilter struct {
	MinPrice decimal.Decimal
	MaxPrice decimal.Decimal
	TickSize decimal.Decimal
}

type LotSizeFilter struct {
	MaxOrderQty decimal.Decimal
	MinOrderQty decimal.Decimal
	QtyStep     decimal.Decimal
}

type LeverageFilter struct {
	MinLeverage  decimal.Decimal
	MaxLeverage  decimal.Decimal
	LeverageStep decimal.Decimal
}

// OpenInterest — запись истории OI.
type OpenInterest struct {
	Symbol       string
	OpenInterest decimal.Decimal
	Timestamp    int64
}

// FundingRate — запись истории ставки финансирования.
type FundingRate struct {
	Symbol          string
	FundingRate     decimal.Decimal
	FundingRateTime int64
}

// LongShortRatio — соотношение лонг/шорт позиций.
type LongShortRatio struct {
	Symbol    string
	BuyRatio  decimal.Decimal
	SellRatio decimal.Decimal
	Timestamp int64
}

// LiquidationRecord — запись принудительной ликвидации.
type LiquidationRecord struct {
	Symbol    string
	Side      string
	Price     decimal.Decimal
	Qty       decimal.Decimal
	Timestamp int64
}

// OrderRequest — параметры нового ордера.
type OrderRequest struct {
	Symbol         string
	Side           string
	OrderType      string
	Qty            string
	Price          string
	StopLoss       string
	TakeProfit     string
	Leverage       int
	TimeInForce    string
	ReduceOnly     bool
	CloseOnTrigger bool
	PositionIdx    int
}

// OrderResponse — ответ на размещение ордера.
type OrderResponse struct {
	OrderID     string
	OrderLinkID string
	Symbol      string
	Side        string
	OrderType   string
	Price       decimal.Decimal
	Qty         decimal.Decimal
	Status      string
	CreatedTime int64
}

// Position — открытая позиция.
type Position struct {
	Symbol           string
	Side             string
	Size             decimal.Decimal
	AvgPrice         decimal.Decimal
	MarkPrice        decimal.Decimal
	UnrealizedPnl    decimal.Decimal
	RealizedPnl      decimal.Decimal
	StopLoss         decimal.Decimal
	TakeProfit       decimal.Decimal
	Leverage         decimal.Decimal
	LiquidationPrice decimal.Decimal
	CreatedTime      int64
	UpdatedTime      int64
}

// AccountBalance — баланс аккаунта.
type AccountBalance struct {
	TotalEquity           decimal.Decimal
	TotalWalletBalance    decimal.Decimal
	TotalAvailableBalance decimal.Decimal
	TotalInitialMargin    decimal.Decimal
	TotalMaintMargin      decimal.Decimal
	Coins                 []CoinBalance
}

// CoinBalance — баланс одной монеты.
type CoinBalance struct {
	Coin                string
	Equity              decimal.Decimal
	WalletBalance       decimal.Decimal
	AvailableToWithdraw decimal.Decimal
	UnrealizedPnl       decimal.Decimal
}

// D конвертирует строку в decimal.Decimal.
func D(s string) decimal.Decimal {
	if s == "" {
		return decimal.Zero
	}
	d, _ := decimal.NewFromString(s)
	return d
}
