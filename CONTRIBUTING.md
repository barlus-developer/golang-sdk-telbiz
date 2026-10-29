# Contributing

This repository implements an unofficial Go SDK for Telbiz. `BUILD_SDK.md` and
the provider/lifecycle documents define its contract. See [API docs](docs/API.md)
and [delivery evidence](docs/DELIVERY_CHECKLIST.md) for implemented behavior and
actual checks. Do not describe unverified behavior as passing.

## Development

Read `AGENTS.md`, the provider reference, lifecycle contract, and test plan.
Use Go 1.27.1, matching the preserved `go.mod` and verified local toolchain, and run:

```bash
go mod download
go test ./...
go test -race ./...
go vet ./...
go build ./examples/...
```

Format Go code with `gofmt`. Keep dependencies minimal. Use local mock servers,
controlled time, and fabricated test inputs. Never run live SMS/top-up requests
in tests or CI. Store local credentials in ignored environment files or a secret
manager; `.env.example` must contain empty placeholders only.

Before staging newly created Go sources, `gofmt -l .` must produce no output.
Lifecycle tests use `testing/synctest` to advance time deterministically. See
[testing guidance](docs/TESTING.md) for scenario-to-test mapping. Example programs
are compiled only during checks; explicitly executing SMS/top-up examples is
billable. The SDK currently has no third-party dependencies.

## Changes and pull requests

Use a focused branch and explain the behavior change. Include relevant tests,
documentation, examples, and an Unreleased changelog entry. Preserve existing
API compatibility unless an intentional breaking change is documented. Discuss
public API and dependency changes in the PR description. Never invent provider
routes or guarantees to complete a feature.

For constructor changes, cover lazy `New`, eager `NewWithContext`, explicit vs
environment credentials, deferred errors, and per-request timeouts. For
authentication changes, verify renewal scheduling, shared first-use/refresh, failure
backoff, cancellation, and shutdown with race tests. For service changes, prove
that a call does not automatically replay a billable POST.

Use the PR template and report actual validation results. Do not include client
secrets, bearer tokens, real recipients, message/OTP content, or sensitive raw
responses in issues, logs, commits, or screenshots. Report vulnerabilities using
`SECURITY.md`.

## Licensing and releases

Follow the existing repository license. If absent, the owner must choose one;
contributors and agents must not add an assumed license grant. Release preparation
and publication follow `docs/RELEASING.md`. A prepared release is not
authorization to push tags or publish.
