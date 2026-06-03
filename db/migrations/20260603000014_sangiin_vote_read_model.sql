-- 参議院本会議投票結果 (touhyoulist) per-member named votes (ADR-000010). Kept SEPARATE
-- from the kokkai vote_event/vote read models so a kokkai reproject can't clobber 参 data
-- and vice versa (read-model independence). Tier 2 disposable: the 参 vote projector
-- rebuilds it (TRUNCATE -> reset projector_offset -> replay). Each row carries provenance
-- (observation_seq) and an event-derived observed_at.

CREATE TABLE interpretation.sangiin_vote_event (
  vote_event_id   text PRIMARY KEY,          -- page slug, e.g. "221-0407-v001"
  session         integer,                   -- 国会回次
  motion          text,                      -- 議案件名
  vote_date       date,
  yes_count       integer,                   -- 賛成票 (announced total)
  no_count        integer,                   -- 反対票 (announced total)
  permalink       text,                      -- 参議院 vote-result page URL (attribution)
  observation_seq bigint NOT NULL REFERENCES observation.event (seq),
  observed_at     timestamptz NOT NULL
);
CREATE INDEX sangiin_vote_event_session_idx ON interpretation.sangiin_vote_event (session, vote_date);

CREATE TABLE interpretation.sangiin_vote (
  id                  bigserial PRIMARY KEY,
  vote_event_id       text NOT NULL REFERENCES interpretation.sangiin_vote_event (vote_event_id),
  option              text NOT NULL,         -- 'yes' | 'no' | 'abstain'
  voter_name          text NOT NULL,
  name_key            text NOT NULL,         -- normalized name → joins legislator_district(name_key) for 都道府県
  parliamentary_group text,                  -- 会派
  CONSTRAINT sangiin_vote_option_known CHECK (option IN ('yes', 'no', 'abstain'))
);
CREATE INDEX sangiin_vote_event_fk_idx ON interpretation.sangiin_vote (vote_event_id);
CREATE INDEX sangiin_vote_namekey_idx ON interpretation.sangiin_vote (name_key);
