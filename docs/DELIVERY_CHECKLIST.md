# Delivery Checklist

Local implementation and constructor update completed 2026-10-12 on branch
`alpha-0.1`. The constructor update began with a clean Git status and modified
the existing SDK. Evidence uses Go 1.27.1 on linux/amd64 and fabricated inputs/local
mocks only. Historical baseline evidence is labeled separately below.

## Implementation

- [x] Inspect existing module/API/license/instructions and preserve unrelated work.
  Discovery Git status was clean; the subsequently appearing `go.mod` was preserved.
- [x] Module `github.com/barlus-developer/golang-sdk-telbiz`, package `telbiz`.
- [x] Lazy `New(config ...Config) *Client` and eager `NewWithContext(ctx, config)`.
- [x] Environment credentials captured only by zero-config `New`; explicit config
  never falls back. Deferred inspectable config/auth errors; invalid arity rejected.
- [x] Configured per-request timeout for auth, renewal, SMS/top-up; zero retains
  30 seconds, negatives invalid; earlier deadlines and injected client bounds win.
- [x] Single shared first-use auth independent of caller cancellation; start idle
  renewal only after first success. Safe Close before/during use and startup races.
- [x] Automatic idle renewal at 60 seconds before normal expiry.
- [x] Short lifetime clamp, single shared auth, bounded backoff, concurrent Close.
- [x] Typed SMS/top-up schemas, input validation, accepted results, safe errors.
- [x] Parse flat/nested envelopes, preserve unknown codes and transaction keys.
- [x] No automatic service POST replay, including redirects and 401 errors.
- [x] No unsupported provider capabilities invented.

## Documentation and repository

- [x] README installation instructions and complete quick start match implemented signatures.
- [x] Usage, authentication, API, errors, testing, releasing, implementation notes complete.
- [x] Package documentation and lifecycle/service Mermaid flows complete.
- [x] AGENTS, CONTRIBUTING, SECURITY, changelog reflect the implementation.
- [x] Existing MIT license preserved unchanged.
- [x] Gitignore and empty env example added; existing PR/issue templates preserved.
- [x] Four runnable examples compile and identify billable effects.
- [x] CI configured for formatting, tests, race detector, vet, example compilation.
- [x] Relative Markdown paths resolve; new sources use only environment credentials
  or explicit fabricated test values. No real credentials were used or added.

## Acceptance

- [x] Required test-plan scenarios covered by local mocks and deterministic fake time.
  See the named-test mapping in [TESTING.md](TESTING.md).
- [x] Formatting, unit/mock tests, race detector, vet, and examples build pass.
- [x] Go-version claims match the preserved module and tested Go 1.27.1 toolchain.
- [x] Review source/diff for scope, credential forwarding, safe errors, cleanup,
  token installation/admission expiry, service replay, and concurrent shutdown.
- [x] Final evidence distinguishes SDK validation from provider unknowns.
- [x] No remote push, Git commit/tag, or release publication performed.

## Constructor update: exact verification evidence

These checks ran on the final Go source revision for this constructor update.
Mock tests passed outside the sandbox; no example program was executed.

| Command/check | Result | Evidence |
|---|---|---|
| `go version` | Passed | `go version go1.27.1 linux/amd64`; module unchanged |
| `gofmt -l .` | Passed | No output, including new `constructor_test.go` |
| `go test ./...` | Passed | Root `ok` (final invocation cached); four example packages compile; earlier uncached run also passed |
| `go test -race ./...` | Passed | Root `ok` in 1.097s; no race reports; example packages compile |
| `go vet ./...` | Passed | Exit 0, no output |
| `go build ./examples/...` | Passed | Exit 0; all four programs compiled only |
| `go build -o /tmp/telbiz-constructor-readme-check /tmp/telbiz-constructor-readme-check.go` | Passed | Complete updated README program extracted and compiled only |
| `git diff --check` | Passed | Exit 0, no whitespace errors |
| `go doc New`, `go doc NewWithContext`, `go doc Config` | Passed | Signatures/defaults match public API docs |
| Relative Markdown path review with Python `pathlib` | Passed | 60 relative path targets resolve; heading anchors excluded in this update's check |
| `git diff -- LICENSE docs/TELBIZ_API_REFERENCE.md go.mod` | Passed | No changes to license, provider snapshot, or Go/module policy |

New deterministic/mock tests cover both constructors, environment capture and
explicit credentials, deferred configuration errors and excess arguments,
first-use coordination across SMS/top-up/manual calls, independent cancellation,
idle renewal after first success, terminal recovery and transient first-use
cooldown, pre-canceled/invalid operations without authentication, all configured
HTTP timeouts and earlier caller deadlines, injected client immutability,
Close before/during initialization, and races between initial success and shutdown.
The existing lifecycle/wire/no-replay suite remains in place with eager call sites
migrated to `NewWithContext`. See [TESTING.md](TESTING.md) for named tests.

