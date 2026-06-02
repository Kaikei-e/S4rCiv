-- Control schema: mutable operational state — neither immutable ground truth nor a
-- recomputable projection (see ADR-000001). Kept physically separate so the observation
-- and interpretation schemas stay principled. Freely mutable.

CREATE SCHEMA control;

-- Source/adapter registry: how each source is fetched (compliance-bearing settings).
CREATE TABLE control.source (
  source        text PRIMARY KEY,               -- 'kokkai', 'egov-law', ...
  base_url      text NOT NULL,
  rate_limit_ms integer NOT NULL,               -- per-source serial interval
  user_agent    text NOT NULL,                  -- identifying UA with contact
  robots_policy jsonb,
  enabled       boolean NOT NULL DEFAULT true,
  CONSTRAINT source_rate_limit_positive CHECK (rate_limit_ms > 0)
);

-- What S4rCiv watches. The seed list; stream_id matches the deterministic identity used
-- in observation.stream once the resource is first observed.
CREATE TABLE control.watch (
  stream_id        text PRIMARY KEY,
  source           text NOT NULL REFERENCES control.source (source),
  source_local_key text NOT NULL,
  canonical_url    text NOT NULL,
  enabled          boolean NOT NULL DEFAULT true,
  added_at         timestamptz NOT NULL DEFAULT now()
);

-- Per-stream polling cursor and backoff. Churns on every poll; deliberately not in
-- observation.stream so the immutable identity row stays clean.
CREATE TABLE control.poll_state (
  stream_id            text PRIMARY KEY,
  last_polled_at       timestamptz,
  next_due_at          timestamptz,
  etag                 text,
  last_modified        text,
  backoff_until        timestamptz,
  consecutive_failures integer NOT NULL DEFAULT 0
);
