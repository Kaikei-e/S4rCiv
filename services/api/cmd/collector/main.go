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
	"s4rciv.org/api/internal/gateway/giinroster"
	"s4rciv.org/api/internal/gateway/kokkai"
	"s4rciv.org/api/internal/gateway/sangiin"
	"s4rciv.org/api/internal/port"
	"s4rciv.org/api/internal/usecase/collect"
	"s4rciv.org/api/internal/usecase/diff"
	"s4rciv.org/api/internal/usecase/project"
)

const (
	collectorVersion = "S4rCiv-collect/0.1.0"
	daemonInterval   = 60 * time.Second
	pollBatch        = 100
	// discoverInterval is how often the daemon refreshes the watch list in-process
	// (ADR-000012). Much coarser than daemonInterval: new resources appear daily at
	// most and source records carry a publish lag.
	discoverInterval = 24 * time.Hour
	// discoverWindowDays is the rolling look-back (by resource date) for date-windowed
	// sources (kokkai meeting_list, egov 更新法令一覧). Wide enough to re-scan late-
	// published records and absorb a multi-week daemon outage; upsert is idempotent so
	// re-scanning is cheap and safe (ADR-000012). Long gaps still need a manual catch-up.
	discoverWindowDays = 90
)

// recentScope is the rolling discover window [today-discoverWindowDays, today],
// inclusive, in the YYYY-MM-DD form the date-windowed listers expect.
func recentScope(now time.Time) port.ListScope {
	return port.ListScope{
		From:  now.AddDate(0, 0, -discoverWindowDays).Format("2006-01-02"),
		Until: now.Format("2006-01-02"),
	}
}

// logDiscover reports an auto-discover outcome without ever being fatal (the
// daemon must keep polling even if a discover cycle fails).
func logDiscover(source string, n int, err error) {
	if err != nil {
		log.Printf("auto-discover %s: %v", source, err)
		return
	}
	log.Printf("auto-discover %s: upserted %d watches", source, n)
}

// pipeline abstracts the per-source command side (poll + project [+ diff]).
type pipeline interface {
	source() string
	cycle(ctx context.Context)
	reproject(ctx context.Context) error
	discover(ctx context.Context, args []string)
	// autoDiscover refreshes the watch list in-daemon so new resources are
	// followed without a manual discover (ADR-000012). It runs in the same
	// process as cycle(), sharing the source's serial rate limiter (DISCIPLINE
	// §1), so it never needs the daemon paused. It must not be fatal: a discover
	// error is logged and the daemon keeps polling.
	autoDiscover(ctx context.Context)
}

func main() {
	fs := flag.NewFlagSet("collector", flag.ExitOnError)
	source := fs.String("source", "kokkai", "source to operate on (kokkai | egov-law | giin-roster | sangiin-vote | sangiin-roster)")
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
			collect.Config{FetcherVersion: collectorVersion, Source: kokkai.SourceName},
		)
		rm := postgres.NewReadModel(pool)
		projector := project.New(postgres.NewEventReader(pool), gw, rm, rm, source)
		return &kokkaiPipeline{collector: collector, projector: projector}, nil

	case giinroster.SourceName:
		// egovhttp is a generic rate-limited + robots-compliant GET client (GetAbs);
		// reused here for the roster pages (ADR-000008). DB is Go-owned; differ N/A.
		httpc, err := egovhttp.New(cfg.BaseURL, ua, cfg.RateLimit)
		if err != nil {
			return nil, err
		}
		gw := giinroster.New(httpc)
		collector := collect.New(
			postgres.NewEventLog(pool), gw, control, gw, sys.Clock{}, sys.IDGen{},
			collect.Config{FetcherVersion: collectorVersion, Source: giinroster.SourceName},
		)
		rm := postgres.NewRosterReadModel(pool, giinroster.StreamID(""))
		projector := project.NewRoster(postgres.NewEventReader(pool), gw, rm, rm, giinroster.SourceName, giinroster.StreamID(""))
		return &giinRosterPipeline{collector: collector, projector: projector}, nil

	case sangiin.SourceName: // 参議院本会議投票結果 (touhyoulist) — per-member roll-calls (ADR-000010)
		httpc, err := egovhttp.New(cfg.BaseURL, ua, cfg.RateLimit)
		if err != nil {
			return nil, err
		}
		gw := sangiin.New(httpc)
		collector := collect.New(
			postgres.NewEventLog(pool), gw, control, noLister{}, sys.Clock{}, sys.IDGen{},
			collect.Config{FetcherVersion: collectorVersion, Source: sangiin.SourceName},
		)
		rm := postgres.NewSangiinVoteReadModel(pool)
		projector := project.NewSangiinVote(postgres.NewEventReader(pool), gw, rm, rm, sangiin.SourceName)
		return &sangiinVotePipeline{collector: collector, projector: projector, gw: gw, control: control}, nil

	case sangiin.RosterSourceName: // 参議院議員名簿 → legislator_district (house=参議院)
		httpc, err := egovhttp.New(cfg.BaseURL, ua, cfg.RateLimit)
		if err != nil {
			return nil, err
		}
		gw := sangiin.New(httpc)
		collector := collect.New(
			postgres.NewEventLog(pool), gw, control, noLister{}, sys.Clock{}, sys.IDGen{},
			collect.Config{FetcherVersion: collectorVersion, Source: sangiin.RosterSourceName},
		)
		rm := postgres.NewRosterReadModel(pool, sangiin.RosterStreamID(""))
		projector := project.NewRoster(postgres.NewEventReader(pool), gw, rm, rm, sangiin.RosterSourceName, sangiin.RosterStreamID(""))
		return &sangiinRosterPipeline{collector: collector, projector: projector, gw: gw, control: control}, nil

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

