// Command collector is the command side: it discovers and polls public-source
// resources over a read-only, rate-limited HTTP boundary, appends observation
// events, and projects the interpretation read models. The source is selected by
// --source (kokkai | egov-law; default kokkai). Subcommands:
//
//	collector [--source S] run                     daemon: poll due watches + project, on a loop
//	collector [--source S] poll-once               one poll+project cycle, then exit
//	collector [--source S] reproject               truncate read models and replay from seq 0
//	collector [--source S] discover --from --until seed the watch list from the source listing
//
// For egov-law, discover also accepts --law-type and --updated (use 更新法令一覧),
// and a cycle additionally runs the differ usecase (interpretation.change).
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

	"s4rciv.org/api/internal/driver/diffrpc"
	"s4rciv.org/api/internal/driver/egovhttp"
	"s4rciv.org/api/internal/driver/kokkaihttp"
	"s4rciv.org/api/internal/driver/postgres"
	"s4rciv.org/api/internal/driver/sys"
	"s4rciv.org/api/internal/gateway/egov"
	"s4rciv.org/api/internal/gateway/kokkai"
	"s4rciv.org/api/internal/port"
	"s4rciv.org/api/internal/usecase/collect"
	"s4rciv.org/api/internal/usecase/diff"
	"s4rciv.org/api/internal/usecase/project"
)

const (
	collectorVersion = "S4rCiv-collect/0.1.0"
	daemonInterval   = 60 * time.Second
	pollBatch        = 100
)

// pipeline abstracts the per-source command side (poll + project [+ diff]).
type pipeline interface {
	source() string
	cycle(ctx context.Context)
	reproject(ctx context.Context) error
	discover(ctx context.Context, args []string)
}

