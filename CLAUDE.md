# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

`go-loghub-uuid` is a dependency-free Go library (package `loghubuuid`, module `github.com/patrickbrandao/go-loghub-uuid`) that generates **UUIDv7** (RFC 9562) with three levels of embedded temporal precision. Only the standard library is used.

**Language split (enforce when editing):** code *identifiers* (types, functions, variables, constants, package/file names) are in **English**; *comments*, the documentation under `docs/`, and the spec are in **Brazilian Portuguese**; human-facing *string literals* (error messages, console/report labels) are also Portuguese. No emojis anywhere — formal text only. So a typical function has an English name and a Portuguese doc comment; keep that pattern.

## Commands

```bash
go build ./...                                            # compile library
go vet ./...                                              # static analysis
go test ./tests/ -v                                       # functional tests
go test ./tests/ -run TestRoundTripString                # single test by name
go test ./tests/ -run '^$' -bench Benchmark -benchmem     # benchmarks (no tests)
go run ./tests/benchmark-bulk                             # generate 1,000,000 per level
```

Note: tests live in `./tests/` and import the library by its **module path** (as an external consumer would), not as an internal package. Run `go test` against `./tests/`, not the repo root.

`TestMonotonicity` depends on a **nanosecond-resolution wall clock** and fails on hosts whose `time.Now()` is only microsecond-resolution (e.g. macOS, where the embedded nanosecond digit is always 0 and random low bits then break ordering ties). This is environmental, not a regression — it passes on a nanosecond-clock host (such as the Linux VM in the docs). Use `-short` to skip the 1M mass tests.

## Architecture

**Production code lives only in the repo root** (`uuid.go`, `conversion.go`, `import.go`, plus `go.mod`/`README`/`STARTHERE`/`LICENSE`). Everything else — `docs/`, `docs/SPEC.md`, `tests/` — is intentionally kept out of root so the production surface stays minimal. Preserve this separation: do not add non-production files to root.

Three source files, each a distinct concern:
- `uuid.go` — types (`Level`, `UUID [16]byte`, `Generator`, level constants), generation logic, package-level default generator.
- `conversion.go` — `String()` / `FromString()` and their aliases (`BinaryToString`, `StringToBinary`).
- `import.go` — `Time` struct and `Import` / `ImportBinary` (extract time fields back out of a UUID).

### The three levels (core concept)

A UUIDv7 carries a 48-bit millisecond timestamp in its top bytes; the lower bits (`rand_a` = 12 bits, `rand_b` = 62 bits) are normally random. This library optionally overwrites those random bits with sub-millisecond precision while **always preserving version (7) and variant (RFC `0b10`)**, so output is always a valid UUIDv7:

| Level    | `rand_a` (12 bits) | top 10 bits of `rand_b` | rest of `rand_b` |
|----------|--------------------|-------------------------|------------------|
| `Level1` | random             | random                  | random           |
| `Level2` | microseconds 0–999 | random                  | random           |
| `Level3` | microseconds 0–999 | nanoseconds 0–999       | random (52 bits) |

Because the precision bits sit immediately after the milliseconds, **lexicographic string order stays chronological** across all levels. Unknown `Level` values fall back to `Level1`. The exact byte layout is documented in the `Generate` doc comment at [uuid.go:119](uuid.go:119) and in [STARTHERE.md](STARTHERE.md) §4 — keep these two in sync if the bit layout ever changes.

`ImportBinary` is deliberately **level-blind**: it always reads `rand_a` as microseconds and the top 10 bits of `rand_b` as nanoseconds, treating random bits as if they were precise time (per spec). It cannot know which level produced a UUID.

### Concurrency / performance design

`Generator` is built once at boot and shared across goroutines. `NewGenerator()` backs entropy with a `sync.Pool` of per-thread PCG PRNGs (`math/rand/v2`), each seeded once from `crypto/rand` — no shared lock, no global contention. `NewGeneratorWith(source)` lets callers swap in custom entropy (e.g. full `crypto/rand`); the supplied function **must be concurrency-safe**. Package-level `Generate`/`GenerateString` use an internal default `Generator`.

Hot paths avoid allocations: binary generation does zero allocs; `String()` writes into a fixed `[36]byte` buffer. When modifying generation or conversion, preserve the zero/low-allocation property — `benchmark_test.go` and `-benchmem` are the guardrails.

## Reference docs

- [STARTHERE.md](STARTHERE.md) — full project map and public API listing.
- [docs/SPEC.md](docs/SPEC.md) — language-agnostic spec for reimplementing from scratch; update it if behavior/layout changes.
- [docs/](docs/) — quick use, full use, testing/benchmark guides.
