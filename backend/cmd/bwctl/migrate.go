package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"time"

	"bigwaves/internal/app"
	"bigwaves/internal/migrate"
	"bigwaves/internal/platform/db/dbq"
	"bigwaves/internal/platform/dbtype"
	"bigwaves/internal/platform/notify"
)

func runImport(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("import-pg", flag.ContinueOnError)
	pg := fs.String("pg", os.Getenv("PG_URL"), "PostgreSQL URL of the Rails database (or PG_URL)")
	out := fs.String("out", "", "new SQLite file to create")
	archive := fs.String("archive", "archive", "directory for the CSV of dropped tables (empty: none)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *pg == "" || *out == "" {
		return fmt.Errorf("-pg and -out are required")
	}
	start := time.Now()
	rep, err := migrate.ImportPG(ctx, *pg, *out, *archive)
	printJSON(rep)
	if err != nil {
		return err
	}
	fmt.Printf("imported in %s; now run: bwctl verify-import -pg ... -db %s\n", time.Since(start).Round(time.Second), *out)
	return nil
}

func runVerify(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("verify-import", flag.ContinueOnError)
	pg := fs.String("pg", os.Getenv("PG_URL"), "PostgreSQL URL of the Rails database (or PG_URL)")
	file := fs.String("db", "", "SQLite file produced by import-pg")
	if err := fs.Parse(args); err != nil {
		return err
	}
	checks, err := migrate.Verify(ctx, *pg, *file)
	if err != nil {
		return err
	}
	failed := 0
	for _, c := range checks {
		mark := "OK  "
		if !c.OK {
			mark = "DIFF"
			failed++
		}
		fmt.Printf("%s %-36s pg=%s sqlite=%s\n", mark, c.Name, c.Postgres, c.SQLite)
	}
	if failed > 0 {
		return fmt.Errorf("%d check(s) differ: do not cut over", failed)
	}
	fmt.Println("all checks OK")
	return nil
}

func runShadow(ctx context.Context, a *app.App, args []string) error {
	fs := flag.NewFlagSet("shadow-compare", flag.ContinueOnError)
	pg := fs.String("pg", os.Getenv("PG_URL"), "PostgreSQL URL of the Rails database (or PG_URL)")
	date := fs.String("date", time.Now().UTC().Format("2006-01-02"), "candle date (UTC)")
	send := fs.Bool("notify", false, "send the result to the Discord alerts channel")
	if err := fs.Parse(args); err != nil {
		return err
	}
	res, err := migrate.Shadow(ctx, *pg, a.EMA1D, *date)
	if err != nil {
		return err
	}
	fmt.Print(res.Text())
	if *send {
		if err := a.Notifier.Send(ctx, notify.Alerts, res.Text()); err != nil {
			return err
		}
	}
	if !res.OK() {
		return fmt.Errorf("shadow run differs from Rails on %s", *date)
	}
	return nil
}

func runExportOrders(ctx context.Context, a *app.App, args []string) error {
	fs := flag.NewFlagSet("export-orders", flag.ContinueOnError)
	since := fs.String("since", "", "RFC3339 time, e.g. 2026-11-01T00:00:00Z")
	if err := fs.Parse(args); err != nil {
		return err
	}
	t, err := time.Parse(time.RFC3339, *since)
	if err != nil {
		return fmt.Errorf("-since: %w", err)
	}
	rows, err := dbq.New(a.DB.Read).ListExchangeOrdersSince(ctx, dbtype.NewTime(t))
	if err != nil {
		return err
	}
	w := csv.NewWriter(os.Stdout)
	_ = w.Write([]string{"client_order_id", "strategy", "ref_id", "symbol", "account", "side", "status", "executed_qty", "quote_qty", "avg_price", "net_qty", "fee_usdt", "executed_at"})
	for _, r := range rows {
		ref := ""
		if r.RefID != nil {
			ref = fmt.Sprint(*r.RefID)
		}
		_ = w.Write([]string{r.ClientOrderID, r.Strategy, ref, r.Symbol, r.Account, r.Side, r.Status, r.ExecutedQty.String(),
			r.QuoteQty.String(), r.AvgPrice.String(), r.NetQty.String(), r.FeeUsdt.String(), r.ExecutedAt.String()})
	}
	w.Flush()
	return w.Error()
}
