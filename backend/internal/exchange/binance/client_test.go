package binance

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"bigwaves/internal/platform/money"
)

func newTest(t *testing.T, h http.HandlerFunc, trading bool) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return New(Config{BaseURL: srv.URL, APIKey: "key", SecretKey: "secret", TradingEnabled: trading})
}

func TestSignedRequestCarriesValidSignature(t *testing.T) {
	c := newTest(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "key", r.Header.Get("X-MBX-APIKEY"))
		raw := r.URL.RawQuery
		i := strings.LastIndex(raw, "&signature=")
		require.Positive(t, i)
		require.Equal(t, sign("secret", raw[:i]), raw[i+len("&signature="):])
		q, _ := url.ParseQuery(raw)
		require.NotEmpty(t, q.Get("timestamp"))
		_, _ = w.Write([]byte(`{"marginLevel":"2.5","userAssets":[{"asset":"USDT","free":"10.5","locked":"0","borrowed":"1","interest":"0.01","netAsset":"9.49"}]}`))
	}, false)

	acc, err := c.CrossMarginAccount(context.Background())
	require.NoError(t, err)
	require.Equal(t, "2.5", acc.MarginLevel.String())
	require.Equal(t, "1.01", acc.Asset("USDT").Debt().String())
	require.True(t, acc.Asset("BTC").Free.IsZero())
}

func TestWritesBlockedWhenTradingDisabled(t *testing.T) {
	var calls atomic.Int32
	c := newTest(t, func(w http.ResponseWriter, _ *http.Request) { calls.Add(1) }, false)

	_, err := c.PlaceMarketOrder(context.Background(), MarketOrder{Account: Cross, Symbol: "SUNUSDT", Side: Buy, QuoteQty: money.FromInt(10)})
	require.ErrorIs(t, err, ErrTradingDisabled)
	_, err = c.BorrowRepay(context.Background(), Borrow, "SUN", money.FromInt(1), false, "")
	require.ErrorIs(t, err, ErrTradingDisabled)
	_, err = c.Transfer(context.Background(), SpotToCross, "USDT", money.FromInt(1))
	require.ErrorIs(t, err, ErrTradingDisabled)
	require.Zero(t, calls.Load(), "nothing may reach the exchange")
}

func TestAPIErrorsAreTyped(t *testing.T) {
	c := newTest(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"code":-1121,"msg":"Invalid symbol."}`))
	}, true)

	_, err := c.ExchangeInfo(context.Background(), "NOPEUSDT")
	require.True(t, HasCode(err, CodeBadSymbol))
	require.False(t, IsTransient(err))
	var apiErr *APIError
	require.True(t, errors.As(err, &apiErr))
	require.Contains(t, apiErr.Request, "symbol=NOPEUSDT")
}

func TestTransientErrors(t *testing.T) {
	require.True(t, IsTransient(&APIError{HTTPStatus: 400, Code: CodeMarginBusy}))
	require.True(t, IsTransient(&APIError{HTTPStatus: 503, Code: 0}))
	require.False(t, IsTransient(&APIError{HTTPStatus: 400, Code: -2010}))
	require.True(t, IsTransient(&url.Error{Op: "Get", URL: "x", Err: errors.New("connection reset")}))
}

func TestOrderLookupMissingIsNotAnError(t *testing.T) {
	c := newTest(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"code":-2013,"msg":"Order does not exist."}`))
	}, true)
	_, found, err := c.OrderByClientID(context.Background(), Cross, "SUNUSDT", "bw_x")
	require.NoError(t, err)
	require.False(t, found)
}

func TestPlaceOrderParamsAndAvgPrice(t *testing.T) {
	c := newTest(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/sapi/v1/margin/order", r.URL.Path)
		q := r.URL.Query()
		require.Equal(t, "false", q.Get("isIsolated"))
		require.Equal(t, "MARKET", q.Get("type"))
		require.Equal(t, "12.5", q.Get("quoteOrderQty"))
		require.Equal(t, "bw_ema1d_7_open", q.Get("newClientOrderId"))
		_, _ = w.Write([]byte(`{"symbol":"SUNUSDT","orderId":1,"clientOrderId":"bw_ema1d_7_open","status":"FILLED",
			"executedQty":"500","cummulativeQuoteQty":"12.5","transactTime":1700000000000,
			"fills":[{"price":"0.025","qty":"500","commission":"0.5","commissionAsset":"SUN"}]}`))
	}, true)

	o, err := c.PlaceMarketOrder(context.Background(), MarketOrder{
		Account: Cross, Symbol: "SUNUSDT", Side: Buy, QuoteQty: money.MustParse("12.5"), ClientOrderID: "bw_ema1d_7_open",
	})
	require.NoError(t, err)
	require.True(t, o.Filled())
	require.Equal(t, "0.025", o.AvgPrice().String())
	require.Len(t, o.Fills, 1)
}

func TestBackOffAfter429(t *testing.T) {
	var calls atomic.Int32
	c := newTest(t, func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
	}, true)

	_, err := c.TickerPrices(context.Background())
	require.ErrorIs(t, err, ErrBanned)
	_, err = c.TickerPrices(context.Background())
	require.ErrorIs(t, err, ErrBanned)
	require.Equal(t, int32(1), calls.Load(), "second call must not hit the server while backing off")
	require.True(t, IsTransient(err))
}

func TestKlinesParse(t *testing.T) {
	c := newTest(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "1d", r.URL.Query().Get("interval"))
		_, _ = w.Write([]byte(`[[1700000000000,"1.0","2.0","0.5","1.5","100",1700086399999,"150",10,"50","75","0"]]`))
	}, false)
	ks, err := c.Klines(context.Background(), "SUNUSDT", "1d", time.Time{}, 10)
	require.NoError(t, err)
	require.Len(t, ks, 1)
	require.Equal(t, "1.5", ks[0].Close.String())
	require.Equal(t, int64(1700000000000), ks[0].OpenTime.UnixMilli())
}

func TestExchangeInfoFilters(t *testing.T) {
	c := newTest(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, `["AUSDT","BUSDT"]`, r.URL.Query().Get("symbols"))
		_, _ = w.Write([]byte(`{"symbols":[{"symbol":"AUSDT","status":"TRADING","baseAsset":"A","quoteAsset":"USDT",
			"isMarginTradingAllowed":true,"filters":[{"filterType":"LOT_SIZE","stepSize":"0.10000000","minQty":"0.1"},
			{"filterType":"NOTIONAL","minNotional":"5.00000000"}]}]}`))
	}, false)
	infos, err := c.ExchangeInfo(context.Background(), "AUSDT", "BUSDT")
	require.NoError(t, err)
	require.Equal(t, "0.1", infos[0].StepSize.String())
	require.Equal(t, "5", infos[0].MinNotional.String())
	require.True(t, infos[0].IsMarginTradingAllowed)
}