func main() {
	fs := flag.NewFlagSet("collector", flag.ExitOnError)
	source := fs.String("source", "kokkai", "source to operate on (kokkai | egov-law)")
	_ = fs.Parse(os.Args[1:])
	rest := fs.Args()
	if len(rest) < 1 {
		usage()
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.Connect(ctx)
	if err != nil {
		log.Fatalf("startup: %v", err)
	}
	defer pool.Close()

	p, err := wire(ctx, pool, *source)
	if err != nil {
		log.Fatalf("wire %s: %v", *source, err)
	}

	switch rest[0] {
	case "run":
		runDaemon(ctx, p)
	case "poll-once":
		p.cycle(ctx)
	case "reproject":
		if err := p.reproject(ctx); err != nil {
			log.Fatalf("reproject: %v", err)
		}
	case "discover":
		p.discover(ctx, rest[1:])
	default:
		usage()
		os.Exit(2)
	}
}

func wire(ctx context.Context, pool *pgxpool.Pool, source string) (pipeline, error) {
	control := postgres.NewControlStore(pool)
	cfg, err := control.Source(ctx, source)
	if err != nil {
		return nil, fmt.Errorf("load source %q: %w", source, err)
	}
	ua := envOr("USER_AGENT", cfg.UserAgent)

	switch source {
	case kokkai.SourceName:
		httpc, err := kokkaihttp.New(cfg.BaseURL, ua, cfg.RateLimit)
		if err != nil {
			return nil, err
		}
		gw := kokkai.New(httpc)
		collector := collect.New(
			postgres.NewEventLog(pool), gw, control, gw, sys.Clock{}, sys.IDGen{},
			collect.Config{FetcherVersion: collectorVersion},
		)
		rm := postgres.NewReadModel(pool)
		projector := project.New(postgres.NewEventReader(pool), gw, rm, rm, source)
		return &kokkaiPipeline{collector: collector, projector: projector}, nil

	case egov.SourceName:
		httpc, err := egovhttp.New(cfg.BaseURL, ua, cfg.RateLimit)
		if err != nil {
			return nil, err
		}
		gw := egov.New(httpc)
		collector := collect.NewEgov(
			postgres.NewEventLog(pool), gw, control, gw, sys.Clock{}, sys.IDGen{},
			collect.Config{FetcherVersion: collectorVersion},
		)
		reader := postgres.NewEventReader(pool)
		lawRM := postgres.NewLawReadModel(pool)
		projector := project.NewLaw(reader, gw, lawRM, lawRM, source)
		changeRM := postgres.NewChangeReadModel(pool)
		differ := diff.New(reader, diffrpc.New(envOr("DIFFER_URL", "http://differ:9090")), changeRM, changeRM, "egov-differ")
		return &egovPipeline{collector: collector, projector: projector, differ: differ}, nil

	default:
		return nil, fmt.Errorf("unknown source %q", source)
	}
}

// ── kokkai pipeline ─────────────────────────────────────────────────────────

type kokkaiPipeline struct {
	collector *collect.Collector
	projector *project.Projector
}

func (k *kokkaiPipeline) source() string { return kokkai.SourceName }

func (k *kokkaiPipeline) cycle(ctx context.Context) {
	emitted, err := k.collector.PollOnce(ctx, kokkai.SourceName, pollBatch)
	if err != nil {
		log.Printf("poll: %v", err)
	} else {
		log.Printf("poll: emitted %d events", emitted)
	}
	projected, err := k.projector.Run(ctx)
	if err != nil {
		log.Printf("project: %v", err)
		return
	}
	log.Printf("project: %d meetings", projected)
}

func (k *kokkaiPipeline) reproject(ctx context.Context) error {
	n, err := k.projector.Reproject(ctx)
	if err != nil {
		return err
	}
	log.Printf("reproject: projected %d meetings", n)
	return nil
}

func (k *kokkaiPipeline) discover(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("discover", flag.ExitOnError)
	from := fs.String("from", "", "start date YYYY-MM-DD (inclusive)")
	until := fs.String("until", "", "end date YYYY-MM-DD (inclusive)")
	max := fs.Int("max", 0, "cap on discovered resources (0 = no cap)")
	_ = fs.Parse(args)
	if *from == "" || *until == "" {
		log.Fatal("discover requires --from and --until (YYYY-MM-DD)")
	}
	n, err := k.collector.Discover(ctx, port.ListScope{From: *from, Until: *until, Max: *max})
	if err != nil {
		log.Fatalf("discover: %v", err)
	}
	log.Printf("discover: upserted %d watches", n)
}

// ── egov-law pipeline ───────────────────────────────────────────────────────

type egovPipeline struct {
	collector *collect.EgovCollector
	projector *project.LawProjector
	differ    *diff.Differ
}

func (e *egovPipeline) source() string { return egov.SourceName }

func (e *egovPipeline) cycle(ctx context.Context) {
	emitted, err := e.collector.PollOnce(ctx, egov.SourceName, pollBatch)
	if err != nil {
		log.Printf("poll: %v", err)
	} else {
		log.Printf("poll: emitted %d events", emitted)
	}
	projected, err := e.projector.Run(ctx)
	if err != nil {
		log.Printf("project: %v", err)
		return
	}
	log.Printf("project: %d laws", projected)
	changes, err := e.differ.Run(ctx)
	if err != nil {
		log.Printf("diff: %v", err)
		return
	}
	log.Printf("diff: %d changes", changes)
}

func (e *egovPipeline) reproject(ctx context.Context) error {
	n, err := e.projector.Reproject(ctx)
	if err != nil {
		return fmt.Errorf("project: %w", err)
	}
	log.Printf("reproject: projected %d laws", n)
	c, err := e.differ.Reproject(ctx)
	if err != nil {
		return fmt.Errorf("diff: %w", err)
	}
	log.Printf("reproject: recomputed %d changes", c)
	return nil
}

func (e *egovPipeline) discover(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("discover", flag.ExitOnError)
	from := fs.String("from", "", "start date YYYY-MM-DD (inclusive; required for --updated)")
	until := fs.String("until", "", "end date YYYY-MM-DD (inclusive; required for --updated)")
	max := fs.Int("max", 0, "cap on discovered resources (0 = no cap)")
	lawType := fs.String("law-type", "", "filter backfill by law_type (Act | CabinetOrder | ...)")
	updated := fs.Bool("updated", false, "discover via 更新法令一覧 over the date window instead of full backfill")
	_ = fs.Parse(args)

	scope := port.ListScope{From: *from, Until: *until, Max: *max}
	var n int
	var err error
	if *updated {
		if *from == "" || *until == "" {
			log.Fatal("discover --updated requires --from and --until (YYYY-MM-DD)")
		}
		n, err = e.collector.DiscoverUpdated(ctx, scope)
	} else {
		n, err = e.collector.Discover(ctx, scope, *lawType)
	}
	if err != nil {
		log.Fatalf("discover: %v", err)
	}
	log.Printf("discover: upserted %d watches", n)
}

// ── shared driver ───────────────────────────────────────────────────────────

func runDaemon(ctx context.Context, p pipeline) {
	log.Printf("collector daemon started (source=%s, interval=%s)", p.source(), daemonInterval)
	ticker := time.NewTicker(daemonInterval)
	defer ticker.Stop()
	for {
		p.cycle(ctx)
		select {
		case <-ctx.Done():
			log.Print("collector daemon stopping")
			return
		case <-ticker.C:
		}
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `usage: collector [--source kokkai|egov-law] <run|poll-once|reproject|discover>

  run         daemon: poll due watches + project on a loop
  poll-once   one poll+project cycle, then exit
  reproject   truncate read models and replay from seq 0
  discover --from YYYY-MM-DD --until YYYY-MM-DD [--max N]
              egov-law also: [--law-type T] [--updated]
`)
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
