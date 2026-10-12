# Delivery Checklist

All items are pending until the implementation agent supplies evidence. This
handoff contains specifications and templates, not a completed SDK.

## Implementation

- [ ] Inspect existing module/API/license/instructions and preserve unrelated work.
- [ ] Module path and package name match the build specification.
- [ ] Implement typed configuration, eager `New`, and context-aware public calls.
- [ ] Implement automatic idle renewal 60 seconds before normal expiry.
- [ ] Implement short lifetime handling, shared auth, bounded backoff, and Close.
- [ ] Implement SMS/top-up wire schemas, validation, result types, and safe errors.
- [ ] Parse flat and nested envelopes; preserve provider result codes and keys.
- [ ] No automatic service POST replay, including redirects or authentication errors.
- [ ] No documented but unsupported provider capabilities are invented.

## Documentation and repository

- [ ] README installation and full quick start work with implemented signatures.
- [ ] Usage, authentication, API, errors, testing, releasing, and implementation notes complete.
- [ ] Package documentation and two meaningful Mermaid flows complete.
- [ ] AGENTS, CONTRIBUTING, SECURITY, and changelog match actual repository behavior.
- [ ] Existing license preserved or owner's missing-license decision recorded.
- [ ] Gitignore, empty env example, PR and issue templates complete.
- [ ] Runnable examples compile and clearly identify billable operations.
- [ ] CI checks formatting, tests, race detector, vet, and examples.
- [ ] All relative documentation links resolve and no secrets appear in files.

## Acceptance

- [ ] `docs/TEST_PLAN.md` scenarios pass using mock endpoints and controlled time.
- [ ] Formatting, `go test ./...`, `go test -race ./...`, `go vet ./...`, example builds pass.
- [ ] Supported Go-version claims match verification evidence.
- [ ] Review final diff for scope, secrets, credential forwarding, and resource leaks.
- [ ] Final report distinguishes implementation, mock validation, provider unknowns, and blockers.
- [ ] No remote push or release publication performed without authorization.

## Implementation evidence

To be completed by the implementation agent:

| Check | Result | Evidence or blocker |
|---|---|---|
| Source formatting | Pending | |
| Unit/integration mock tests | Pending | |
| Race detector | Pending | |
| Vet | Pending | |
| Examples build | Pending | |
| Documented Go versions | Pending | |
| Docs/link and public API review | Pending | |
| License/release readiness | Pending | |

Provider unknowns to retain: overlapping-token behavior, final delivery/credit,
transaction-key lookup, idempotency, rate limits, full HTTP status mapping, SMS
length/encoding, and supported operators/denominations.