Compatibility changes: old `New(ctx, config)` calls must use `NewWithContext` to
retain eager behavior. `New` now reports initialization errors through operations.
The additional `Config.Timeout` field requires positional config literals to
migrate to named fields. Existing service methods retain their contexts and
never automatically replay submissions.

Restrictions during this update: the first sandboxed test attempt encountered a
read-only Go-cache entry and a call-site rename error (`errors.NewWithContext`),
which was corrected. A later sandboxed test attempt could not bind local
`httptest` listeners (`socket: operation not permitted`). These attempts are not
passes; the final unit/race checks succeeded with approved execution outside the
sandbox. Vet, example/README builds, and formatting succeeded inside the sandbox.
The requested skill search initially hit registry DNS restrictions, then succeeded
with approved network access; the already installed Go design patterns skill was
used. No dependency or skill installation was added to the repository.

## Baseline verification evidence (historical)

The following table records the earlier implementation delivery; additional
baseline-only checks were not rerun as part of this constructor update.

| Command/check | Final result | Evidence |
|---|---|---|
| `go version` | Passed | `go version go1.27.1 linux/amd64` |
| `gofmt -l .` | Passed | No output; includes newly created/untracked Go sources |
| `go test ./...` | Passed | Root package `ok`; four example packages compile with no tests; package example executes against a local mock |
| `go test -race ./...` | Passed | Root package `ok`; no race reports after final fixes |
| `go vet ./...` | Passed | Exit 0, no output |
| `go build ./examples/...` | Passed | Exit 0; all four programs compiled, none executed |
| `go build -o /tmp/telbiz-readme-check /tmp/telbiz-readme-check.go` | Passed | Complete README Go program extracted and compiled only |
| `go build -o /tmp/telbiz-errors-check.a /tmp/telbiz-errors-check.go` | Passed | Complete error-guide Go example extracted and compiled only |
| `git diff --check` | Passed | Exit 0, no whitespace errors |
| `go doc -all .` plus source/docs review | Passed | Exported types, constants, fields, methods and signatures agree with `docs/API.md` |
| Relative Markdown path review using Python `pathlib` | Passed | 67 relative links checked, including 12 heading anchors; all targets resolve |
| `git diff -- LICENSE docs/TELBIZ_API_REFERENCE.md go.mod` | Passed | No changes to tracked license/provider snapshot; supplied untracked module file contents remain unchanged |

The final tests cover conservative start-based expiry and installation expiry,
normal/variable/short token timing, idle renewal, shared waits and cancellation,
old-token preservation, capped backoff across expiry, sequential/concurrent
cooldown enforcement, terminal rejection/manual recovery, stale/current 401,
generic rejections, exact wire schemas, input and envelope validation,
unknown outcomes, body limits/closure, redirects and connection loss without
replay, caller resource ownership, and shutdown. `synctest` workers/timers are
contained in each test bubble; leaking workers would prevent completion.

## Restrictions encountered and resolved

- Initial sandboxed `go test ./...` attempts failed because `httptest` could not
  bind loopback sockets (`socket: operation not permitted`). Final unit and race
  commands ran successfully with approved execution outside the sandbox, using
  local mocks only.
- The first race run found unsynchronized counters in mock test instrumentation.
  These were made atomic. Subsequent runs, including the final source revision,
  passed. The initial race command was not treated as a pass.
- A final sandboxed `go vet ./...` attempt failed on a read-only Go build-cache
  entry. Final vet and example builds succeeded with approved cache access
  outside the sandbox. Commands after that failed chained vet did not run in that
  attempt; their successful final executions are listed separately above.

## Remaining external verification

| Item | Status |
|---|---|
| Licensing | Existing MIT license; no local blocker |
| Local delivery checks | Complete; no unresolved local check blocker |
| Older/newer Go versions | Unverified; no compatibility claim beyond Go 1.27.1 |
| Remote CI run | Workflow delivered; not executed remotely in this work |
| Remote installation/release | Source delivered locally; no push, tag or release publication |
| Live Telbiz compatibility/delivery/credit | Unverified; no live provider requests made |

Provider unknowns remain: overlapping-token validity, final delivery/credit,
transaction-key uniqueness/lookup, idempotency, rate limits, full HTTP mappings,
SMS length/encoding/segment billing, operators, and top-up denominations.
There is no provider-supported reconciliation endpoint in the supplied snapshot.
