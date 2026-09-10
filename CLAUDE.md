# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

`go-loghub-uuid` is a dependency-free Go library (package `loghubuuid`, module `github.com/patrickbrandao/go-loghub-uuid`) that generates **UUIDv7** (RFC 9562) with three levels of embedded temporal precision, plus every other RFC 9562 version (1, 2, 3, 4, 5, 6, 8). Only the standard library is used.

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

```bash
go test ./tests/ -race -run 'TestConcurrentUniqueness|TestMassConcurrent'  # detector de corrida
go test ./tests/ -run '^$' -fuzz FuzzFromString -fuzztime 60s              # fuzzing do parser
```

Note: tests live in `./tests/` and import the library by its **module path** (as an external consumer would), not as an internal package. Run `go test` against `./tests/`, not the repo root. Use `-short` to skip the 1M mass tests.

**Ordering has no monotonic counter.** Ordering is chronological *at the level's resolution*, with a **random** tie-break inside the same embedded instant. Generating a UUID is faster than most hosts' clock step, so consecutive UUIDs routinely tie (always, at Level1, whose resolution is the millisecond). Never write an ordering test that counts "regressions in a tight loop against a tolerated threshold" — that measures the host clock, not the library, and is why the old `TestMonotonicity` failed permanently on microsecond-clock hosts such as macOS. The clock-independent invariant lives in `TestOrderingFollowsEmbeddedTime`; `TestTieRateReport` reports the tie rate as a diagnostic; `TestMonotonicity` now sleeps between generations so the embedded instant genuinely advances.

## Architecture

**Production code lives only in the repo root**, alongside `go.mod`/`README`/`STARTHERE`/`LICENSE`. Everything else — `docs/`, `docs/SPEC.md`, `tests/` — is intentionally kept out of root so the production surface stays minimal. Preserve this separation: do not add non-production files to root.

Each source file is a distinct concern.

**UUIDv7 core — the hot path. Do not add cost here.**
- `uuid.go` — types (`Level`, `UUID [16]byte`, `Generator`, level constants), generation logic, package-level default generator.
- `conversion.go` — `String()` / `FromString()` and their aliases (`BinaryToString`, `StringToBinary`).
- `import.go` — `Time` struct and `Import` / `ImportBinary` (extract time fields back out of a UUID).

**Other UUID versions** — none of these are reached from the v7 path.
- `clock.go` — gregorian epoch constants, `GregorianTime`, and the mutex-guarded state (monotonic 100 ns clock, 14-bit clock sequence, 48-bit node) shared by v1/v2/v6.
- `version1.go` — `GenerateV1`, `GenerateV6`.
- `version2.go` — `Domain`, `GenerateV2` and the DCE shortcuts.
- `namebased.go` — namespace constants, `GenerateV3` (MD5), `GenerateV5` (SHA-1), `GenerateHash`.
- `version4.go` — `GenerateV4`, on the `Generator` and as a package function.
- `version8.go` — `GenerateV8` and its random variant.

**Support API.**
- `parse.go` — lenient `Parse`/`ParseBytes` (four formats, generic over `string`/`[]byte` to stay allocation-free), `Validate`, `FromBytes`, `MustParse`, `Must`, and the wrapped format errors.
- `values.go` — `Nil`, `Max`, `Compare`, `URN`, `UUIDs`, `VersionString`, `VariantString`.
- `encoding.go` — `encodeHex` plus the text and binary marshalers.
- `sql.go` — `Scan`, `Value`, `NullUUID`.
- `inspect.go` — version-aware `Timestamp`, `TimestampWithLevel`, `GregorianTime`, `ClockSequence`, `NodeID`, `Domain`, `ID`.
- `entropy.go` — `NewGeneratorWithReader`, `NewCryptoGenerator`, `ErrEntropySource`.
- `compat.go` — aliases matching `github.com/google/uuid` names and signatures. See `docs/MIGRATION.md`.

