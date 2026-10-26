// Package binance is a small client for the Binance spot and cross-margin REST endpoints the
// strategies use. It signs requests, respects the request-weight limits, and refuses every
// write call unless trading is enabled. It never retries a write: retries belong to the caller
// (the todo queue), which reuses orders by clientOrderId.
package binance

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Config struct {
	BaseURL        string
	APIKey         string
	SecretKey      string
	TradingEnabled bool
	// RecvWindow in milliseconds (default 10000).
	RecvWindow int
}

type Client struct {
	cfg  Config
	http *http.Client
	now  func() time.Time

	mu         sync.Mutex
	timeOffset time.Duration // server time minus local time
	usedWeight int           // last X-MBX-USED-WEIGHT-1M (spot) seen
	weightAt   time.Time
	bannedTill time.Time
}

// weightLimit is the per-minute IP weight Binance allows for /api; we slow down well before it.
const (
	weightLimit    = 6000
	weightSlowDown = 4800
)

func New(cfg Config) *Client {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api-gcp.binance.com"
	}
	if cfg.RecvWindow == 0 {
		cfg.RecvWindow = 10000
	}
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: 20 * time.Second},
		now:  time.Now,
	}
}

// TradingEnabled reports whether write calls are allowed.
func (c *Client) TradingEnabled() bool { return c.cfg.TradingEnabled }

type security int

const (
	public security = iota
	signed
)

type request struct {
	method   string
	path     string
	params   url.Values
	security security
	write    bool // places an order, borrows, repays or moves funds
}

func (c *Client) do(ctx context.Context, r request, out any) error {
	if r.write && !c.cfg.TradingEnabled {
		slog.WarnContext(ctx, "binance write blocked", "req", describe(r))
		return ErrTradingDisabled
	}
	if err := c.waitForWeight(ctx); err != nil {
		return err
	}

	params := url.Values{}
	for k, v := range r.params {
		params[k] = v
	}
	if r.security == signed {
		if c.cfg.APIKey == "" || c.cfg.SecretKey == "" {
			return fmt.Errorf("%w (%s)", ErrNotConfigured, r.path)
		}
		params.Set("timestamp", strconv.FormatInt(c.serverNow().UnixMilli(), 10))
		params.Set("recvWindow", strconv.Itoa(c.cfg.RecvWindow))
	}
	query := encodeSorted(params)
	if r.security == signed {
		query += "&signature=" + sign(c.cfg.SecretKey, query)
	}

	u := c.cfg.BaseURL + r.path
	if query != "" {
		u += "?" + query
	}
	req, err := http.NewRequestWithContext(ctx, r.method, u, nil)
	if err != nil {
		return err
	}
	if r.security == signed {
		req.Header.Set("X-MBX-APIKEY", c.cfg.APIKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("binance %s %s: %w", r.method, r.path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	c.recordWeight(resp)

	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return fmt.Errorf("binance %s %s: read body: %w", r.method, r.path, err)
	}

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusTeapot {
		c.backOff(resp)
		slog.ErrorContext(ctx, "binance rate limited", "status", resp.StatusCode, "retry_after", resp.Header.Get("Retry-After"))
		return fmt.Errorf("%w (http %d)", ErrBanned, resp.StatusCode)
	}
	if resp.StatusCode >= 400 {
		apiErr := &APIError{HTTPStatus: resp.StatusCode, Request: describe(r)}
		var payload struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
		}
		if json.Unmarshal(body, &payload) == nil && payload.Code != 0 {
			apiErr.Code, apiErr.Msg = payload.Code, payload.Msg
		} else {
			apiErr.Msg = truncate(string(body), 200)
		}
		if apiErr.Code == CodeInvalidTimestamp {
			// Clock drift: resync so the caller's retry succeeds.
			_ = c.SyncTime(ctx)
		}
		return apiErr
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("binance %s %s: decode: %w", r.method, r.path, err)
	}
	return nil
}

// SyncTime measures the offset between Binance's clock and ours.
func (c *Client) SyncTime(ctx context.Context) error {
	var st struct {
		ServerTime int64 `json:"serverTime"`
	}
	before := c.now()
	if err := c.do(ctx, request{method: http.MethodGet, path: "/api/v3/time"}, &st); err != nil {
		return err
	}
	after := c.now()
	local := before.Add(after.Sub(before) / 2)
	c.mu.Lock()
	c.timeOffset = time.UnixMilli(st.ServerTime).Sub(local)
	c.mu.Unlock()
	return nil
}

func (c *Client) serverNow() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now().Add(c.timeOffset)
}

func (c *Client) recordWeight(resp *http.Response) {
	h := resp.Header.Get("X-Mbx-Used-Weight-1m")
	if h == "" {
		return
	}
	w, err := strconv.Atoi(h)
	if err != nil {
		return
	}
	c.mu.Lock()
	c.usedWeight, c.weightAt = w, c.now()
	c.mu.Unlock()
}

func (c *Client) backOff(resp *http.Response) {
	wait := time.Minute
	if s, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && s > 0 {
		wait = time.Duration(s) * time.Second
	}
	c.mu.Lock()
	c.bannedTill = c.now().Add(wait)
	c.mu.Unlock()
}

// waitForWeight blocks while banned or when the minute's weight is close to the limit.
func (c *Client) waitForWeight(ctx context.Context) error {
	c.mu.Lock()
	now := c.now()
	if now.Before(c.bannedTill) {
		c.mu.Unlock()
		return ErrBanned
	}
	var wait time.Duration
	if c.usedWeight >= weightSlowDown && c.weightAt.Truncate(time.Minute).Equal(now.Truncate(time.Minute)) {
		wait = now.Truncate(time.Minute).Add(time.Minute).Sub(now)
	}
	c.mu.Unlock()
	if wait <= 0 {
		return nil
	}
	slog.WarnContext(ctx, "binance weight near limit, pausing", "wait", wait)
	t := time.NewTimer(wait)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func sign(secret, payload string) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(payload))
	return hex.EncodeToString(m.Sum(nil))
}

// encodeSorted is url.Values.Encode; spelled out so the signed string is obviously the sent one.
func encodeSorted(v url.Values) string {
	keys := make([]string, 0, len(v))
	for k := range v {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		for _, val := range v[k] {
			if b.Len() > 0 {
				b.WriteByte('&')
			}
			b.WriteString(url.QueryEscape(k))
			b.WriteByte('=')
			b.WriteString(url.QueryEscape(val))
		}
	}
	return b.String()
}

func describe(r request) string {
	shown := encodeSorted(r.params)
	return strings.TrimSpace(r.method + " " + r.path + " " + shown)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