func (k *kokkaiPipeline) autoDiscover(ctx context.Context) {
	n, err := k.collector.Discover(ctx, recentScope(time.Now()))
	logDiscover(kokkai.SourceName, n, err)
}

// ── giin-roster pipeline ────────────────────────────────────────────────────

type giinRosterPipeline struct {
	collector *collect.Collector
	projector *project.RosterProjector
}

func (g *giinRosterPipeline) source() string { return giinroster.SourceName }

func (g *giinRosterPipeline) cycle(ctx context.Context) {
	emitted, err := g.collector.PollOnce(ctx, giinroster.SourceName, pollBatch)
	if err != nil {
		log.Printf("poll: %v", err)
	} else {
		log.Printf("poll: emitted %d events", emitted)
	}
	projected, err := g.projector.Run(ctx)
	if err != nil {
		log.Printf("project: %v", err)
		return
	}
	log.Printf("project: %d roster pages", projected)
}

func (g *giinRosterPipeline) reproject(ctx context.Context) error {
	n, err := g.projector.Reproject(ctx)
	if err != nil {
		return err
	}
	log.Printf("reproject: projected %d roster pages", n)
	return nil
}

func (g *giinRosterPipeline) discover(ctx context.Context, _ []string) {
	// The roster is a fixed page set, so discover ignores --from/--until.
	n, err := g.collector.Discover(ctx, port.ListScope{})
	if err != nil {
		log.Fatalf("discover: %v", err)
	}
	log.Printf("discover: upserted %d watches", n)
}

func (g *giinRosterPipeline) autoDiscover(ctx context.Context) {
	// Fixed page set; re-discover daily to catch roster changes (e.g. after an election).
	n, err := g.collector.Discover(ctx, port.ListScope{})
	logDiscover(giinroster.SourceName, n, err)
}

// ── 参議院 pipelines (vote + roster) ─────────────────────────────────────────

// noLister satisfies the Collector's MeetingLister dependency for sources whose
// discovery is bespoke (sangiin) and never goes through collect.Discover.
type noLister struct{}

func (noLister) ListMeetings(context.Context, port.ListScope) ([]port.MeetingRef, error) {
	return nil, nil
}

type sangiinVotePipeline struct {
	collector *collect.Collector
	projector *project.SangiinVoteProjector
	gw        *sangiin.Gateway
	control   port.ControlStore
}

func (p *sangiinVotePipeline) source() string { return sangiin.SourceName }

func (p *sangiinVotePipeline) cycle(ctx context.Context) {
	emitted, err := p.collector.PollOnce(ctx, sangiin.SourceName, pollBatch)
	if err != nil {
		log.Printf("poll: %v", err)
	} else {
		log.Printf("poll: emitted %d events", emitted)
	}
	projected, err := p.projector.Run(ctx)
	if err != nil {
		log.Printf("project: %v", err)
		return
	}
	log.Printf("project: %d vote pages", projected)
}

func (p *sangiinVotePipeline) reproject(ctx context.Context) error {
	n, err := p.projector.Reproject(ctx)
	if err != nil {
		return err
	}
	log.Printf("reproject: projected %d vote pages", n)
	return nil
}

func (p *sangiinVotePipeline) discover(ctx context.Context, _ []string) {
	refs, err := p.gw.DiscoverVotes(ctx)
	if err != nil {
		log.Fatalf("discover: %v", err)
	}
	for _, r := range refs {
		if err := p.control.UpsertWatch(ctx, port.Watch{
			StreamID: r.StreamID, Source: sangiin.SourceName,
			SourceLocalKey: r.SourceLocalKey, CanonicalURL: r.CanonicalURL,
		}); err != nil {
			log.Fatalf("upsert watch %s: %v", r.StreamID, err)
		}
	}
	log.Printf("discover: upserted %d watches", len(refs))
}

