// Command bwctl is the operator CLI: migrations, imports and one-off maintenance tasks.
package main

import (
	"context"
	"fmt"
	"os"

	"bigwaves/internal/config"
	"bigwaves/internal/platform/db"
	"bigwaves/internal/platform/logging"
)

const usage = `usage: bwctl <command>

commands:
  migrate    apply database migrations
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	if err := run(context.Background(), os.Args[1], os.Args[2:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cmd string, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logging.Setup(cfg)

	switch cmd {
	case "migrate":
		d, err := db.Open(ctx, cfg.DBPath)
		if err != nil {
			return err
		}
		defer func() { _ = d.Close() }()
		return d.Migrate(ctx)
	default:
		fmt.Fprint(os.Stderr, usage)
		return fmt.Errorf("unknown command %q", cmd)
	}
}
