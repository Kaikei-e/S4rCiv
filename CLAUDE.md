# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Key documents — read these first

| File | Role | Authority |
|---|---|---|
| [`docs/concepts/CORE_CONCEPT_0001.md`](docs/concepts/CORE_CONCEPT_0001.md) | Single source of truth: purpose, scope, architecture, event schema, abuse model (Japanese) | **Wins on any conflict** |
| [`DISCIPLINE.md`](DISCIPLINE.md) | The canonical list of **what you must NOT do** (rate limits, passivity, anti-doxxing, data integrity, …) | Binding. Read before writing any collector or touching the data layer |
| `CLAUDE.md` (this file) | Quick orientation and summary | Defers to the two above |

## Project state

**Concept stage (v0). There is no code yet** — only `LICENSE`, a one-line `README.md`, the concept doc, and `DISCIPLINE.md`. There are no build, lint, or test commands because nothing is implemented. Do not invent them; add the real commands here when scaffolding begins.

## What s4rCiv is

A **passive, read-only "flight recorder" for public records** plus a situation-room dashboard for citizens. It continuously collects Japanese public primary-source data (legislation, laws/ordinances, public money, procurement) and records *changes* — including deletions — into an immutable, hash-chained log, so anyone can trace **when / what / how something changed or was removed**. Non-partisan civic-tech infrastructure in the g0v / Audrey Tang lineage; a modern successor to EDGI Web Monitoring.

## Design principles (the reason the project exists)

These are constraints, not aspirations. Any feature or dependency that violates one is wrong by definition — surface the conflict instead of building it. The **operational prohibitions derived from these are enumerated in [`DISCIPLINE.md`](DISCIPLINE.md)**; the summary here is for orientation.

1. **Passive / read-only** — public-endpoint HTTP GET only. No auth, no submissions, no writes, no automated actions. A *sentinel*, never an actor.
2. **Public primary sources only.**
3. **Append-only, hash-chained log** — keep everything (incl. deletions/reversions); the log is tamper-evident so s4rCiv can prove it has not rewritten history.
4. **Dual-plane separation** — *observation plane* (raw snapshots + hash-chained events; immutable ground truth) vs *interpretation plane* (normalized entities + classification + LLM summaries; recomputable projections carrying provenance + confidence).
5. **Standards-based, no silos** — Akoma Ntoso (laws/proceedings), Popolo (people/roles), OCDS (procurement).
6. **AI summarizes only, never judges** — no scoring/opinions in the data layer; every summary links back to source text/diff with confidence + provenance.
7. **Built-in source compliance** — per-source rate limiting, robots.txt, identifying User-Agent, attribution + fetch timestamp on every record, Internet Archive dual-sourcing where possible.

The hardest, most load-bearing rules: **never profile/dox private individuals** (the target is always accountable public actors), **never act partisanly** (same mechanical pipeline for all), and **never present a decontextualized diff** (always with surrounding context + full-text link).

## Planned architecture (target, not yet built)

Single self-hosted binary, adapter-based, embedded event store.

```
Source Adapters (Rust/Go, HTTP GET only)
  → Normalizer (AKN / Popolo / OCDS + diff/classify)
  → Event Log (CQRS, append-only, hash-chained = observation plane)
  → Read Models (timeline / entity / vote / contract / funding + LLM summaries = interpretation plane)
  → Web (SvelteKit + MapLibre/WebGPU situation-room dashboard)
```

- **Source adapters are the unit of extension.** New source = new pluggable adapter (collect + normalize), loosely coupled and versioned. Record observation gaps as `ResourceVanished` (silence is information).
- **Observation events**: `ResourceObserved`, `ResourceChanged`, `ResourceVanished`, `ResourceRestored`. Envelope schema (uuidv7 event_id, stream_id, seq, content_hash, prev_content_hash, log_prev_hash, payload_ref, diff, confidence) in concept §9.1.
- **Diff precision by source type**: structured XML/AKN → structural diff (article/clause level); semi-structured/PDF → text diff with `confidence=low` + source-PDF link. Changes heuristically classified `administrative` vs `substantive`; `substantive` → human review queue, and the review result is itself an interpretation-plane event.

### MVP order (concept §11)

M0 skeleton (binary, hash-chained event log, adapter interface, plane separation) → M1 国会会議録 API adapter (Diet proceedings, VoteEvents) → M2 e-Gov 法令 API v2 adapter (laws, AKN structural diff) → M3 dashboard v0 → M4 map → M5 summaries v0 → M6 public release.

## Source catalog (priority order; concept §6)

MVP sources:

- **国会会議録検索API** (`https://kokkai.ndl.go.jp/api/`) — three GET endpoints: `meeting_list` (metadata, ≤100/req), `meeting` (full speech text, ≤10/req), `speech` (per-speech, ≤100/req). JSON via `recordPacking=json`; pagination via `startRecord` / `maximumRecords` / response `nextRecordPosition`; search-condition total ≤2000 bytes; stream unit = 21-char `issueID`. No registration, but **serial access with a few seconds between requests** is required and abuse is blocked without notice. NDL holds copyright on the database and on NDL-staff speeches → attribution mandatory. Coverage: 1st Diet (1947)–present. Spec: `https://kokkai.ndl.go.jp/api.html`
- **e-Gov 法令API v2** (`https://laws.e-gov.go.jp/api/2/`) — change detection via 更新法令一覧 (`https://laws.e-gov.go.jp/update/`).

Later: 官報/告示 base registry (≈2026), 政治資金 (online publication mandated 2027-01), procurement/OCDS (`https://www.p-portal.go.jp/`).

## Open questions affecting implementation (concept §14)

Deliberately unresolved — confirm with the user before baking in: hosting model; self-mirroring vs Internet-Archive-linking of primary documents; administrative/substantive classification heuristics and thresholds; low-confidence OCR handling in the UI; private-donor display granularity; alert delivery boundaries.

## License

The server body (this repository) is **AGPL-3.0**, per concept §13 and mySociety-style civic-tech convention (keep SaaS forks open). Decided separately when those artifacts exist: data/schema license (CC0 or CC BY) and client-library license (Apache-2.0/MIT) — not covered by the repository `LICENSE`.