func (p *sangiinVotePipeline) autoDiscover(ctx context.Context) {
	// 記名投票 accrues within a session; re-discover daily so new roll-calls are followed.
	refs, err := p.gw.DiscoverVotes(ctx)
	if err != nil {
		logDiscover(sangiin.SourceName, 0, err)
		return
	}
	for _, r := range refs {
		if err := p.control.UpsertWatch(ctx, port.Watch{
			StreamID: r.StreamID, Source: sangiin.SourceName,
			SourceLocalKey: r.SourceLocalKey, CanonicalURL: r.CanonicalURL,
		}); err != nil {
			logDiscover(sangiin.SourceName, 0, fmt.Errorf("upsert %s: %w", r.StreamID, err))
			return
		}
	}
	logDiscover(sangiin.SourceName, len(refs), nil)
}

type sangiinRosterPipeline struct {
	collector *collect.Collector
	projector *project.RosterProjector
	gw        *sangiin.Gateway
	control   port.ControlStore
}

func (p *sangiinRosterPipeline) source() string { return sangiin.RosterSourceName }

func (p *sangiinRosterPipeline) cycle(ctx context.Context) {
	emitted, err := p.collector.PollOnce(ctx, sangiin.RosterSourceName, pollBatch)
	if err != nil {
		log.Printf("poll: %v", err)
	} else {
		log.Printf("poll: emitted %d events", emitted)
	}
	projected, err := p.projector.Run(ctx)
	if err != nil {
		log.Printf("project: %v", err)
		return
	}
	log.Printf("project: %d roster pages", projected)
}

func (p *sangiinRosterPipeline) reproject(ctx context.Context) error {
	n, err := p.projector.Reproject(ctx)
	if err != nil {
		return err
	}
	log.Printf("reproject: projected %d roster pages", n)
	return nil
}

func (p *sangiinRosterPipeline) discover(ctx context.Context, _ []string) {
	ref, err := p.gw.DiscoverRoster(ctx)
	if err != nil {
		log.Fatalf("discover: %v", err)
	}
	if err := p.control.UpsertWatch(ctx, port.Watch{
		StreamID: ref.StreamID, Source: sangiin.RosterSourceName,
		SourceLocalKey: ref.SourceLocalKey, CanonicalURL: ref.CanonicalURL,
	}); err != nil {
		log.Fatalf("upsert watch: %v", err)
	}
	log.Printf("discover: upserted 1 watch (%s)", ref.CanonicalURL)
}

func (p *sangiinRosterPipeline) autoDiscover(ctx context.Context) {
	// Single fixed roster page; re-discover daily to track membership changes.
	ref, err := p.gw.DiscoverRoster(ctx)
	if err != nil {
		logDiscover(sangiin.RosterSourceName, 0, err)
		return
	}
	if err := p.control.UpsertWatch(ctx, port.Watch{
		StreamID: ref.StreamID, Source: sangiin.RosterSourceName,
		SourceLocalKey: ref.SourceLocalKey, CanonicalURL: ref.CanonicalURL,
	}); err != nil {
		logDiscover(sangiin.RosterSourceName, 0, err)
		return
	}
	logDiscover(sangiin.RosterSourceName, 1, nil)
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

func (e *egovPipeline) autoDiscover(ctx context.Context) {
	// 更新法令一覧 over the rolling window picks up newly enacted/amended laws
	// (full backfill stays manual). Same path as `discover --updated`.
	n, err := e.collector.DiscoverUpdated(ctx, recentScope(time.Now()))
	logDiscover(egov.SourceName, n, err)
}

// ── shared driver ───────────────────────────────────────────────────────────

func runDaemon(ctx context.Context, p pipeline) {
	log.Printf("collector daemon started (source=%s, interval=%s, discover=%s)",
		p.source(), daemonInterval, discoverInterval)
	ticker := time.NewTicker(daemonInterval)
	defer ticker.Stop()
	// Zero value forces an autoDiscover on the first cycle so a fresh start (or a
	// restart after downtime) refreshes the watch list before polling (ADR-000012).
	var lastDiscover time.Time
	for {
		if time.Since(lastDiscover) >= discoverInterval {
			p.autoDiscover(ctx)
			lastDiscover = time.Now()
		}
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
	fmt.Fprint(os.Stderr, `usage: collector [--source kokkai|egov-law|giin-roster|sangiin-vote|sangiin-roster] <run|poll-once|reproject|discover>

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
