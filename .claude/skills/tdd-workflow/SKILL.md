---
name: tdd-workflow
description: Test-Driven Development workflow for all languages. Use when implementing new features, fixing bugs, or refactoring code, or when the user says "TDDで". Enforces outside-in order E2E → contract (CDC, when a boundary is crossed) → Unit (RED-GREEN-REFACTOR) with concrete stubs, and a local CI parity sweep (format/lint/type/test) before handoff.
allowed-tools: Bash, Read, Glob, Grep, Edit, Write
argument-hint: <feature-description> [--component=<dir>]
---

# TDD Workflow

Test-Driven Development workflow. Use in both Plan mode and implementation mode
whenever the task may change code or tests.

**Outside-in order for feature work: E2E → CDC → Unit.** This governs the *order of
writing tests*. The test pyramid still governs *quantity*: few E2E, more contract,
many unit tests ([Fowler — Practical Test Pyramid](https://martinfowler.com/articles/practical-test-pyramid.html)).
Two different axes — order vs. quantity — both apply.

**Finish every task with Phase 5 — local CI parity.** After Phases 0–4 are green, run
the same formatters / linters / type checkers / tests the touched component's CI runs,
locally, before declaring the work complete. "Tests pass" ≠ "CI will pass".

Three layers:

- **E2E (outermost)** — *"does the user journey / cross-component flow work?"*
  Browser journeys → **Playwright**; HTTP / API scenarios → an HTTP-level E2E
  (Playwright `APIRequest`, Hurl, or `reqwest`/`httpx`-driven test).
- **CDC (only when a change crosses a component boundary)** — *"do consumer and provider
  agree on the wire shape?"* → a contract test (e.g. Pact) at each crossed boundary.
- **Unit** — *"does each component work?"* → per-layer tests (Handler / Usecase / Gateway / Driver).

For a pure refactor inside one component's inner layers (no UI, no boundary change),
skip Phase 0 and Phase 1 and jump to Phase 2.

> s4rCiv の主構成は Rust/Go の source adapter・normalizer・event store、SvelteKit/TS の
> dashboard、Python の LLM 要約パイプライン。サービス名・boundary はまだ確定していないので、
> 以下は `<component>` 等のプレースホルダで書く。実体が決まったら具体名に差し替える。

## Phase 0: E2E FIRST

**Goal:** Write the outermost failing test expressing the user-visible / cross-component
behavior the change must deliver — the acceptance test that drives everything else.

### Decision tree

- Touches browser UI (Svelte component, page, user flow) → **Playwright**
- Touches an HTTP endpoint / API / component-to-component flow → **HTTP-level E2E**
- Full-stack (FE calls a new BE endpoint) → **both**
- Pure inner-layer refactor with no external behavior change → skip Phase 0 (go to Phase 2)

### Playwright (browser E2E) best practices

(from [playwright.dev best practices](https://playwright.dev/docs/best-practices))

- **Locators**: `getByRole` / `getByLabel` / `getByText` / `getByTestId` — avoid CSS / XPath
- **Web-first async assertions**: `await expect(locator).toBeVisible()` — never `expect(await locator.isVisible()).toBe(true)`
- Trust auto-waiting — no manual `waitForTimeout` or retry loops
- One `test()` = one user journey; fresh browser context per test
- Mock third-party deps (e.g. via MSW); seed sessions/fixtures — don't rely on "whatever is in the DB"

### HTTP-level E2E best practices

- **Parameterize** hosts / tokens — never hardcode `http://localhost:...`
- **Health-gate** before exercising business endpoints (retry until ready)
- **Assertions**: status/headers first, then explicit body assertions (`jsonpath`/`xpath`, `contains`, `matches /regex/`, `isUuid`, `isIsoDate`)
- DB-backed scenarios that depend on FK / sequence ordering must run serially

### Steps

1. **Detect scope** with the decision tree (UI / API / both / skip).
2. **Write the failing E2E first** (new spec/scenario, following a neighboring file as template).
3. **Run it** — confirm RED for the *right reason* (missing behavior), not the wrong reason
   (404 / connection refused from a missing route stub, syntax error, service not up).
4. **Commit the failing E2E on its own**:
   ```bash
   git commit -m "test(e2e): add failing <feature> scenario"
   ```
5. **Proceed**: boundary crossed → Phase 1 (CDC); otherwise → Phase 2 (Unit RED).

## Phase 1: CDC CONTRACT CHECK (only if a boundary is crossed)

**Goal:** If the change touches a component boundary, add/update a contract test so every
crossed boundary has a contract. Run after Phase 0's E2E is RED.

### Detect if a boundary is crossed

- Modifies a request/response format between components?
- Adds/modifies an API endpoint consumed by another component?
- Introduces or changes a **required header / auth requirement** (optional → required, etc.)?
- Modifies a shared schema / proto?

### If a boundary is touched

1. **Consumer side first** — write/update the consumer contract test in the calling component.
2. **Run consumer test** → generates the contract artifact.
3. **Provider side** — run provider verification against the contract.
4. **Provider-adds-requirement rule** — if a provider tightens what consumers must send
   (new required header/field, stricter auth), the consumer-driven contract pipeline only
   protects you if **every** consumer has a verified contract. Enumerate all consumers,
   confirm each has a contract that pins the new requirement, and have the provider verify
   the union. Do not ship the tightening until that gate is green.

If no boundary is crossed, skip to Phase 2.

## Phase 2: RED (Write Failing Unit Test)

**Goal:** Define expected behavior through unit tests BEFORE implementation. For feature
work, enter only after Phase 0 (and Phase 1 if a boundary is crossed) are RED.

### Steps

1. **Detect language & component** — `go.mod`, `pyproject.toml`, `Cargo.toml`, `package.json`.
   Identify the Clean Architecture layer (see `clean-architecture` skill).
2. **Write the test** — define expected behavior (success / error / edge cases), not file or
   symbol existence.
3. **Create the implementation stub first** so the test fails for the right reason, not a
   missing symbol:
   - Go: `panic("not implemented")`
   - Python: `raise NotImplementedError`
   - Rust: `unimplemented!()`
   - TypeScript: `throw new Error("not implemented")`
4. **Verify the test fails for the RIGHT reason** (not syntax/import errors). If it passes
   without implementation, rewrite it.
5. **Commit the failing test on its own** (CLAUDE.md: RED と GREEN は別 commit):
   ```bash
   git commit -m "test(<component>): add failing tests for <feature>"
   ```

## Phase 3: GREEN (Minimal Implementation)

**Goal:** Write ONLY enough code to pass the tests.

- Write minimal code; do **not** modify tests to make them pass; do **not** add features
  not covered by tests.
- All tests must pass before proceeding.
- Check layer violations (Handler → Usecase → Port; Usecase → Port only; Gateway → Port, Driver).

## Phase 4: REFACTOR (Clean Up)

**Goal:** Improve code quality while keeping tests green.

- Remove duplication, improve naming, simplify logic — run tests after each change.
- If Phase 1 detected a boundary change, re-run the contract tests (consumer + provider).
- **Final commit** (separate from the RED commit):
  ```bash
  git commit -m "feat(<component>): implement <feature>"
  ```

## Phase 5: LOCAL CI PARITY (MANDATORY before handoff)

**Goal:** Reproduce locally the gates each touched component's CI would run, as the last
step before reporting complete. Phases 0–4 guarantee tests pass; Phase 5 guarantees
**formatters, linters, type checkers, and security scanners** also pass.

Skipping this is the most common cause of "green locally, red in CI" — a stray unused
import, format drift, or a lint rule that only runs in CI.

### Steps

1. **Enumerate every component directory touched** (`git diff --name-only` against the branch point).
2. **For each touched component**, run the language gate below. All must pass before handoff.
3. **Never suppress a failing gate** (no `// nolint`, no loosening config, no skipping a test)
   to unblock — fix the underlying issue or escalate.

### Per-language CI parity commands

**Go**
```bash
cd <component>
gofmt -l . | grep -v '^$'        # must print nothing
go vet ./...
golangci-lint run ./...          # if installed
go test ./... -race
```

**Rust**
```bash
cd <component>
cargo fmt --all -- --check
cargo clippy --all-targets --all-features -- -D warnings
cargo build --release
cargo test --all
```

**Python** (型チェッカは Pyrefly。mypy は使わない)
```bash
cd <component>
uv sync --all-extras --dev
uv run ruff check .
uv run ruff format --check .
uv run pyrefly check
uv run pytest
```

**TypeScript / Svelte** (`web/`, pnpm)
```bash
cd web
pnpm install --frozen-lockfile
pnpm run check                    # svelte-check + tsc (the script that exists today)
pnpm run lint                     # if a linter is configured (Biome / ESLint)
pnpm test                         # if a test runner is configured (Vitest)
pnpm run test:e2e                 # Playwright, if UI touched (Phase 0 scope)
```

### Reporting

At handoff, state explicitly which components' CI-parity gates you ran and their exit
status. If a gate is skipped, say so explicitly (e.g. "skipped golangci-lint — not
installed, rely on CI"). Do **not** silently skip.

## Test File Conventions

| Language | Unit Test | Contract Test (CDC) |
|----------|-----------|---------------------|
| Go | `*_test.go` in same package | `driver/contract/*_test.go` |
| Python | `tests/test_*.py` | `tests/contract/test_*.py` |
| Rust | `#[cfg(test)]` module or `tests/*.rs` | `tests/contract/*.rs` |
| TypeScript | `*.test.ts` / `*.spec.ts` | `src/test/contracts/*.test.ts` |

Per-language test stub templates live in `templates/` (go / rust / python / typescript).

## Clean Architecture Integration

1. Identify the target layer (Handler, Usecase, Gateway, Driver) — see the `clean-architecture` skill.
2. Mock dependencies from outer layers (use fakes in usecase tests, mocks at the driver layer).
3. Test only the layer's responsibility.

## Anti-Patterns (AVOID)

1. Writing implementation before tests
2. Modifying tests to make them pass
3. Adding features not covered by tests
4. Skipping error / edge case tests
5. Testing implementation details instead of behavior
6. Changing a component API without updating its CDC consumer tests
7. Writing unit tests that only fail because a file or function does not exist yet
8. Using RED to validate missing symbols instead of behavior through a concrete stub
9. Tightening a provider's requirements without updating every consumer's contract — see Phase 1
10. Treating auth / required-header changes as "infra" and skipping Phase 0 — they change the request contract
11. `expect(await locator.isVisible()).toBe(true)` in Playwright — use `await expect(locator).toBeVisible()`
12. CSS / XPath selectors in Playwright when `getByRole` / `getByTestId` work
13. Hardcoding `http://localhost:...` in HTTP-level E2E — parameterize the host
14. Writing unit tests first and backfilling E2E at the end — violates outside-in order
15. Declaring work complete without running Phase 5 (local CI parity)
16. Suppressing a Phase 5 failure (disabling a lint rule, loosening config, skipping a test) to green the gate

## References

- Martin Fowler: The Practical Test Pyramid — https://martinfowler.com/articles/practical-test-pyramid.html
- Martin Fowler: TestPyramid bliki — https://martinfowler.com/bliki/TestPyramid.html
- Playwright: Best Practices — https://playwright.dev/docs/best-practices
- Playwright: Continuous Integration — https://playwright.dev/docs/ci
- Pact: Contract Tests vs Functional Tests — https://docs.pact.io/consumer/contract_tests_not_functional_tests
- Pact: Handling authentication and authorization — https://docs.pact.io/provider/handling_auth
- Pact: Can I Deploy — https://docs.pact.io/pact_broker/can_i_deploy
