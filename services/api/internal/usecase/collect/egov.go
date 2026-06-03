package collect

import (
	"context"
	"fmt"

	"s4rciv.org/api/internal/gateway/egov"
	"s4rciv.org/api/internal/port"
)

// EgovCollector is the command-side collector for the egov-law source. It reuses
// the generic poll path (EnsureStream -> Fetch -> Decide -> Append -> MarkPolled)
// via an embedded Collector and only differs in discovery: the e-Gov 法令一覧
// (backfill) and 更新法令一覧 (re-poll) instead of kokkai's meeting_list.
type EgovCollector struct {
	*Collector
	control port.ControlStore
	lister  port.LawLister
}

func NewEgov(
	log port.EventLog, fetcher port.ResourceFetcher, control port.ControlStore,
	lister port.LawLister, clock port.Clock, ids port.IDGenerator, cfg Config,
) *EgovCollector {
	base := New(log, fetcher, control, nil, clock, ids, cfg)
	return &EgovCollector{Collector: base, control: control, lister: lister}
}

// Discover backfills the watch list from /laws (optionally filtered by law_type).
func (c *EgovCollector) Discover(ctx context.Context, scope port.ListScope, lawType string) (int, error) {
	refs, err := c.lister.ListLaws(ctx, scope, lawType)
	if err != nil {
		return 0, fmt.Errorf("list laws: %w", err)
	}
	return c.upsert(ctx, refs)
}

// DiscoverUpdated adds laws updated within the scope window (from 更新法令一覧).
func (c *EgovCollector) DiscoverUpdated(ctx context.Context, scope port.ListScope) (int, error) {
	refs, err := c.lister.ListUpdated(ctx, scope)
	if err != nil {
		return 0, fmt.Errorf("list updated laws: %w", err)
	}
	return c.upsert(ctx, refs)
}

func (c *EgovCollector) upsert(ctx context.Context, refs []port.LawRef) (int, error) {
	for _, r := range refs {
		if err := c.control.UpsertWatch(ctx, port.Watch{
			StreamID: r.StreamID, Source: egov.SourceName,
			SourceLocalKey: r.SourceLocalKey, CanonicalURL: r.CanonicalURL,
		}); err != nil {
			return 0, fmt.Errorf("upsert watch %s: %w", r.StreamID, err)
		}
	}
	return len(refs), nil
}
