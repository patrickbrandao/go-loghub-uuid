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
go test ./ -list Example                                  # list the runnable examples (example_test.go)
golangci-lint run ./...                                   # linter, same config as CI (.golangci.yml, needs v2)
```

```bash
go test ./tests/ ./ -short -coverpkg=github.com/patrickbrandao/go-loghub-uuid -coverprofile=cover.out   # cobertura
go tool cover -func=cover.out                                                                            # por função
```

```bash
go test ./... -race -short                                       # detector de corrida (suíte inteira)
go test ./tests/ -short -run 'Allocations|SingleAllocation' -v   # travas de alocação (sem -race)
go test ./tests/ -run '^$' -fuzz FuzzFromString -fuzztime 60s    # fuzzing do parser estrito
go test ./tests/ -run '^$' -fuzz FuzzParse -fuzztime 60s         # fuzzing do parser permissivo
go test ./tests/ -run '^$' -fuzz FuzzNullUUIDJSON -fuzztime 60s  # fuzzing do JSON de NullUUID
```

Note: tests live in `./tests/` and import the library by its **module path** (as an external consumer would), not as an internal package. Run `go test` against `./tests/`, not the repo root, except for the two root test files: `clock_internal_test.go` covers pure clock functions and the sequence-floor state machine, which cannot be driven from outside the package; `example_test.go` (package `loghubuuid_test`) holds the `Example` functions, which godoc only associates with the package when they live in its directory. Examples with `// Output:` use only fixed vectors, never the clock or the PRNG. Use `-short` to skip the 1M mass tests.

**Linter.** `.golangci.yml` (v2 format) enables `errcheck`, `govet`, `staticcheck`, `unused`, `ineffassign`, `gosec`, `errorlint`, `revive` (exported-comment rule) and `nolintlint`; `misspell` is off because the comments are Portuguese. `gosec` G115 (integer truncation) is excluded globally: byte packing by shift-and-truncate is the whole library, and the hot path must not gain masks to silence it. Every `//nolint` needs a linter name and a reason. Do not touch `uuid.go`/`conversion.go`/`import.go` to satisfy a lint finding without benchmarking before and after.

**Coverage.** Measured with `-coverpkg` because the suite is a separate package; CI (stable only) prints `go tool cover -func`, uploads the HTML as an artifact and fails below 95% (`COVERAGE_MIN` in `ci.yml`). Only the `crypto/rand` failure branches (`strongSeed`, `fillRandom`, the `NewRandom`/`NewV7` recover paths), unreachable on Go 1.24+, remain uncovered by design; the rest is secondary parser error branches that fuzzing exercises.

**Allocation locks run under `-race` too.** Until `v0.3.0` three of them skipped themselves under the detector, because the default generator's `sync.Pool` deliberately drops one in four items on `Put` when built with `-race` and `AllocsPerRun` counted the rebuilt PRNG. The pool is gone (the default generator now reads `math/rand/v2`'s runtime source), so the skip, its `skipIfRaceDetector` helper and the `race_enabled_test.go` / `race_disabled_test.go` build-tag pair were removed. CI still measures the locks in a separate step without the detector, since that is the reference measurement. Do not "fix" a future failure here by relaxing the locks.

**CI.** `.github/workflows/ci.yml` has three jobs. `test` (Linux, Go 1.22 and stable) runs gofmt, vet, build, cross-compile plus vet for `windows/amd64`, `darwin/arm64` and `linux/arm64`, golangci-lint, `-race -short`, the allocation locks, a short `-benchmem` benchmark and the coverage report (the last three tool-dependent steps on stable only). `test-os` runs build, vet and `go test ./... -short` on `windows-latest` and `macos-latest`, but only on pull requests, tags, the weekly schedule and manual dispatch, to save the pricier runners. `deep` (weekly and on dispatch) runs the full suite and 60 s of fuzzing on each of the three targets with `continue-on-error`, uploads `tests/testdata/fuzz/` as the `fuzz-corpus` artifact whenever a campaign fails, and fails the job afterwards; reproduction steps are in `docs/TEST-AND-BENCHMARK.md`. Never tag a release without a green `test` job on that commit; the procedure is in `docs/RELEASE.md`. Keep `go.mod` at `go 1.22` unless a newer API is genuinely needed; CI on 1.22 is what enforces that.

**Ordering has no monotonic counter.** Ordering is chronological *at the level's resolution*, with a **random** tie-break inside the same embedded instant. Generating a UUID is faster than most hosts' clock step, so consecutive UUIDs routinely tie (always, at Level1, whose resolution is the millisecond). Never write an ordering test that counts "regressions in a tight loop against a tolerated threshold" — that measures the host clock, not the library, and is why the old `TestMonotonicity` failed permanently on microsecond-clock hosts such as macOS. The clock-independent invariant lives in `TestOrderingFollowsEmbeddedTime`; `TestTieRateReport` reports the tie rate as a diagnostic; `TestMonotonicity` now sleeps between generations so the embedded instant genuinely advances.

## Architecture

