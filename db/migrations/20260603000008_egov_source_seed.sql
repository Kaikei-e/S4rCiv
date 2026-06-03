-- Seed the e-Gov 法令 API v2 (egov-law) source into the control plane. control is
-- mutable operational state (ADR-000001): operators may tune these at runtime.
-- rate_limit_ms encodes the DISCIPLINE §1 serial+interval requirement; e-Gov
-- publishes no explicit limit, so we default conservatively (serial, a few seconds).
--
-- Legal footing (contrast with kokkai/ADR-000004): law text itself is outside
-- copyright (著作権法13条「権利の目的とならない著作物」), so mirroring the 法令標準XML
-- snapshot and showing full text carry no DB-copyright tension. Usage requires
-- indicating use of e-Gov 法令検索 / 法令API (政府標準利用規約 ≒ CC BY 4.0): the
-- attribution permalink is stamped on every projected record (DISCIPLINE §9).
INSERT INTO control.source (source, base_url, rate_limit_ms, user_agent, robots_policy, enabled)
VALUES (
  'egov-law',
  'https://laws.e-gov.go.jp/api/2',
  2500,
  'S4rCiv-collect/0.1.0 (+mailto:contact@example.org)',
  '{"respect": true, "source_of_truth": "live"}'::jsonb,
  true
);
