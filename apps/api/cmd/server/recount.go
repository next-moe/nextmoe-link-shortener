package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/next-moe/nextmoe-link-shortener/apps/api/internal/config"
	"github.com/next-moe/nextmoe-link-shortener/apps/api/internal/db"
	"github.com/next-moe/nextmoe-link-shortener/apps/api/internal/engine"
)

// recountBots rebuilds the settlement tables from the raw visits with the
// current bot rule: `/app recount-bots [-from YYYY-MM-DD] [-to YYYY-MM-DD]`.
// It runs in the API container (`docker exec <api> /app recount-bots`), whose
// image has no shell, and exits with the process status.
func recountBots(cfg config.Config, args []string) int {
	fs := flag.NewFlagSet("recount-bots", flag.ContinueOnError)
	rawFrom := fs.String("from", "", "first JST day to rebuild (default: the first settlement day on record)")
	rawTo := fs.String("to", "", "last JST day to rebuild (default: yesterday, JST)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	var from time.Time
	if *rawFrom != "" {
		d, err := engine.ParseDate(*rawFrom)
		if err != nil {
			fmt.Fprintln(os.Stderr, "-from must be YYYY-MM-DD")
			return 2
		}
		from = d
	}
	to := engine.Yesterday()
	if *rawTo != "" {
		d, err := engine.ParseDate(*rawTo)
		if err != nil {
			fmt.Fprintln(os.Stderr, "-to must be YYYY-MM-DD")
			return 2
		}
		to = d
	}

	database, err := db.Open(cfg.DBDSN)
	if err != nil {
		slog.Error("cannot connect to postgres", "error", err)
		return 1
	}
	defer func() { _ = database.Close() }()

	res, err := engine.New(database.Gorm).RecountSettlement(from, to)
	if err != nil {
		slog.Error("recount failed", "error", err)
		return 1
	}
	slog.Info("recount done", "from", res.From, "to", res.To, "visits", res.Visits,
		"bot_visits", res.BotVisits, "link_days", res.LinkDays)
	return 0
}
