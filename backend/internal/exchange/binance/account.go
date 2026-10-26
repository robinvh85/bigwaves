package binance

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"bigwaves/internal/platform/money"
)

type SpotBalance struct {
	Asset  string    `json:"asset"`
	Free   money.Dec `json:"free"`
	Locked money.Dec `json:"locked"`
}

// SpotBalances returns the non-zero balances of the spot wallet.
func (c *Client) SpotBalances(ctx context.Context) ([]SpotBalance, error) {
	var out struct {
		Balances []SpotBalance `json:"balances"`
	}
	err := c.do(ctx, request{
		method: http.MethodGet, path: "/api/v3/account",
		params: url.Values{"omitZeroBalances": {"true"}}, security: signed,
	}, &out)
	return out.Balances, err
}

type MarginAsset struct {
	Asset    string    `json:"asset"`
	Free     money.Dec `json:"free"`
	Locked   money.Dec `json:"locked"`
	Borrowed money.Dec `json:"borrowed"`
	Interest money.Dec `json:"interest"`
	NetAsset money.Dec `json:"netAsset"`
}

// Debt is what has to be repaid for this asset: principal plus accrued interest.
func (a MarginAsset) Debt() money.Dec { return a.Borrowed.Add(a.Interest) }

type MarginAccount struct {
	MarginLevel         money.Dec     `json:"marginLevel"`
	TotalAssetOfBtc     money.Dec     `json:"totalAssetOfBtc"`
	TotalLiabilityOfBtc money.Dec     `json:"totalLiabilityOfBtc"`
	TotalNetAssetOfBtc  money.Dec     `json:"totalNetAssetOfBtc"`
	TradeEnabled        bool          `json:"tradeEnabled"`
	BorrowEnabled       bool          `json:"borrowEnabled"`
	UserAssets          []MarginAsset `json:"userAssets"`
}

// Asset returns one asset of the cross-margin account, zero if absent.
func (a MarginAccount) Asset(name string) MarginAsset {
	for _, ua := range a.UserAssets {
		if ua.Asset == name {
			return ua
		}
	}
	return MarginAsset{Asset: name}
}

// CrossMarginAccount returns the cross-margin account.
func (c *Client) CrossMarginAccount(ctx context.Context) (MarginAccount, error) {
	var out MarginAccount
	err := c.do(ctx, request{method: http.MethodGet, path: "/sapi/v1/margin/account", security: signed}, &out)
	return out, err
}

type LoanType string

const (
	Borrow LoanType = "BORROW"
	Repay  LoanType = "REPAY"
)

// BorrowRepay borrows or repays asset on cross margin (or isolated when symbol is set and
// isolated is true). It is a write call.
func (c *Client) BorrowRepay(ctx context.Context, typ LoanType, asset string, amount money.Dec, isolated bool, symbol string) (tranID int64, err error) {
	p := url.Values{
		"type":       {string(typ)},
		"asset":      {asset},
		"amount":     {amount.String()},
		"isIsolated": {strconv.FormatBool(isolated)},
	}
	if isolated {
		p.Set("symbol", symbol)
	}
	var out struct {
		TranID int64 `json:"tranId"`
	}
	err = c.do(ctx, request{method: http.MethodPost, path: "/sapi/v1/margin/borrow-repay", params: p, security: signed, write: true}, &out)
	return out.TranID, err
}

type LoanRecord struct {
	Type      LoanType  `json:"type"`
	Asset     string    `json:"asset"`
	Amount    money.Dec `json:"amount"`
	Principal money.Dec `json:"principal"`
	Interest  money.Dec `json:"interest"`
	Status    string    `json:"status"` // PENDING, CONFIRMED, FAILED
	Timestamp int64     `json:"timestamp"`
	TxID      int64     `json:"txId"`
}

// LoanRecords returns BORROW or REPAY records for asset since start (up to 100).
func (c *Client) LoanRecords(ctx context.Context, typ LoanType, asset string, start time.Time) ([]LoanRecord, error) {
	p := url.Values{
		"type":      {string(typ)},
		"asset":     {asset},
		"startTime": {strconv.FormatInt(start.UnixMilli(), 10)},
		"size":      {"100"},
	}
	var out struct {
		Rows []LoanRecord `json:"rows"`
	}
	err := c.do(ctx, request{method: http.MethodGet, path: "/sapi/v1/margin/borrow-repay", params: p, security: signed}, &out)
	return out.Rows, err
}

type TransferDirection string

const (
	SpotToCross TransferDirection = "MAIN_MARGIN"
	CrossToSpot TransferDirection = "MARGIN_MAIN"
)

// Transfer moves asset between the spot wallet and cross margin. It is a write call.
func (c *Client) Transfer(ctx context.Context, dir TransferDirection, asset string, amount money.Dec) (tranID int64, err error) {
	p := url.Values{"type": {string(dir)}, "asset": {asset}, "amount": {amount.String()}}
	var out struct {
		TranID int64 `json:"tranId"`
	}
	err = c.do(ctx, request{method: http.MethodPost, path: "/sapi/v1/asset/transfer", params: p, security: signed, write: true}, &out)
	return out.TranID, err
}

type MarginPair struct {
	Symbol        string `json:"symbol"`
	Base          string `json:"base"`
	Quote         string `json:"quote"`
	IsMarginTrade bool   `json:"isMarginTrade"`
	IsBuyAllowed  bool   `json:"isBuyAllowed"`
	IsSellAllowed bool   `json:"isSellAllowed"`
}

// CrossMarginPairs lists every cross-margin symbol.
func (c *Client) CrossMarginPairs(ctx context.Context) ([]MarginPair, error) {
	var out []MarginPair
	err := c.do(ctx, request{method: http.MethodGet, path: "/sapi/v1/margin/allPairs", security: signed}, &out)
	return out, err
}

type MarginAssetInfo struct {
	AssetName    string `json:"assetName"`
	IsBorrowable bool   `json:"isBorrowable"`
	IsMortgage   bool   `json:"isMortgageable"`
}

// CrossMarginAssets lists every cross-margin asset and whether it can be borrowed.
func (c *Client) CrossMarginAssets(ctx context.Context) ([]MarginAssetInfo, error) {
	var out []MarginAssetInfo
	err := c.do(ctx, request{method: http.MethodGet, path: "/sapi/v1/margin/allAssets", security: signed}, &out)
	return out, err
}

// MaxBorrowable is how much of asset the account could borrow right now.
func (c *Client) MaxBorrowable(ctx context.Context, asset string) (money.Dec, error) {
	var out struct {
		Amount money.Dec `json:"amount"`
	}
	err := c.do(ctx, request{method: http.MethodGet, path: "/sapi/v1/margin/maxBorrowable", params: url.Values{"asset": {asset}}, security: signed}, &out)
	return out.Amount, err
}

// DailyInterestRate returns the latest daily borrow rate of asset (e.g. 0.0001 = 0.01%/day).
func (c *Client) DailyInterestRate(ctx context.Context, asset string, since time.Time) (money.Dec, bool, error) {
	p := url.Values{"asset": {asset}, "startTime": {strconv.FormatInt(since.UnixMilli(), 10)}}
	var out []struct {
		DailyInterestRate money.Dec `json:"dailyInterestRate"`
		Timestamp         int64     `json:"timestamp"`
	}
	if err := c.do(ctx, request{method: http.MethodGet, path: "/sapi/v1/margin/interestRateHistory", params: p, security: signed}, &out); err != nil {
		return money.Zero, false, err
	}
	if len(out) == 0 {
		return money.Zero, false, nil
	}
	// Newest first.
	return out[0].DailyInterestRate, true, nil
}