**Two invariants worth stating explicitly.** First, `String()` and `encodeHex` duplicate the same eight lines on purpose: `String()` is the hot path and must not pay a call. Second, `FromString` still returns the bare `ErrInvalidFormat` sentinel, so `err == ErrInvalidFormat` keeps working for existing callers; only `Parse` returns the wrapped, more specific errors.

### The three levels (core concept)

A UUIDv7 carries a 48-bit millisecond timestamp in its top bytes; the lower bits (`rand_a` = 12 bits, `rand_b` = 62 bits) are normally random. This library optionally overwrites those random bits with sub-millisecond precision while **always preserving version (7) and variant (RFC `0b10`)**, so output is always a valid UUIDv7:

| Level    | `rand_a` (12 bits) | top 10 bits of `rand_b` | rest of `rand_b` |
|----------|--------------------|-------------------------|------------------|
| `Level1` | random             | random                  | random           |
| `Level2` | microseconds 0–999 | random                  | random           |
| `Level3` | microseconds 0–999 | nanoseconds 0–999       | random (52 bits) |

Because the precision bits sit immediately after the milliseconds, **lexicographic string order stays chronological** across all levels (subject to the tie-break caveat above). Unknown `Level` values fall back to `Level1`. The exact byte layout is documented in the `Generate` doc comment at [uuid.go:148](uuid.go:148) and in [STARTHERE.md](STARTHERE.md) §4 — keep these two in sync if the bit layout ever changes.

**Entropy draws per level.** Level2/Level3 put the microseconds in `rand_a`, so they consume **one** 64-bit word; only Level1 (and unknown levels, which behave as Level1) consumes **two**. `tests/robustness_test.go` locks this in — it matters for callers who supply `crypto/rand` through `NewGeneratorWith`.

**Clock drift in v1/v2/v6.** The shared clock advances one 100 ns tick per generation, so a sustained burst pushes the embedded instant ahead of the wall clock. RFC 9562 allows this. It deliberately differs from `google/uuid`, which keeps the wall-clock instant and increments the 14-bit clock sequence instead: no drift there, but no strict ordering inside one tick either. `TestTimeBasedClockDrift` documents and bounds it; `TestTimestampRoundTrip` resets the accumulated drift by changing the clock sequence, which zeroes the last-time floor.

**Adding `MarshalText` changed the JSON wire format.** A `UUID` used to serialize as a 16-number array; it now serializes as the canonical string. Same for `gob`. The API is additive, the stored data is not.

`ImportBinary` is deliberately **level-blind**: it always reads `rand_a` as microseconds and the top 10 bits of `rand_b` as nanoseconds, treating random bits as if they were precise time (per spec). It cannot know which level produced a UUID.

### Concurrency / performance design

`Generator` is built once at boot and shared across goroutines. `NewGenerator()` backs entropy with a `sync.Pool` of per-thread PCG PRNGs (`math/rand/v2`), each seeded once from `crypto/rand` — no shared lock, no global contention. `NewGeneratorWith(source)` lets callers swap in custom entropy (e.g. full `crypto/rand`); the supplied function **must be concurrency-safe**. Package-level `Generate`/`GenerateString` use an internal default `Generator`.

Hot paths avoid allocations: binary generation does zero allocs; `String()` writes into a fixed `[36]byte` buffer. When modifying generation or conversion, preserve the zero/low-allocation property — `benchmark_test.go` and `-benchmem` are the guardrails.

## Reference docs

- [STARTHERE.md](STARTHERE.md) — full project map and public API listing.
- [docs/SPEC.md](docs/SPEC.md) — language-agnostic specification sufficient to reimplement the entire library from scratch across all supported UUID versions (1 to 8, parsing, concurrency, and serialization).
- [docs/MIGRATION.md](docs/MIGRATION.md) — moving from `github.com/google/uuid`; lists what was intentionally not imported (`SetRand`, the rand pool, `SetNodeInterface`) and why.
- [docs/](docs/) — quick use, full use, testing/benchmark guides.
