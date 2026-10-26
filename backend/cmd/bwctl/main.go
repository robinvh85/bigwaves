// Command bwctl is the operator CLI: migrations, users, one-off jobs and imports.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"syscall"

	"bigwaves/internal/app"
	"bigwaves/internal/config"
	"bigwaves/internal/platform/logging"
)

const usage = `usage: bwctl <command> [args]

commands:
  migrate              apply database migrations
  user add <email>     allow an email to sign in
  job list             list scheduled jobs
  job run <name>       run a job now and wait for it
  ema1d ...            EMA1D tools (bwctl ema1d for the list)

migration (see .claude/plans/03-migration-cutover.md):
  import-pg -pg URL -out FILE [-archive DIR]   copy the Rails PostgreSQL data into a NEW SQLite file
  verify-import -pg URL -db FILE               compare counts, totals and open positions
  shadow-compare -pg URL [-date D] [-notify]   compare today's Rails sync with the bot (dry run)
  export-orders -since RFC3339                 orders the bot placed since a time, as CSV (rollback)
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1], os.Args[2:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cmd string, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logging.Setup(cfg)

	// These work on files given as flags, not on DB_PATH.
	switch cmd {
	case "import-pg":
		return runImport(ctx, args)
	case "verify-import":
		return runVerify(ctx, args)
	}

	a, err := app.New(ctx, cfg) // also applies migrations
	if err != nil {
		return err
	}
	defer func() { _ = a.Close() }()

	switch {
	case cmd == "migrate":
		fmt.Println("migrations applied")
		return nil
	case cmd == "user" && len(args) == 2 && args[0] == "add":
		u, err := a.Auth.AddUser(ctx, args[1])
		if err != nil {
			return err
		}
		fmt.Printf("user %d %s can sign in\n", u.ID, u.Email)
		return nil
	case cmd == "job" && len(args) == 1 && args[0] == "list":
		jobs := a.Scheduler.Jobs()
		sort.Slice(jobs, func(i, j int) bool { return jobs[i].Name < jobs[j].Name })
		for _, j := range jobs {
			fmt.Printf("%-32s %-40s %s\n", j.Name, j.Cron, j.Description)
		}
		return nil
	case cmd == "job" && len(args) == 2 && args[0] == "run":
		return a.Scheduler.RunSync(args[1])
	case cmd == "ema1d":
		return runEma1d(ctx, a, args)
	case cmd == "shadow-compare":
		return runShadow(ctx, a, args)
	case cmd == "export-orders":
		return runExportOrders(ctx, a, args)
	default:
		fmt.Fprint(os.Stderr, usage)
		return fmt.Errorf("unknown command: %s %v", cmd, args)
	}
}
