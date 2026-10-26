package notify

import (
	"context"
	"crypto/sha1" //nolint:gosec // fingerprint, not security
	"encoding/hex"
	"fmt"
	"log/slog"
	"regexp"
	"time"

	"github.com/getsentry/sentry-go"

	"bigwaves/internal/platform/db"
	"bigwaves/internal/platform/db/dbq"
	"bigwaves/internal/platform/dbtype"
)

// Alerter reports errors to logs, Sentry and the Discord alerts channel. The same error from the
// same source is sent to Discord at most once per Window; every occurrence is still counted.
// Unlike the Rails version, the fingerprint includes the (normalized) message, so two different
// errors of the same type no longer hide each other.
type Alerter struct {
	DB       *db.DB
	Notifier Notifier
	Window   time.Duration
	Now      func() time.Time
}

func NewAlerter(d *db.DB, n Notifier) *Alerter {
	return &Alerter{DB: d, Notifier: n, Window: 10 * time.Minute, Now: time.Now}
}

var volatile = regexp.MustCompile(`\d+(\.\d+)?|0x[0-9a-f]+`)

func fingerprint(source string, err error) string {
	norm := volatile.ReplaceAllString(err.Error(), "#")
	sum := sha1.Sum([]byte(source + "|" + fmt.Sprintf("%T", err) + "|" + norm)) //nolint:gosec
	return hex.EncodeToString(sum[:])
}

// Report records err from source and notifies when the throttle window allows it.
func (a *Alerter) Report(ctx context.Context, source string, err error) {
	if err == nil {
		return
	}
	slog.ErrorContext(ctx, "error", "source", source, "err", err)
	now := a.Now()
	fp := fingerprint(source, err)
	q := dbq.New(a.DB.Write)
	ev, dbErr := q.UpsertErrorEvent(ctx, dbq.UpsertErrorEventParams{
		Fingerprint: fp, Source: source, Message: err.Error(),
		FirstSeenAt: dbtype.NewTime(now), LastSeenAt: dbtype.NewTime(now),
	})
	if dbErr != nil {
		slog.ErrorContext(ctx, "record error event", "err", dbErr)
		return
	}
	if ev.NotifiedAt != nil && now.Sub(ev.NotifiedAt.Time) < a.Window {
		return
	}
	sentry.WithScope(func(s *sentry.Scope) {
		s.SetTag("source", source)
		sentry.CaptureException(err)
	})
	msg := fmt.Sprintf("⚠️ **%s** lỗi: %s", source, err.Error())
	if ev.Count > 1 {
		msg += fmt.Sprintf(" (lần thứ %d)", ev.Count)
	}
	if sendErr := a.Notifier.Send(ctx, Alerts, msg); sendErr != nil {
		slog.ErrorContext(ctx, "send alert", "err", sendErr)
		return
	}
	if err := q.MarkErrorNotified(ctx, dbq.MarkErrorNotifiedParams{NotifiedAt: ptr(dbtype.NewTime(now)), Fingerprint: fp}); err != nil {
		slog.ErrorContext(ctx, "mark error notified", "err", err)
	}
}

func ptr[T any](v T) *T { return &v }
