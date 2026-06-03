-- 参議院 vote-result + roster sources (ADR-000010). Both are public GET pages on
-- www.sangiin.go.jp (passive/read-only, §4-1). 30s per-source interval (DISCIPLINE §1)
-- with an identifying User-Agent (§7). One vote page = one full roll-call, so the
-- request volume is tiny.
INSERT INTO control.source (source, base_url, rate_limit_ms, user_agent, robots_policy, enabled) VALUES
  ('sangiin-vote',   'https://www.sangiin.go.jp', 30000,
   'S4rCiv-collect/0.1.0 (+mailto:contact@example.org)',
   '{"respect": true, "source_of_truth": "live"}'::jsonb, true),
  ('sangiin-roster', 'https://www.sangiin.go.jp', 30000,
   'S4rCiv-collect/0.1.0 (+mailto:contact@example.org)',
   '{"respect": true, "source_of_truth": "live"}'::jsonb, true);
