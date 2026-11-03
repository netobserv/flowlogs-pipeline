# Agent Guide for flowlogs-pipeline

Task-oriented instructions for coding agents working in this repository.
Read `CONTRIBUTING.md` and `README.md` before starting.

## Who consumes FLP

Keep this in mind, it usually tells you what matters in a change:

- The main consumer is the [NetObserv operator](https://github.com/netobserv/network-observability-operator): it generates FLP's config and runs it. Features are designed with that integration in mind, not for hand-written configs.
- The main input is the [eBPF agent](https://github.com/netobserv/netobserv-ebpf-agent), sending protobuf flows over gRPC, directly or through Kafka. That ingest path, and everything downstream of it, is the hot path. Other inputs (IPFIX, stdin, file, synthetic) are secondary.
- Main outputs are Prometheus metrics and Loki logs.

## Core expectations

- Prefer minimal, surgical changes over broad refactors or speculative cleanup.
- Read the package you are editing and follow its existing naming, error handling, config patterns, tests and concurrency patterns — within reason. Where the existing code is poor, don't mimic it.
- Do not add per-file license headers. Apache v2 is always assumed (present at the repository root).
- Write comments only for intent, invariants and non-obvious constraints. Do not restate the code. Be concise.
- Be conservative on the hot path: avoid unnecessary allocations, copies, reflection, locking and high-cardinality metrics in per-flow code.
- Validate and bound anything coming from the network: flow records are attacker-influenced data.
- Keep dependencies minimal and justified.
- Never hand-edit `vendor/`. Change `go.mod` and run `make vendors`.

## Breaking changes

Config API backward compatibility is good to have, not mandatory. Deprecating is nicer than removing (see `api.CollectorType`). When a change does break, say so explicitly in the PR description so the release notes flag it, and expect a sister PR on the operator to follow.

## Layout

- `cmd/` — binaries: `flowlogs-pipeline`, `confgenerator`, plus the doc generators.
- `pkg/api/` — the config API: every YAML/JSON stage parameter type.
- `pkg/pipeline/{ingest,decode,transform,extract,encode,write}/` — the stages; `pipeline_builder.go` wires them.
- `pkg/confgen/`, `network_definitions/` — mostly unused legacy; don't wire new features there.
- `docs/` — mostly generated, see below.

## Config API changes

New or changed stage parameters in `pkg/api` need `yaml`, `json` and `doc` struct tags (see `pkg/api/write_ipfix.go`), then `make docs` to regenerate `docs/api.md`, `docs/metrics.md`, `docs/operational-metrics.md` and the README help blocks. Generated files are checked in; commit them with the change.

## Default workflow

1. Read the relevant package and its tests.
2. Add or update a test that captures the required behavior or regression.
3. Implement the smallest change that makes it pass.
4. Update docs and generated artifacts while the context is fresh.
5. Run `make build && make tests-unit` before considering the work complete.

## Verification

- `make build` — fmt, lint, compile, docs. This is the canonical check.
- `make tests-unit` — full unit tests (race detector, `-p 1`). `make tests-fast` during iteration.
- `make tests-e2e` — requires kind; only for changes touching end-to-end behavior.
- `make benchmarks` — for hot-path changes; see `docs/development.md` for profiling.
- Lint config is `.golangci.yml`. Its cyclop limit of 20 is a guideline: if satisfying it means splitting a function into artificial chunks, raise the limit a little instead.
- gocritic enforces `hugeParam` / `rangeValCopy`: pass large structs by pointer.

## Repository habits

- Prefer focused diffs. Avoid drive-by cleanup.
- Commit subjects are `NETOBSERV-1234: short description` when a Jira issue exists, otherwise a plain description.
- Follow existing stage and option patterns instead of inventing new abstractions.
- Exclude `vendor/` when searching the repository.

## Review tasks

- Lead with findings, ordered by severity, with file and line references.
- Focus on correctness, concurrency safety, resource growth on the hot path, operator-facing config impact, missing tests, and stale or missing generated docs.
- Call out when a diff is broader than necessary.
- If you find no issues, say so explicitly and note any residual risks.
