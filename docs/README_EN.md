# S4RCIV

**_sentinel for civic records_** — a passive, read-only flight recorder for public records, plus a situation-room dashboard for citizens.

![status](https://img.shields.io/badge/status-in%20development%20(M1%E2%80%93M2)-yellow)
![license](https://img.shields.io/badge/license-AGPL--3.0-blue)
[![standard-readme compliant](https://img.shields.io/badge/readme%20style-standard-brightgreen.svg)](https://github.com/RichardLitt/standard-readme)

[日本語](../README.md) | **English**

> A non-partisan, read-only civic-tech platform that continuously observes Japan's **public primary-source data** — legislation, laws and ordinances, public money, procurement — and records its **changes (including deletions) into a tamper-evident, append-only log** for anyone to trace and verify.

Democratic transparency is not served by showing only "how things stand right now." It works only when anyone can later trace **when something changed, what changed, how it changed — and what was quietly removed.** S4RCIV combines a "flight recorder" that keeps recording public primary sources with a "situation-room dashboard" that lets you survey many sources at once, putting this **traceability over time** back into citizens' hands.

S4RCIV is not a tool for confronting power. It is information infrastructure for **recording the public outputs of accountable public actors in a form anyone can verify.** It adds no opinion and no judgement to the record; all it keeps are observed facts and citations that lead back to the source. What is monitored, and by what criteria, is published — and the same mechanical pipeline is applied to every actor alike.

In lineage, S4RCIV follows the "radical transparency" and "beneficial information flows" of g0v / Audrey Tang (Plurality), and is a modern successor to the EDGI Web Monitoring project (diff-monitoring of government web pages) in the United States.

## Table of Contents

- [Why S4RCIV](#why-s4rciv)
- [What it does](#what-it-does)
- [Design principles](#design-principles)
- [What it does not do](#what-it-does-not-do)
- [Architecture](#architecture)
- [Verifiability](#verifiability)
- [Sources](#sources)
- [Status and roadmap](#status-and-roadmap)
- [Running locally](#running-locally)
- [Related work](#related-work)
- [Contributing](#contributing)
- [License](#license)
- [Acknowledgements](#acknowledgements)

## Why S4RCIV

Existing monitoring-oriented civic tech tends to be siloed into single-purpose sites — freedom-of-information, voting records, budgets — and most of them show only a snapshot of the "current value." Yet the value of public records lies in their **history of change**: when a clause was rewritten, how a contract amount shifted over time, when a published document disappeared. Without a way to confirm these after the fact, transparency becomes hollow.

S4RCIV treats **change itself**, not the current value, as its primary unit of record. Even the loss of observability — silence — is kept as information.

There is also a timing case for acting now, as several institutional tailwinds converge:

- **e-Gov Laws API v2** (released 2025-03; OpenAPI, statute XML/JSON, with an updated-laws list) makes machine-readable diff-monitoring of legislation practical.
- **Digitization of the Official Gazette (官報)** and provision of a **base registry for public notices (告示)** are in progress, targeted for within fiscal 2026.
- **Online filing and web publication of political funds reports become mandatory from 2027-01**, advancing machine readability.
- EDGI's Web Monitoring, the prior art here, is currently dormant — leaving the recording / monitoring space relatively open.

## What it does

The S4RCIV pipeline reduces cleanly to three stages:

1. **Continuous collection** — fetch primary sources using HTTP GET only, against public endpoints.
2. **Recording change** — hash the fetched content and record the diff against the previous version into an append-only, immutable log (`ResourceObserved` / `ResourceChanged` / `ResourceVanished` / `ResourceRestored`).
3. **Visualization** — present it as timelines, entities, maps, and summaries, always with citation and freshness.

The scope of observation is limited to **institutions, public money, and public acts**:

- The legislative process (plenary and committee proceedings, bills, votes, recorded roll-call votes)
- Enactment and revision of laws, public notices, directives, and regulations
- Public money (budgets, political funds reports, party subsidies)
- Public procurement and contracts (tenders, awards, sole-source contracts)
- The **public** communications and filings of public officials and political organizations

## Design principles

These are constraints, not aspirations. Any feature or dependency that violates a principle is wrong by definition — surface the conflict instead of building it. See [`concepts/CORE_CONCEPT_0001.md`](concepts/CORE_CONCEPT_0001.md) for detail.

1. **Passive / read-only** — HTTP GET against public endpoints only. No auth, no submissions, no writes, no automated actions. A *sentinel*, never an actor.
2. **Public primary sources only** — only first-hand information that is already publicly available.
3. **Append-only, immutable log** — keep everything, including deletions and reversions. The log itself is hash-chained to be tamper-evident.
4. **Separation of observation and interpretation** — physically separate the tamper-evident ground truth (observation plane) from the recomputable, provenance- and confidence-bearing projections (interpretation plane).
5. **Standards-based, no silos** — Akoma Ntoso (laws / proceedings), Popolo (people / roles), OCDS (procurement).
6. **AI summarizes only, never judges** — no scoring or commentary; every summary links back to source text / diff, with confidence and provenance.
7. **Built-in source compliance** — per-source rate limiting, robots.txt, an identifying User-Agent, attribution and fetch timestamp on every record, and Internet Archive dual-sourcing where possible.

## What it does not do

S4RCIV's trustworthiness rests as much on what it **does not** do as on what it does. The authoritative list of prohibitions lives in [`../DISCIPLINE.md`](../DISCIPLINE.md). In summary:

- **Never monitor, profile, or expose private individuals.** The subject of observation is always an accountable public actor (politicians, parties, political organizations, public officials). Private individuals — such as small political-fund donors — are never cross-linked across records.
- **Never act partisanly.** No targeting of a particular party or ideology; the same criteria and the same pipeline apply to all. The criteria for selecting what to monitor are published.
- **Never present a decontextualized diff as a conclusion.** A diff is always shown with its surrounding context and a full-text link (to prevent "gotcha" framing).
- **Never let AI judge or evaluate.** Summarization and clustering only; no summary is emitted without a link to the source, a confidence level, and provenance.
- **Never auto-post to single out an individual.** Alerts are fact-based, citation-linked, and opt-in only.

## Architecture

A small set of self-hostable services (Go for collection and query, Rust for the structural diff, SvelteKit for the web, Postgres) combined adapter-style. Adding a new source equals adding a new adapter (collect + normalize). See concept document [§8](concepts/CORE_CONCEPT_0001.md) for detail.

```mermaid
flowchart TB
  SRC["Public APIs / pages (+ Internet Archive)"]
  subgraph OBS["Observation plane (immutable · append-only · hash-chain)"]
    COL["Source adapters / collection<br/>collector (Go)<br/>kokkai · e-Gov laws · Sangiin votes · member rosters"]
    LOG[("Event log, CQRS<br/>append-only · hash-chain<br/>ground truth")]
  end
  subgraph INT["Interpretation plane (recomputable + provenance / confidence)"]
    DIF["Structural diff — differ<br/>Rust · Connect-RPC · stateless"]
    RM[("Read models<br/>timeline · roll-call votes · laws · district vote map")]
  end
  API["api (Go · Connect-RPC · read-only)"]
  WEB["Web — SvelteKit situation-room dashboard"]
  SRC -->|HTTP GET only| COL --> LOG
  LOG -. projection .-> DIF --> RM
  LOG -. projection .-> RM
  RM --> API --> WEB
```

The structural diff is handled by a standalone **differ** service (Rust · Connect-RPC · stateless, ADR-000005); collection (`collector`) and query (`api`) are separate Go binaries. The **observation plane** is the immutable ground truth — raw snapshots plus hash-chained change events. The **interpretation plane** consists of normalized entities, change classification, and summaries; it is a projection that can be recomputed from the observation plane at any time, where every field carries provenance and confidence. Interpretation is never written back into the observation plane.

The UI conventions are specified in [`design/DESIGN_LANGUAGE.md`](design/DESIGN_LANGUAGE.md) (dark by default, targeting WCAG 2.2 AA, color used only to convey state).

## Verifiability

The observation-plane log is append-only. Each event carries the hash of the previous snapshot (`prev_content_hash`) and the log's own hash chain (`log_prev_hash`). This makes the log **tamper-evident** — not tamper-proof, but such that any tampering can be detected. A third party can independently verify that S4RCIV has not rewritten its own records after the fact.

Integrity verification is not a per-record "verified" badge; it runs **bounded**, in the browser, on a case page — recomputing only the segment from the most recent signed checkpoint (ADR-000014). For a project that calls itself a "record of the record," this verifiability is not a feature but a precondition. Where possible, content is also fetched via the Internet Archive (Memento), reinforcing the trail through a third-party archive.

## Sources

Each source is implemented as an adapter, run with per-source rate limiting on by default (the authoritative discipline lives in [`../DISCIPLINE.md`](../DISCIPLINE.md)).

| Source | Content | Endpoint | Status |
|---|---|---|---|
| National Diet Library Minutes Search API (国会会議録検索API) | Plenary / committee proceedings, speeches, recorded votes | `https://kokkai.ndl.go.jp/api/` | Implemented (M1) |
| e-Gov Laws API v2 (Digital Agency) | Statute XML (constitution, laws, cabinet/ministerial orders, etc.), updated-laws list | `https://laws.e-gov.go.jp/api/2/` | Implemented (M2) |
| House of Councillors roll-call votes (参議院 記名投票) | Per-member yea/nay on Sangiin plenary votes (the axis of the district map) | `https://www.sangiin.go.jp/` | Implemented (M4) |
| Official member rosters of both Houses (両院 公式議員名簿) | Sitting members' parliamentary group and district (Popolo identity, supports the vote map) | `https://www.shugiin.go.jp/` · `https://www.sangiin.go.jp/` | Implemented (M4) |
| Official Gazette / public-notice base registry (官報・告示) | Machine-readable structured data for public notices | Targeted within FY2026 | Future |
| Political funds reports (Ministry of Internal Affairs, 政治資金収支報告書) | Political-fund income and expenditure | Web publication mandated 2027-01 | Future |
| Public procurement (調達ポータル) | Tenders, awards, contracts (normalized to OCDS) | `https://www.p-portal.go.jp/` | Future |

> For the Diet minutes, copyright of the database and of speeches by NDL staff belongs to the NDL, so attribution is mandatory. Laws, public notices, and directives are "works not subject to rights" under Article 13 of the Copyright Act, giving collection, diff display, and redistribution a firm legal footing.

## Status and roadmap

**In development. M1 (Diet minutes) and M2 (e-Gov laws) collect, diff, project, and serve over the read-only query API.** M3 / M4 are partially implemented; M5 / M6 are not started. Public release (M6) has not been reached, and operational deployment instructions do not yet exist. For now the project runs as a local Docker Compose stack (see [Running locally](#running-locally)). The reasoning behind the design is recorded in [`ADR/`](ADR) (000001–000015).

Milestones (concept document §11; status: ✓ done / ◐ partial / ○ not started):

- **✓ M0 — Skeleton**: three-schema model (observation / interpretation / control), append-only + hash-chained event log, adapter interface, observation/interpretation plane separation.
- **✓ M1 — Legislative adapter**: fetch proceedings and speeches from the Diet minutes API; project members / parliamentary groups with Popolo; build `VoteEvent` from recorded votes.
- **✓ M2 — Laws adapter**: poll the e-Gov "updated laws" list; compute AKN structural diffs via a standalone differ service (down to articles, paragraphs, items, sub-items, and defined terms).
- **◐ M3 — Dashboard v0**: the cross-source timeline (bidirectional keyset pagination) and per-member roll-call votes are implemented. Watch & alerts are design-only (ADR-000007: no server push; feed plus device-local storage).
- **◐ M4 — Map**: a district choropleth of House of Councillors roll-call votes (per-prefecture breakdown + proportional panel + coverage) is implemented. The House of Representatives is blocked because per-member votes are not published, so the map pivots to the Sangiin (ADR-000010).
- **○ M5 — Summaries v0**: a thin summarization layer, with source links required. Not started.
- **○ M6 — Public release**: finalize licensing, self-hosting instructions, and publish the criteria for selecting monitored targets. Not started.

## Running locally

For now the project runs as a local Docker Compose stack (project name `s4rciv`). Prerequisites are Docker, plus Go if you want to run the tests.

```sh
cp .env.example .env            # set POSTGRES_* / USER_AGENT
# write the DB password into secrets/db_password.txt
docker compose up -d            # brings up db · migrate(Atlas) · api · collector · differ · web
# web: http://127.0.0.1:3000   api (Connect-RPC, read-only): 127.0.0.1:8080
cd services/api && go test ./...
```

The stack starts empty (the watch list only grows via `discover`). For seeding data (`collector discover` and other subcommands), proto regeneration, migrations, and container operations, see [`../CLAUDE.md`](../CLAUDE.md) and the `docker-compose` skill.

## Related work

S4RCIV favors cooperation over competition. By conforming to standards (AKN / Popolo / OCDS), it preserves the ability to connect with these projects.

- **Digital Democracy 2030 / Kouchou-AI** (participation & deliberation side, non-partisan OSS) — complementary; S4RCIV's structured records and diffs can feed deliberation as input context.
- **Code for Japan / Code for 選挙** (Popolo, legislative trackers) — cooperation through Popolo interoperability.
- **Seiji Shikin Center / political-finance-database** (public-money side) — S4RCIV complements them with time-series diffs and cross-source correlation.
- **mySociety** (TheyWorkForYou / WhatDoTheyKnow) / **EDGI Web Monitoring** — the overseas lineage and prior art.

## Contributing

Issues and Discussions are welcome. Before working on code or a collection adapter, please read [`concepts/CORE_CONCEPT_0001.md`](concepts/CORE_CONCEPT_0001.md) (the authoritative design) and [`../DISCIPLINE.md`](../DISCIPLINE.md) (the authoritative list of prohibitions). Proposals that conflict with the principles are treated as discussion, not implementation.

## License

The server body (this repository) is **[AGPL-3.0](../LICENSE)** (following the mySociety-style civic-tech convention of keeping SaaS forks open).

The data / schema license (CC0 or CC BY) and the client-library license (Apache-2.0 / MIT) are decided separately, once those artifacts exist (not covered by this repository's `LICENSE`).

## Acknowledgements

The design owes much to the following lineage: g0v / Audrey Tang (Plurality), EDGI Web Monitoring, mySociety, the AI Objectives Institute's Talk to the City, and the communities behind the Akoma Ntoso / Popolo / OCDS standards.

Primary sources and links:

- National Diet Library Minutes Search API spec — <https://kokkai.ndl.go.jp/api.html>
- e-Gov Laws API v2 — <https://laws.e-gov.go.jp/api/2/swagger-ui> / updated-laws list — <https://laws.e-gov.go.jp/update/>
- House of Councillors (roll-call votes / member roster) — <https://www.sangiin.go.jp/>
- Political funds reports (MIC) — <https://www.soumu.go.jp/senkyo/seiji_s/seijishikin/>
- Procurement portal — <https://www.p-portal.go.jp/>
- EDGI Web Monitoring — <https://envirodatagov.org/website-monitoring/>
- Talk to the City — <https://www.talktothe.city/>
- Digital Democracy 2030 — <https://dd2030.org/>
