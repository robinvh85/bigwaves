package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"bigwaves/internal/app"
	"bigwaves/internal/ema1d"
	"bigwaves/internal/ema1d/engine"
	"bigwaves/internal/platform/db/dbq"
)

const ema1dUsage = `usage: bwctl ema1d <command>

  seed [-added-on YYYY-MM-DD]          create the basket pairs (idempotent)
  check-pairs                          check cross margin / borrowable on Binance
  signals                              print the strategy table from stored candles
  update-candles                       fetch the missing 1D/4H candles from Binance
  import-candles -dir DIR              load <coin>_1D.csv / <coin>_4H.csv (tradingview-ema)
  daily-check [-real] [-send] [-force] run the daily check (dry run by default)
`

func runEma1d(ctx context.Context, a *app.App, args []string) error {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, ema1dUsage)
		return fmt.Errorf("missing ema1d command")
	}
	s := a.EMA1D
	fs := flag.NewFlagSet("ema1d "+args[0], flag.ContinueOnError)
	switch args[0] {
	case "seed":
		addedOn := fs.String("added-on", time.Now().UTC().Format("2006-01-02"), "first candle date whose entries are taken")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		t, err := time.Parse("2006-01-02", *addedOn)
		if err != nil {
			return err
		}
		rep, err := s.Seed(ctx, t)
		printJSON(rep)
		return err
	case "check-pairs":
		res, err := s.CheckPairs(ctx, nil)
		for _, r := range res {
			fmt.Printf("%-8s short_supported=%-5v %s\n", r.Coin, r.ShortSupported, r.Note)
		}
		return err
	case "signals":
		sig, err := s.Signals(ctx)
		if err != nil {
			return err
		}
		for _, r := range sig.Rows {
			vao, px := "", ""
			if r.Vao != nil {
				vao = *r.Vao
			}
			if r.GiaVao != nil {
				px = engine.Repr(*r.GiaVao)
			}
			fmt.Println(strings.Join([]string{r.Pair, r.Ngay, engine.Repr(r.Gia), r.EmaOpen, r.CatHomNay, r.Lenh, vao, px, r.Lai, r.ThoatTheo, r.HanhDong, r.Nhom, r.GhiChu}, "  "))
		}
		fmt.Printf("\ns=%s\n", sig.S)
		if len(sig.Warnings) > 0 {
			fmt.Printf("warnings: %s\n", strings.Join(sig.Warnings, "; "))
		}
		return nil
	case "update-candles":
		rep, err := s.UpdateCandles(ctx)
		printJSON(rep)
		return err
	case "import-candles":
		dir := fs.String("dir", "", "directory of the CSV files")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		return importCandles(ctx, a, *dir)
	case "daily-check":
		real := fs.Bool("real", false, "run for real (sync and store), not a dry run")
		send := fs.Bool("send", false, "send the report even on a dry run")
		force := fs.Bool("force", false, "run although the daily check is switched off")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		res, err := s.DailyCheck(ctx, ema1d.DailyCheckOptions{DryRun: !*real, Deliver: *send || *real, Force: *force})
		if res.Report != "" {
			fmt.Println(res.Report)
		}
		res.Report = ""
		printJSON(res)
		return err
	default:
		fmt.Fprint(os.Stderr, ema1dUsage)
		return fmt.Errorf("unknown ema1d command %q", args[0])
	}
}

func printJSON(v any) {
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(b))
}

// importCandles loads the research CSV files (time,open,high,low,close; no header) of the basket
// pairs. Pairs without a file are left to update-candles, which fetches the whole history.
func importCandles(ctx context.Context, a *app.App, dir string) error {
	if dir == "" {
		return fmt.Errorf("-dir is required")
	}
	pairs, err := a.EMA1D.PairRows(ctx)
	if err != nil {
		return err
	}
	q := dbq.New(a.DB.Write)
	for _, p := range pairs {
		for _, interval := range []string{"1D", "4H"} {
			file := filepath.Join(dir, strings.ToLower(p.Coin)+"_"+interval+".csv")
			f, err := os.Open(file) //nolint:gosec // operator-provided path
			if err != nil {
				fmt.Printf("%s %s: no %s\n", p.Coin, interval, file)
				continue
			}
			n := 0
			seen := map[string]bool{}
			sc := bufio.NewScanner(f)
			for sc.Scan() {
				parts := strings.Split(strings.TrimSpace(sc.Text()), ",")
				if len(parts) < 5 || parts[0] == "" || seen[parts[0]] {
					continue
				}
				seen[parts[0]] = true
				ts, err := strconv.ParseInt(parts[0], 10, 64)
				if err != nil {
					_ = f.Close()
					return fmt.Errorf("%s: %w", file, err)
				}
				var px [4]float64
				for i := range px {
					if px[i], err = strconv.ParseFloat(parts[i+1], 64); err != nil {
						_ = f.Close()
						return fmt.Errorf("%s: %w", file, err)
					}
				}
				if err := q.UpsertCandle(ctx, dbq.UpsertCandleParams{PairID: p.PairID, Interval: interval, OpenTime: ts, Open: px[0], High: px[1], Low: px[2], Close: px[3]}); err != nil {
					_ = f.Close()
					return err
				}
				n++
			}
			_ = f.Close()
			fmt.Printf("%s %s: %d candles\n", p.Coin, interval, n)
		}
	}
	return nil
}
