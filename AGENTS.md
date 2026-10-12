# Agent Instructions

## Mission

Build and document the unofficial Telbiz Go SDK for module
`github.com/barlus-developer/golang-sdk-telbiz`, package `telbiz`.
Complete `BUILD_SDK.md` and `docs/DELIVERY_CHECKLIST.md`.

## Read first

Read the existing repository instructions and inspect Git status before edits.
Then read `BUILD_SDK.md`, `docs/TELBIZ_API_REFERENCE.md`,
`docs/TOKEN_LIFECYCLE.md`, `docs/TEST_PLAN.md`, and `CONTRIBUTING.md`.
The provider reference defines the wire contract; the build and lifecycle
documents define SDK behavior. Record conflicts explicitly rather than inventing
provider capabilities. Reconcile these instructions with existing files.

## Work rules

- Preserve unrelated edits and existing repository metadata.
- Produce the implementation, tests, working examples, documentation, and CI.
- Use idiomatic Go, small focused components, and minimal dependencies. A small
  SDK does not require an application framework, ORM, database, or server.
- Keep authentication coordination and transport internals unexported. If
  internal abstraction is useful, use clear domain/port/adapter names; do not
  impose a full architecture on this library.
- Use contexts, bounded HTTP calls, safe concurrent state, and deterministic
  lifecycle tests. Never use real credentials in tests or fixtures.
- Automatic renewal means obtaining a new access token with the same credentials.
  Never claim to rotate the Client ID or Secret.
- Do not invent refresh-token, webhook, sandbox, final-status, idempotency,
  data-package, or delivery APIs absent from the supplied reference.
- Never automatically replay SMS or top-up submissions, including after a 401,
  redirect, timeout, network failure, or server error.
- No logging of credentials, access tokens, recipient numbers, or SMS content.
- Do not make live billable requests as validation. Tests use local mock servers.
- Respect existing LICENSE and Go-version policy. Do not invent a license grant,
  owner email, security contact, or provider affiliation.
- No destructive Git operations, remote push, or release publication without
  authorization. Follow existing commit rules; this handoff adds no mandatory
  per-task commit gate.

## Verification and reporting

Run formatting checks, `go test ./...`, `go test -race ./...`, and
`go vet ./...`, and compile all examples. Record the exact checks actually run.
When a check is blocked, identify the command and reason; never label it passed.
Update the delivery checklist with evidence. Summarize the final public API,
token-renewal behavior, documentation delivered, checks, and remaining blockers.
Do not claim compatibility with a live provider from mock tests alone.
