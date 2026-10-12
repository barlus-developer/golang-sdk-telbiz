# Delivery Checklist

Local implementation completed 2026-10-12. Evidence below uses Go 1.27.1 on
linux/amd64. Verification uses fabricated inputs and local mocks only.

## Implementation

- [x] Inspect existing module/API/license/instructions and preserve unrelated work.
  Discovery Git status was clean; the subsequently appearing `go.mod` was preserved.
- [x] Module `github.com/barlus-developer/golang-sdk-telbiz`, package `telbiz`.
- [x] Typed configuration, eager `New`, context-aware public calls.
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

## Exact verification evidence

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
