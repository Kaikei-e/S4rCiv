-- Carry speech-level provenance on extracted votes: which speech a VoteEvent was
-- parsed from. Required so a needs_review verdict (and the UI) can land on the
-- exact source text + its permalink, never a decontextualized conclusion
-- (DISCIPLINE §7; immutable-design "Why as first-class"). Disposable read model,
-- backfilled by a reproject — no re-fetch from the source.
ALTER TABLE interpretation.vote_event ADD COLUMN source_speech_id text;
