-- M1 interpretation read models for the kokkai (国会会議録) adapter.
-- Tier 2 disposable projections (ADR-000002): NO mutation guard — projectors
-- rebuild these in place (TRUNCATE -> reset projector_offset -> replay). Each row
-- carries provenance (observation_seq) into the immutable ground truth and an
-- event-derived observed_at (never a projector wall-clock — invariant 3).
-- Read models reference observation only, never each other, so each can be
-- truncated independently during a reproject.

-- ── Popolo: people / organizations / memberships ────────────────────────────────
-- Identity is conservative and source-local (ADR-000004 anti-profiling): person_id
-- is a deterministic function of the observed (name, yomi); no cross-source name
-- resolution, no Wikidata. Homonym collisions are flagged via identity_confidence.

CREATE TABLE interpretation.person (
  person_id           text PRIMARY KEY,        -- deterministic id from normalized (name, yomi)
  name                text NOT NULL,
  yomi                text NOT NULL,
  identity_confidence text NOT NULL,
  first_observation_seq bigint NOT NULL REFERENCES observation.event (seq),
  last_observation_seq  bigint NOT NULL REFERENCES observation.event (seq),
  CONSTRAINT person_identity_confidence_known
    CHECK (identity_confidence IN ('high', 'medium', 'low'))
);

CREATE TABLE interpretation.organization (
  org_id         text PRIMARY KEY,             -- deterministic id from normalized name
  name           text NOT NULL,
  classification text NOT NULL                 -- 'parliamentary_group' (会派) for M1
);

CREATE TABLE interpretation.membership (
  id              bigserial PRIMARY KEY,
  person_id       text NOT NULL REFERENCES interpretation.person (person_id),
  organization_id text NOT NULL REFERENCES interpretation.organization (org_id),
  role            text,
  first_observation_seq bigint NOT NULL REFERENCES observation.event (seq),
  last_observation_seq  bigint NOT NULL REFERENCES observation.event (seq),
  UNIQUE (person_id, organization_id)
);
CREATE INDEX membership_person_idx ON interpretation.membership (person_id);

-- ── Meeting / speech read model ─────────────────────────────────────────────────
-- One meeting row per 21-char issueID (= one observation stream). Speeches are
-- normalized children. speaker is an attribute; we deliberately do NOT index
-- speech(person_id): the API offers meeting / timeline / diff axes only, never a
-- per-person speech anthology (ADR-000004, Copyright Act art. 40 proviso).

CREATE TABLE interpretation.meeting (
  issue_id        text PRIMARY KEY,
  stream_id       text NOT NULL,
  session         integer,                     -- 国会回次
  house           text,                        -- 衆議院 / 参議院 / 両院
  meeting_name    text,                        -- 会議名 (本会議 / 委員会名)
  issue           text,                        -- 号
  meeting_date    date,
  permalink       text,                        -- NDL reference URL (attribution)
  was_ocr         boolean NOT NULL DEFAULT false,
  observation_seq bigint NOT NULL REFERENCES observation.event (seq),
  observed_at     timestamptz NOT NULL         -- event-time fetch timestamp
);
CREATE INDEX meeting_session_house_idx ON interpretation.meeting (session, house);
CREATE INDEX meeting_date_idx ON interpretation.meeting (meeting_date);

CREATE TABLE interpretation.speech (
  speech_id        text PRIMARY KEY,
  issue_id         text NOT NULL,
  speech_order     integer NOT NULL,
  speaker          text,
  speaker_yomi     text,
  speaker_group    text,                       -- 会派
  speaker_position text,                       -- 役職
  speech           text,                       -- full text (art. 40 / PDL1.0)
  speech_url       text,
  person_id        text,                       -- Popolo link, NULL when unresolved
  observation_seq  bigint NOT NULL REFERENCES observation.event (seq),
  observed_at      timestamptz NOT NULL
);
CREATE INDEX speech_issue_idx ON interpretation.speech (issue_id, speech_order);

-- ── Vote: VoteEvent / Vote (Popolo voting) ──────────────────────────────────────
-- Built by a deterministic parser over the meeting snapshot (no LLM). Low-confidence
-- or substantive extractions set needs_review; the human verdict is recorded as a
-- Tier 1 interpretation.event and folded back on reproject. extractor_version makes
-- re-extraction a reproject (invariant 4/6).

CREATE TABLE interpretation.vote_event (
  vote_event_id     text PRIMARY KEY,
  issue_id          text NOT NULL,
  motion            text,                      -- 議案・件名
  yes_count         integer,
  no_count          integer,
  abstain_count     integer,
  result            text,                      -- 'passed' | 'rejected' | 'unknown'
  confidence        text NOT NULL,
  needs_review      boolean NOT NULL DEFAULT false,
  extractor_version text NOT NULL,
  observation_seq   bigint NOT NULL REFERENCES observation.event (seq),
  observed_at       timestamptz NOT NULL,
  CONSTRAINT vote_event_confidence_known
    CHECK (confidence IN ('high', 'medium', 'low')),
  CONSTRAINT vote_event_result_known
    CHECK (result IN ('passed', 'rejected', 'unknown'))
);
CREATE INDEX vote_event_issue_idx ON interpretation.vote_event (issue_id);

CREATE TABLE interpretation.vote (
  id            bigserial PRIMARY KEY,
  vote_event_id text NOT NULL REFERENCES interpretation.vote_event (vote_event_id),
  option        text NOT NULL,                 -- 'yes' | 'no' | 'abstain'
  voter_name    text NOT NULL,                 -- raw extracted name
  person_id     text,                          -- Popolo link, NULL when unresolved
  confidence    text NOT NULL,
  CONSTRAINT vote_option_known CHECK (option IN ('yes', 'no', 'abstain')),
  CONSTRAINT vote_confidence_known CHECK (confidence IN ('high', 'medium', 'low'))
);
CREATE INDEX vote_event_fk_idx ON interpretation.vote (vote_event_id);
