-- Seed the kokkai (国会会議録検索API) source into the control plane. control is
-- mutable operational state (ADR-000001): operators may tune these at runtime.
-- rate_limit_ms encodes the DISCIPLINE §1 serial+interval requirement (NDL asks
-- for serial access a few seconds apart; abuse is blocked without notice).
INSERT INTO control.source (source, base_url, rate_limit_ms, user_agent, robots_policy, enabled)
VALUES (
  'kokkai',
  'https://kokkai.ndl.go.jp/api',
  3000,
  'S4rCiv-collect/0.1.0 (+mailto:contact@example.org)',
  '{"respect": true, "source_of_truth": "live"}'::jsonb,
  true
);
