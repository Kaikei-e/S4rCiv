-- Observation plane: immutable, hash-chained ground truth (see ADR-000001).
-- Two integrity layers live here: a per-stream content chain (prev_content_hash)
-- and a single global log chain (log_prev_hash/log_hash) over every event.
-- Mutation is blocked by triggers; a dedicated least-privilege app role that also
-- REVOKEs UPDATE/DELETE is an infra follow-up (the triggers already bind table owners).

-- Resource identity. Insert-once; polling/operational state lives in the control schema.
CREATE TABLE observation.stream (
  stream_id        text PRIMARY KEY,            -- '{source}:{source_local_key}'
  source           text NOT NULL,
  source_local_key text NOT NULL,
  canonical_url    text,
  UNIQUE (source, source_local_key)
);

-- Content-addressed raw payloads. Immutable by construction: identical bytes are
-- the same row, so a snapshot can only be added, never changed.
CREATE TABLE observation.snapshot (
  content_hash bytea PRIMARY KEY,               -- sha256 of the fetched bytes
  bytes        bytea,                           -- compressed payload, when mirrored
  external_ref text,                            -- e.g. Internet Archive URL, when not mirrored
  byte_size    bigint NOT NULL,
  media_type   text,
  was_ocr      boolean NOT NULL DEFAULT false,  -- extraction quality (a mechanical fetch fact)
  CONSTRAINT snapshot_has_payload CHECK (bytes IS NOT NULL OR external_ref IS NOT NULL)
);

CREATE TYPE observation.event_type AS ENUM
  ('ResourceObserved', 'ResourceChanged', 'ResourceVanished', 'ResourceRestored');

-- The append-only event log. `seq` (global order) is assigned by the append trigger;
-- the application never supplies it. `log_hash` is computed by the application over the
-- business facts + chain link; the DB only enforces ordering and linkage continuity.
CREATE TABLE observation.event (
  seq                 bigint PRIMARY KEY,
  event_id            uuid NOT NULL UNIQUE,           -- uuidv7, external identifier
  stream_id           text NOT NULL REFERENCES observation.stream (stream_id),
  stream_seq          bigint NOT NULL,
  type                observation.event_type NOT NULL,
  source              text NOT NULL,
  fetcher_version     text NOT NULL,
  observed_at         timestamptz NOT NULL,           -- business fact (hashed)
  source_published_at timestamptz,                    -- business fact (hashed, NULL when absent)
  content_hash        bytea REFERENCES observation.snapshot (content_hash),
  prev_content_hash   bytea,                          -- per-stream content chain
  log_prev_hash       bytea NOT NULL,                 -- global log chain link
  log_hash            bytea NOT NULL,                 -- application-computed
  recorded_at         timestamptz NOT NULL DEFAULT now(),  -- ops metadata only, never hashed
  CONSTRAINT event_stream_seq_unique UNIQUE (stream_id, stream_seq),
  CONSTRAINT vanished_has_no_content CHECK ((type = 'ResourceVanished') = (content_hash IS NULL))
);

-- Single-row cursor that serializes appends and pins the chain head. Mutable, but
-- fully reconstructible from the event table — it is not ground truth itself.
CREATE TABLE observation.chain_head (
  id       integer PRIMARY KEY DEFAULT 1,
  seq      bigint NOT NULL,
  log_hash bytea NOT NULL,
  CONSTRAINT chain_head_singleton CHECK (id = 1)
);
INSERT INTO observation.chain_head (id, seq, log_hash)
VALUES (1, 0, '\x0000000000000000000000000000000000000000000000000000000000000000');

-- Append-only attestations of the log state. Starts as the linked-list head;
-- root_hash becomes a Merkle root and signature/signer_key_id fill in once signing exists.
CREATE TABLE observation.checkpoint (
  checkpoint_id uuid PRIMARY KEY,
  through_seq   bigint NOT NULL,
  tree_size     bigint NOT NULL,
  root_hash     bytea NOT NULL,
  alg_version   text NOT NULL,                  -- 'linked-v1' -> 'merkle-v1'
  signature     bytea,
  signer_key_id text,
  recorded_at   timestamptz NOT NULL DEFAULT now()
);

-- Serialize on the chain_head row, verify the link, assign seq, advance the head.
CREATE FUNCTION observation.append_event() RETURNS trigger
  LANGUAGE plpgsql AS $$
DECLARE
  head observation.chain_head%ROWTYPE;
BEGIN
  SELECT * INTO head FROM observation.chain_head WHERE id = 1 FOR UPDATE;
  IF NEW.log_prev_hash IS DISTINCT FROM head.log_hash THEN
    RAISE EXCEPTION 'broken observation log chain: head expects prev_hash %, got %',
      encode(head.log_hash, 'hex'), encode(NEW.log_prev_hash, 'hex');
  END IF;
  NEW.seq := head.seq + 1;
  UPDATE observation.chain_head SET seq = NEW.seq, log_hash = NEW.log_hash WHERE id = 1;
  RETURN NEW;
END;
$$;
CREATE TRIGGER append_event BEFORE INSERT ON observation.event
  FOR EACH ROW EXECUTE FUNCTION observation.append_event();

-- Reject every mutation of the immutable tables (chain_head is deliberately excluded).
CREATE FUNCTION observation.reject_mutation() RETURNS trigger
  LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'observation.% is append-only: % rejected', TG_TABLE_NAME, TG_OP;
END;
$$;
CREATE TRIGGER no_mutate BEFORE UPDATE OR DELETE ON observation.stream
  FOR EACH ROW EXECUTE FUNCTION observation.reject_mutation();
CREATE TRIGGER no_mutate BEFORE UPDATE OR DELETE ON observation.snapshot
  FOR EACH ROW EXECUTE FUNCTION observation.reject_mutation();
CREATE TRIGGER no_mutate BEFORE UPDATE OR DELETE ON observation.event
  FOR EACH ROW EXECUTE FUNCTION observation.reject_mutation();
CREATE TRIGGER no_mutate BEFORE UPDATE OR DELETE ON observation.checkpoint
  FOR EACH ROW EXECUTE FUNCTION observation.reject_mutation();