**Production code lives only in the repo root**, alongside `go.mod`, `README.md`, `STARTHERE.md`, `CHANGELOG.md`, `LICENSE`, `SECURITY.md` and `CONTRIBUTING.md` (GitHub reads both from root), this file, the two root test files `clock_internal_test.go` and `example_test.go`, the tool configuration `.golangci.yml`, and `.github/` (which GitHub requires at root). Everything else — `docs/`, `docs/SPEC.md`, `tests/` — is intentionally kept out of root so the production surface stays minimal. Preserve this separation: do not add other non-production files to root. `CHANGELOG.md` is the project history; every behavior change gets an entry under "Não publicado" with the file it touched.

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

Because the precision bits sit immediately after the milliseconds, **lexicographic string order stays chronological** across all levels (subject to the tie-break caveat above). Unknown `Level` values fall back to `Level1`. The exact byte layout is documented in the `Generate` doc comment at [uuid.go:154](uuid.go:154), in [STARTHERE.md](STARTHERE.md) §4 and in `docs/SPEC.md` §3.1 — keep the three in sync if the bit layout ever changes.

**Entropy draws per level.** Level2/Level3 put the microseconds in `rand_a`, so they consume **one** 64-bit word; only Level1 (and unknown levels, which behave as Level1) consumes **two**. `tests/robustness_test.go` locks this in — it matters for callers who supply `crypto/rand` through `NewGeneratorWith`.

**Clock drift in v1/v2/v6.** The shared clock advances one 100 ns tick per generation, so a sustained burst pushes the embedded instant ahead of the wall clock. RFC 9562 allows this. It deliberately differs from `google/uuid`, which keeps the wall-clock instant and increments the 14-bit clock sequence instead: no drift there, but no strict ordering inside one tick either. `TestTimeBasedClockDrift` documents and bounds it.

**Clock floor is per clock sequence.** `clock.go` keeps `seqLastTime`, a map from each clock sequence ever used in the process to the last instant emitted with it. Switching sequences saves the outgoing floor and adopts the incoming one (zero for a never-used sequence), so for every sequence the emitted instants are strictly increasing for the life of the process and no (instant, sequence) pair ever repeats — that is the uniqueness guarantee for v1/v6, independent of node changes. `SetClockSequence(-1)` always draws an unused sequence, which is the only way to discard accumulated drift (tests use it to resynchronize with the wall clock). This deliberately differs from `google/uuid`, which zeroes the floor on any sequence change and can repeat a v1 UUID when returning to an old sequence. Locked in by `TestSequenceFloorSurvivesRoundTrip` (internal, simulates drift directly) and `TestClockSequenceReuseNeverRepeats`. The map is only touched on sequence changes, never per generation.

**`Scan` treats empty text as absent.** `""` and `[]byte{}` set `Nil` without error (and `Valid=false` on `NullUUID`), matching `google/uuid`; migrated code reading `DEFAULT ''` columns depends on it. `IsInvalidLengthError` uses `errors.Is`. `NullUUID.UnmarshalJSON` delegates to `encoding/json` only when the string contains a backslash; the no-escape path stays allocation-free.

**Adding `MarshalText` changed the JSON wire format.** A `UUID` used to serialize as a 16-number array; it now serializes as the canonical string. Same for `gob`. The API is additive, the stored data is not.

`ImportBinary` is deliberately **level-blind**: it always reads `rand_a` as microseconds and the top 10 bits of `rand_b` as nanoseconds, treating random bits as if they were precise time (per spec). It cannot know which level produced a UUID.

### Concurrency / performance design

`Generator` is built once at boot and shared across goroutines. `NewGenerator()` draws entropy from `math/rand/v2`'s package functions, which since Go 1.22 read the runtime's generator: one ChaCha8 instance per thread, seeded by the OS — no shared lock, no pool, no state for the library to keep. `NewGeneratorWith(source)` lets callers swap in custom entropy (e.g. full `crypto/rand`); the supplied function **must be concurrency-safe**. Package-level `Generate`/`GenerateString` use an internal default `Generator`.

Hot paths avoid allocations: binary generation does zero allocs; `String()` writes into a fixed `[36]byte` buffer. When modifying generation or conversion, preserve the zero/low-allocation property — `benchmark_test.go` and `-benchmem` are the guardrails.

## Reference docs

- [STARTHERE.md](STARTHERE.md) — full project map and public API listing.
- [CHANGELOG.md](CHANGELOG.md) — history per version and rejected proposals with reasons. Check it before re-proposing any of those.
- [docs/SPEC.md](docs/SPEC.md) — language-agnostic specification sufficient to reimplement the entire library from scratch across all supported UUID versions (1 to 8, parsing, concurrency, and serialization).
- [docs/MIGRATION.md](docs/MIGRATION.md) — moving from `github.com/google/uuid`; lists what was intentionally not imported (`SetRand`, the rand pool, `SetNodeInterface`) and why.
- [docs/RELEASE.md](docs/RELEASE.md) — release procedure: prerequisites, tag and `gh release`, post-publication check, and why tags are never moved.
- [SECURITY.md](SECURITY.md) — private vulnerability reporting and the documented threat model; [CONTRIBUTING.md](CONTRIBUTING.md) — the subset of these rules that applies to external contributors.
- [docs/](docs/) — quick use, full use, testing/benchmark guides.
