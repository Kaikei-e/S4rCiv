-- Pin every source's polling interval to a uniform 30s. control is mutable
-- operational state (ADR-000001); this forward migration supersedes the
-- per-source seed defaults (kokkai 5000ms / egov-law 2500ms) rather than editing
-- those applied seeds in place (which would break migrate integrity on an
-- already-migrated database). 30s sits well above the DISCIPLINE §1 "serial
-- access a few seconds apart" floor, so it tightens source compliance, never
-- relaxes it. Idempotent: an operator may already have set this at runtime.
UPDATE control.source SET rate_limit_ms = 30000;
