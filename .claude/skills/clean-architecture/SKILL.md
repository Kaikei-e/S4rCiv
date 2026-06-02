---
name: clean-architecture
description: Clean Architecture layer patterns (Handler → Usecase → Port → Gateway → Driver), mapped to s4rCiv's adapter model and observation/interpretation plane separation. Use when implementing or reviewing layered backend code (source adapters, normalizer, read models, CQRS).
---

# Clean Architecture Layers

```
Handler -> Usecase -> Port -> Gateway -> Driver
```

## s4rCiv mapping

These layers map onto s4rCiv's adapter-based, CQRS, dual-plane design:

- **Driver / Gateway** = the HTTP-GET boundary of a **source adapter** (the only place that touches an external public endpoint) plus the event-store / read-model persistence drivers. Keep collection (raw fetch) and the anti-corruption mapping to standard schemas (AKN / Popolo / OCDS) here.
- **Usecase** = normalization, diff/classification, and event emission into the append-only log (**observation plane**) — no direct external dependencies, only Port interfaces.
- **Read-model projectors** that build the **interpretation plane** are themselves Usecase-level orchestration over Port interfaces; they are recomputable and must not be a source of truth.
- A new source = a new adapter implementing the same Port contracts (the unit of extension). Keep adapters loosely coupled and versioned.

## Layer Rules

| Layer | Responsibility | Can Depend On |
|-------|----------------|---------------|
| Handler | HTTP/gRPC entry points, validation, response formatting | Usecase, Port |
| Usecase | Business logic orchestration, NO external dependencies | Port only |
| Port | Interface definitions (contracts) | Nothing |
| Gateway | Anti-corruption layer, external service mapping | Port, Driver |
| Driver | Database, API, external integrations | External libraries |

## File Patterns

- `**/rest/**` or `**/handler/**` = Handler layer
- `**/usecase/**` = Usecase layer
- `**/port/**` = Port layer (interfaces)
- `**/gateway/**` = Gateway layer
- `**/driver/**` = Driver layer

## Common Violations

- Handler importing Driver directly (must go through Usecase)
- Usecase importing external packages (use Port interfaces)
- Circular dependencies between layers
