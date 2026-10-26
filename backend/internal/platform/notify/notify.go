// Package notify sends messages to Discord webhooks and throttles repeated error alerts.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// Channel picks the webhook.
type Channel string

const (
	Orders  Channel = "orders"  // orders placed, closed, failed
	Alerts  Channel = "alerts"  // job failures, margin level, pair problems
	Reports Channel = "reports" // daily reports
)

// Notifier sends a message to a channel. Implementations must be safe for concurrent use.
type Notifier interface {
	Send(ctx context.Context, ch Channel, msg string) error
}

// discordLimit is below Discord's 2000-character content limit to leave room for a part counter.
const discordLimit = 1900

type Discord struct {
	webhooks map[Channel]string
	http     *http.Client
}

func NewDiscord(order, alert, report string) *Discord {
	return &Discord{
		webhooks: map[Channel]string{Orders: order, Alerts: alert, Reports: report},
		http:     &http.Client{Timeout: 15 * time.Second},
	}
}

// Send posts msg, split into several messages when it is too long. Without a webhook for the
// channel the message is only logged.
func (d *Discord) Send(ctx context.Context, ch Channel, msg string) error {
	url := d.webhooks[ch]
	if url == "" {
		slog.InfoContext(ctx, "discord (no webhook)", "channel", ch, "msg", msg)
		return nil
	}
	for _, part := range Split(msg, discordLimit) {
		if err := d.post(ctx, url, part); err != nil {
			return err
		}
	}
	return nil
}

func (d *Discord) post(ctx context.Context, url, content string) error {
	body, _ := json.Marshal(map[string]string{"content": content})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := d.http.Do(req)
	if err != nil {
		return fmt.Errorf("discord: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("discord: http %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

// Split cuts s into chunks of at most limit runes, preferring line breaks.
func Split(s string, limit int) []string {
	var parts []string
	for len([]rune(s)) > limit {
		r := []rune(s)
		cut := limit
		if i := strings.LastIndex(string(r[:limit]), "\n"); i > 0 {
			cut = len([]rune(string(r[:limit])[:i]))
		}
		parts = append(parts, strings.TrimRight(string(r[:cut]), "\n"))
		s = strings.TrimLeft(string(r[cut:]), "\n")
	}
	if s != "" {
		parts = append(parts, s)
	}
	return parts
}

// Recorder keeps sent messages in memory; for tests.
type Recorder struct {
	Messages []Sent
}

type Sent struct {
	Channel Channel
	Msg     string
}

func (r *Recorder) Send(_ context.Context, ch Channel, msg string) error {
	r.Messages = append(r.Messages, Sent{ch, msg})
	return nil
}
