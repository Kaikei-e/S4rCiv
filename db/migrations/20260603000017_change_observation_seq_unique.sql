-- interpretation.change is the egov-law diff/classification read model (Tier-2,
-- disposable). It holds exactly ONE row per observation event seq: the structural
-- change computed for that one ResourceChanged. Until now that "one row per seq" rule
-- lived only in the projector (a bare INSERT) with just a non-unique index, so a
-- double-apply — a reproject racing the live differ daemon, a retried batch — could
-- silently duplicate a seq and the timeline LEFT JOIN would then emit the same item
-- twice. Promote the rule to a DB invariant so a double-apply fails loud, and so
-- ApplyChange can upsert (ON CONFLICT (observation_seq)) for a reader-atomic,
-- truncate-less reproject (ADR-000024, aligning egov reproject with the kokkai one of
-- ADR-000022). No existing rows are affected: each seq already appears at most once.
--
-- The pre-existing non-unique change_observation_idx is now redundant — the UNIQUE
-- constraint creates its own index on the same column — so it is dropped.

DROP INDEX IF EXISTS interpretation.change_observation_idx;

ALTER TABLE interpretation.change
  ADD CONSTRAINT change_observation_seq_unique UNIQUE (observation_seq);
