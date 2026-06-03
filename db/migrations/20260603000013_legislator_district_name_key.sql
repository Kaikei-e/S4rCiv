-- name_key lets 参議院 votes join the roster by normalized name (ADR-000010). The 参
-- vote-result pages carry no ふりがな, so person_id (which bakes in yomi) can't match a
-- name-only vote; the name join uses this normalized key instead. Existing 衆 rows get
-- '' until the giin-roster projector replays (衆 uses the person_id join, not this).
ALTER TABLE interpretation.legislator_district ADD COLUMN name_key text NOT NULL DEFAULT '';
CREATE INDEX legislator_district_namekey_idx ON interpretation.legislator_district (name_key);
