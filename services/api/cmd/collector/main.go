// Command collector is the command side: it discovers and polls kokkai resources
// over a read-only, rate-limited HTTP boundary, appends observation events, and
// projects the interpretation read models. Subcommands:
//
//	collector run                          daemon: poll due watches + project, on a loop
//	collector poll-once                    one poll+project cycle, then exit
//	collector reproject                    truncate read models and replay from seq 0
//	collector discover --from --until      seed the watch list from meeting_list
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"s4rciv.org/api/internal/driver/kokkaihttp"
	"s4rciv.org/api/internal/driver/postgres"
	"s4rciv.org/api/internal/driver/sys"
	"s4rciv.org/api/internal/gateway/kokkai"
	"s4rciv.org/api/internal/port"
	"s4rciv.org/api/internal/usecase/collect"
	"s4rciv.org/api/internal/usecase/project"
)

const (
	source           = "kokkai"
	collectorVersion = "S4rCiv-collect/0.1.0"
	daemonInterval   = 60 * time.Second
	pollBatch        = 100
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app, err := wire(ctx)
	if err != nil {
		log.Fatalf("startup: %v", err)
	}
	defer app.pool.Close()

	switch os.Args[1] {
	case "run":
		runDaemon(ctx, app)
	case "poll-once":
		cycle(ctx, app)
	case "reproject":
		n, err := app.projector.Reproject(ctx)
		if err != nil {
			log.Fatalf("reproject: %v", err)
		}
		log.Printf("reproject: projected %d meetings", n)
	case "discover":
		runDiscover(ctx, app)
	default:
		usage()
		os.Exit(2)
	}
}

type app struct {
	pool      *pgxpool.Pool
	collector *collect.Collector
	projector *project.Projector
}

func wire(ctx context.Context) (*app, error) {
	pool, err := postgres.Connect(ctx)
	if err != nil {
		return nil, err
	}
	control := postgres.NewControlStore(pool)
	cfg, err := control.Source(ctx, source)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("load source %q: %w", source, err)
	}
	ua := envOr("USER_AGENT", cfg.UserAgent)
	httpc, err := kokkaihttp.New(cfg.BaseURL, ua, cfg.RateLimit)
	if err != nil {
		pool.Close()
		return nil, err
	}
	gw := kokkai.New(httpc)
	collector := collect.New(
		postgres.NewEventLog(pool), gw, control, gw,
		sys.Clock{}, sys.IDGen{},
		collect.Config{FetcherVersion: collectorVersion},
	)
	rm := postgres.NewReadModel(pool)
	projector := project.New(postgres.NewEventReader(pool), gw, rm, rm, source)
	return &app{pool: pool, collector: collector, projector: projector}, nil
}

func runDaemon(ctx context.Context, a *app) {
	log.Printf("collector daemon started (source=%s, interval=%s)", source, daemonInterval)
	ticker := time.NewTicker(daemonInterval)
	defer ticker.Stop()
	for {
		cycle(ctx, a)
		select {
		case <-ctx.Done():
			log.Print("collector daemon stopping")
			return
		case <-ticker.C:
		}
	}
}

func cycle(ctx context.Context, a *app) {
	emitted, err := a.collector.PollOnce(ctx, source, pollBatch)
	if err != nil {
		log.Printf("poll: %v", err)
	} else {
		log.Printf("poll: emitted %d events", emitted)
	}
	projected, err := a.projector.Run(ctx)
	if err != nil {
		log.Printf("project: %v", err)
		return
	}
	log.Printf("project: %d meetings", projected)
}

func runDiscover(ctx context.Context, a *app) {
	fs := flag.NewFlagSet("discover", flag.ExitOnError)
	from := fs.String("from", "", "start date YYYY-MM-DD (inclusive)")
	until := fs.String("until", "", "end date YYYY-MM-DD (inclusive)")
	max := fs.Int("max", 0, "cap on discovered resources (0 = no cap)")
	_ = fs.Parse(os.Args[2:])
	if *from == "" || *until == "" {
		log.Fatal("discover requires --from and --until (YYYY-MM-DD)")
	}
	n, err := a.collector.Discover(ctx, port.ListScope{From: *from, Until: *until, Max: *max})
	if err != nil {
		log.Fatalf("discover: %v", err)
	}
	log.Printf("discover: upserted %d watches", n)
}

func usage() {
	fmt.Fprint(os.Stderr, `usage: collector <run|poll-once|reproject|discover>

  run         daemon: poll due watches + project on a loop
  poll-once   one poll+project cycle, then exit
  reproject   truncate read models and replay from seq 0
  discover --from YYYY-MM-DD --until YYYY-MM-DD [--max N]
`)
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
