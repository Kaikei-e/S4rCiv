-- Seed the giin-roster (両院公式議員名簿) source into the control plane (ADR-000008).
-- control is mutable operational state (ADR-000001): operators may tune these at
-- runtime. rate_limit_ms is seeded at the uniform 30s established by migration
-- 20260603000010 (that earlier pin cannot re-apply to a source added after it), so
-- this source starts compliant with the §1 floor without relying on a later sweep.
-- The roster is a low-frequency fetch (it changes only on 改選/補選/会派異動), so 30s
-- is amply courteous.
--
-- giin-roster spans BOTH houses — 衆議院 (www.shugiin.go.jp) and 参議院
-- (www.sangiin.go.jp). base_url anchors the registry on the 衆議院 site; the two
-- per-house rosters are registered as separate Streams (control.watch) at discover
-- time, each with its own canonical_url. The gateway is a public-page HTTP GET
-- adapter (passive/read-only, §4-1) with an identifying User-Agent (§7).
INSERT INTO control.source (source, base_url, rate_limit_ms, user_agent, robots_policy, enabled)
VALUES (
  'giin-roster',
  'https://www.shugiin.go.jp',
  30000,
  'S4rCiv-collect/0.1.0 (+mailto:contact@example.org)',
  '{"respect": true, "source_of_truth": "live"}'::jsonb,
  true
);
