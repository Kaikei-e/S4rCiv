-- Interpretation plane: two tiers (see ADR-000002).
--   Tier 1 — interpretation.event: append-only, hash-chained durable facts that are NOT
--            recomputable from observation (human review verdicts, corrections, overrides).
--            Survives every reproject. Its own log chain proves S4rCiv has not silently
--            rewritten its own verdicts.
--   Tier 2 — read models (everything else): disposable projections, rebuilt by projectors
--            folding observation + interpretation events. Freely TRUNCATE-able.

CREATE TABLE interpretation.event (
  seq             bigint PRIMARY KEY,           -- assigned by the append trigger
  event_id        uuid NOT NULL UNIQUE,         -- uuidv7
  type            text NOT NULL,                -- ReviewVerdict | Correction | Override
  observation_seq bigint REFERENCES observation.event (seq),  -- provenance into ground truth
  actor           text NOT NULL,                -- who decided
  decided_at      timestamptz NOT NULL,         -- business fact (hashed)
  payload         jsonb NOT NULL,
  log_prev_hash   bytea NOT NULL,               -- interpretation-plane log chain link
  log_hash        bytea NOT NULL,               -- application-computed
  recorded_at     timestamptz NOT NULL DEFAULT now()  -- ops metadata only, never hashed
);

CREATE TABLE interpretation.chain_head (
  id       integer PRIMARY KEY DEFAULT 1,
  seq      bigint NOT NULL,
  log_hash bytea NOT NULL,
  CONSTRAINT chain_head_singleton CHECK (id = 1)
);
INSERT INTO interpretation.chain_head (id, seq, log_hash)
VALUES (1, 0, '\x0000000000000000000000000000000000000000000000000000000000000000');

CREATE FUNCTION interpretation.append_event() RETURNS trigger
  LANGUAGE plpgsql AS $$
DECLARE
  head interpretation.chain_head%ROWTYPE;
BEGIN
  SELECT * INTO head FROM interpretation.chain_head WHERE id = 1 FOR UPDATE;
  IF NEW.log_prev_hash IS DISTINCT FROM head.log_hash THEN
    RAISE EXCEPTION 'broken interpretation log chain: head expects prev_hash %, got %',
      encode(head.log_hash, 'hex'), encode(NEW.log_prev_hash, 'hex');
  END IF;
  NEW.seq := head.seq + 1;
  UPDATE interpretation.chain_head SET seq = NEW.seq, log_hash = NEW.log_hash WHERE id = 1;
  RETURN NEW;
END;
$$;
CREATE TRIGGER append_event BEFORE INSERT ON interpretation.event
  FOR EACH ROW EXECUTE FUNCTION interpretation.append_event();

CREATE FUNCTION interpretation.reject_mutation() RETURNS trigger
  LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'interpretation.% is append-only: % rejected', TG_TABLE_NAME, TG_OP;
END;
$$;
CREATE TRIGGER no_mutate BEFORE UPDATE OR DELETE ON interpretation.event
  FOR EACH ROW EXECUTE FUNCTION interpretation.reject_mutation();

-- ── Tier 2: disposable read models ──────────────────────────────────────────────
-- No mutation guard: projectors rebuild these in place (TRUNCATE -> reset offset -> replay).
-- The standard-specific entity tables (Akoma Ntoso / Popolo / OCDS, vote, contract,
-- funding, timeline) are defined per adapter in M1/M2; only the diff/classification
-- read model and the projector cursor are established now.

CREATE TABLE interpretation.change (
  id               bigserial PRIMARY KEY,
  observation_seq  bigint NOT NULL REFERENCES observation.event (seq),
  differ_version   text NOT NULL,
  diff             jsonb NOT NULL,
  classification   text NOT NULL,
  class_confidence text NOT NULL,
  CONSTRAINT change_classification_known
    CHECK (classification IN ('administrative', 'substantive')),
  CONSTRAINT change_confidence_known
    CHECK (class_confidence IN ('high', 'medium', 'low'))
);
CREATE INDEX change_observation_idx ON interpretation.change (observation_seq);

CREATE TABLE interpretation.projector_offset (
  projector  text PRIMARY KEY,
  last_seq   bigint NOT NULL DEFAULT 0,
  rebuilding boolean NOT NULL DEFAULT false
);
