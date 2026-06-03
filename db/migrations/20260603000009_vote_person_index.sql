-- Per-legislator 記名投票 record axis (ADR-000006).
--
-- This index is added DELIBERATELY. Named votes are factual records of accountable
-- public actors (Diet members), so compiling "how legislator X voted" is legitimate
-- and is the ONLY per-person axis S4rCiv exposes (served by ListLegislatorVotes,
-- high-confidence identities only). It is the asymmetric counterpart to
-- interpretation.speech(person_id), which is intentionally LEFT UNINDEXED
-- (ADR-000004) to prevent a per-person speech anthology.
CREATE INDEX vote_person_idx ON interpretation.vote (person_id);
