package httpapi

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"bigwaves/internal/ema1d"
	"bigwaves/internal/ema1d/engine"
	"bigwaves/internal/emacross"
	"bigwaves/internal/platform/money"
)

func d(s string) *money.Dec { v := money.MustParse(s); return &v }
func sp(s string) *string   { return &s }

// roundTrip fails when a field of the domain value has no counterpart in the API type (it would
// be dropped silently by convert).
func roundTrip[T any](t *testing.T, domain any) {
	t.Helper()
	want, err := json.Marshal(domain)
	require.NoError(t, err)
	api, err := convert[T](domain)
	require.NoError(t, err)
	got, err := json.Marshal(api)
	require.NoError(t, err)
	require.JSONEq(t, string(want), string(got))
}

func TestDomainTypesMatchTheAPI(t *testing.T) {
	now := time.Date(2026, 10, 10, 2, 0, 0, 0, time.UTC)
	days := 3
	pos := ema1d.PositionRow{
		ID: 1, Coin: "SUN", PairName: "SUNUSDT", Side: "SHORT", Status: "OPEN", SignalDate: "2026-10-05", ExitRule: sp("XO"),
		Amount: *d("10500"), OpenPrice: d("0.02"), RefOpenPrice: d("0.0213"), OpenSlippagePercent: d("6.1"),
		ClosePrice: d("0.018"), RefClosePrice: d("0.0181"), CloseSlippagePercent: d("-0.5"), CurrentPrice: d("0.018"),
		NotionalUsdt: *d("210"), SizePercent: *d("3.5"), EquityPercent: d("3"), ProfitUsdt: *d("20"), ProfitPercent: d("9.5"),
		FeeUsdt: *d("0.2"), InterestUsdt: *d("0.1"), OpenAt: &now, CloseAt: &now, DaysHeld: &days,
	}
	step, dep, posID := "BORROWED", int64(2), int64(1)
	todo := ema1d.TodoRow{
		ID: 3, Coin: "SUN", Action: "OPEN", Side: "SHORT", SignalDate: "2026-10-10", Status: "PENDING", Source: "SYNC",
		RefPrice: d("0.02"), ExitRule: sp("XO"), Attempts: 1, RunAfter: &now, LastError: "x", Step: &step,
		DependsOnID: &dep, PositionID: &posID, Cancellable: true, Retryable: false, CreatedAt: now, UpdatedAt: now,
	}
	st := ema1d.Status{
		GeneratedAt: now, Enabled: true, PauseOpen: true, DailyCheckEnabled: true, TradingEnabled: true,
		Capital:       ema1d.Capital{BudgetUsdt: *d("6000"), RealizedProfitUsdt: *d("1"), EquityUsdt: *d("6001"), UnrealizedProfitUsdt: *d("2"), InterestPaidUsdt: *d("3"), FeePaidUsdt: *d("4"), ClosedPositions: 5},
		Exposure:      ema1d.Exposure{LongUsdt: *d("1"), ShortUsdt: *d("2"), LongPercent: d("3"), ShortPercent: d("4"), LongCount: 1, ShortCount: 2},
		MarginLevel:   ema1d.MarginStatus{Value: d("3.2"), At: &now, Warn: *d("1.5"), Alert: *d("1.3")},
		Sizes:         []ema1d.SizeRow{{Key: "coreLong", Percent: *d("7"), SuggestedPercent: *d("7")}},
		OpenPositions: []ema1d.PositionRow{pos}, Todos: []ema1d.TodoRow{todo},
		LastSync:         &ema1d.LastSync{RunID: "r", Date: "2026-10-10", At: now, CreatedTodos: 1, Skipped: []ema1d.SkippedEntry{{Coin: "SUN", Action: "OPEN", Side: "LONG", Reason: "x"}}, Warnings: []string{"w"}},
		SyncMissingToday: true,
	}
	roundTrip[Ema1dStatus](t, st)

	pair := ema1d.PairRow{
		ID: 1, PairID: 2, Coin: "SUN", PairName: "SUNUSDT", PairGroup: "CORE", Status: "ACTIVE", AllowLong: true, AllowShort: true,
		ShortSupported: true, LongExitRule: sp("DIV4H"), AddedOn: "2026-10-01", RemovedOn: sp("2026-10-20"), SignalFrom: sp("2026-10-02"),
		CheckNote: "ok", CheckedAt: &now, CurrentPrice: d("0.02"),
		Position:     &ema1d.PairPosition{PositionID: 1, Side: "SHORT", Status: "OPEN", SignalDate: "2026-10-05", UnrealizedProfitUsdt: *d("1"), UnrealizedPercent: d("2")},
		Wanted:       &ema1d.Wanted{State: "S", EntryDate: sp("2026-10-05"), Rule: sp("XO"), SyncDate: "2026-10-10"},
		PendingTodos: []string{"CLOSE SHORT"}, Mismatch: true,
	}
	roundTrip[Ema1dPairRow](t, pair)

	roundTrip[Ema1dTradesSummary](t, ema1d.TradesSummary{Count: 2, TotalProfitUsdt: *d("1"), WinRatePercent: d("50"), AvgWinUsdt: d("2"), AvgLossUsdt: d("-1"), InterestUsdt: *d("0.1"), FeeUsdt: *d("0.2")})
	roundTrip[Ema1dConfig](t, ema1d.DefaultConfig())
	roundTrip[Ema1dEquityPoint](t, ema1d.EquityPoint{Date: "2026-10-10", EquityUsdt: *d("6000"), MarginLevel: d("3")})
	roundTrip[Ema1dDailyCheck](t, ema1d.DailyCheckRow{SignalDate: "2026-10-10", Status: "SENT", RunID: "auto", Attempts: 1, Report: "r", LastError: "", SyncedAt: &now, SentAt: &now})
	roundTrip[Ema1dPairCheck](t, ema1d.PairCheck{Coin: "SUN", ShortSupported: true, CrossMargin: true, Note: "n"})
	roundTrip[Ema1dManualTodoResult](t, ema1d.ManualTodoResult{Created: true, Todo: todo})

	px := 2474.88
	roundTrip[Ema1dSignals](t, ema1d.SignalResult{Date: "2026-10-24", S: "ETH:S:...", Warnings: []string{}, Rows: []engine.Row{{
		Pair: "ETH", Ngay: "2026-10-24", Gia: 1.5, EmaOpen: "GIẢM", CatHomNay: "-", Lenh: "SHORT", Vao: sp("2026-10-09"), GiaVao: &px,
		Lai: "+3.7%", ThoatTheo: "XO", HanhDong: "-", Nhom: "RỔ", GhiChu: "", Open: true, Side: "S",
	}}})

	alert := emacross.Alert{
		ID: 1, PairID: 2, PairName: "SUNUSDT", Timeframe: "H4", Direction: "UP", Price: *d("0.02"), GapPercent: 0.35,
		CandleTime: now, SentAt: &now, Error: "", CreatedAt: now,
	}
	roundTrip[EmaCrossPair](t, emacross.Pair{
		PairID: 2, PairName: "SUNUSDT", PairStatus: "ACTIVE", CurrentPrice: d("0.02"), H1: true, H4: true, D1: false,
		States:     []emacross.State{{Timeframe: "H4", Trend: "UP", GapPercent: 0.35, Price: *d("0.02"), CandleTime: now, CheckedAt: now}},
		LastAlerts: []emacross.Alert{alert},
	})
	roundTrip[EmaCrossConfig](t, emacross.DefaultConfig())
}
