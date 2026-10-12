# Telbiz Go SDK — Build Specification

Status: local SDK implementation, tests, examples, documentation, and CI delivered.
Prepared 2026-10-12; implementation evidence is recorded in
[docs/DELIVERY_CHECKLIST.md](docs/DELIVERY_CHECKLIST.md). This document retains the
build and acceptance requirements. Live provider compatibility and remote CI
execution have not been established; no release or remote push was performed.

## 1. Objective

Create a complete, documented, maintainable Go SDK in
`github.com/barlus-developer/golang-sdk-telbiz` with package name `telbiz`.
Consumers configure their Telbiz Client ID and Secret once. The client handles
initial authentication, access-token caching, automatic renewal, and bearer
authorization for SMS and mobile airtime top-up.

This SDK is unofficial. Provider affiliation must not be implied.

## 2. Grounding and scope

Read `docs/TELBIZ_API_REFERENCE.md`, supplied by the owner and reviewed there on
2026-10-12. Preserve it as a provider-contract snapshot. Its source links point
to the Telbiz developer manual. If current provider documentation becomes
available during implementation, record material differences and update the
snapshot deliberately. Do not silently expand scope based on another SDK.

| Operation | Method | Exact path | Request fields |
|---|---|---|---|
| Authentication | POST | `/api/v1/Connect/token` | `clientID`, `secret`, `grantType` |
| SMS submission | POST | `/api/v1/SMSService/newtransaction` | `title`, `phone`, `message` |
| Airtime top-up | POST | `/api/v1/TopupService/newtransaction` | `phone`, `amount` |

Default base URL: `https://api.telbiz.la`. Requests use JSON and
`Content-Type: application/json`. Authentication sends
`grantType: "client_credentials"`; service calls use the documented bearer header.
Preserve route casing and JSON names exactly.

Provider `expire` is a lifetime in seconds. Use the authentication schema rather
than the conflicting overview label `3600 ms`. Do not hardcode every token to
3,600 seconds: schedule from the actual positive `expire` returned.

No refresh-token endpoint is documented. Renewal repeats the authentication POST
with the same Client ID and Secret. No data-package, delivery-status, webhook,
sandbox, or idempotency interface belongs in the initial release.

## 3. Required consumer API

Implement this intended API surface. Go types below are a design contract; any
necessary adjustment must be justified and reflected consistently in examples.

| API | Contract |
|---|---|
| `New(ctx context.Context, cfg Config) (*Client, error)` | Validate config, authenticate immediately, start background renewal only after success |
| `(*Client).SendSMS(ctx context.Context, req SMSRequest) (*SMSResponse, error)` | Validate, acquire valid token, submit once |
| `(*Client).Topup(ctx context.Context, req TopupRequest) (*TopupResponse, error)` | Validate, acquire valid token, submit once |
| `(*Client).RefreshToken(ctx context.Context) error` | Force renewal or join a renewal already running; do not return the token |
| `(*Client).Close() error` | Stop client-owned background work; idempotent |

`Config` must include `ClientID`, `ClientSecret`, optional `BaseURL`, optional
`HTTPClient`, and optional `RefreshBefore`. `RefreshBefore == 0` uses 60 seconds;
negative values are invalid. Use a finite documented request timeout for SDK
network calls, including with an injected HTTP client. Keep the configuration
immutable after construction. Never mutate the caller's HTTP client or transport.

Do not force consumers to supply or manually maintain an access token. An
exported token getter is unnecessary. A diagnostic metadata API, if added, must
exclude token strings and credentials.

### Illustrative consumer usage

This excerpt illustrates the implemented API. Complete compilable examples with
imports, error handling, and environment-variable credentials are under
[examples/](examples/) and in the [README](README.md).

```go
client, err := telbiz.New(ctx, telbiz.Config{
    ClientID:     os.Getenv("TELBIZ_CLIENT_ID"),
    ClientSecret: os.Getenv("TELBIZ_CLIENT_SECRET"),
})
if err != nil {
    return err
}
defer client.Close()

result, err := client.SendSMS(ctx, telbiz.SMSRequest{
    Title:   telbiz.TitleOTP,
    Phone:   "2012345678",
    Message: "Your verification code is 123456",
})
```

Long-lived applications reuse one client. Multiple clients renew independently;
no distributed or cross-process token coordination is promised.

## 4. Automatic token renewal

Follow `docs/TOKEN_LIFECYCLE.md` in full. Constructor authentication is eager.
Renew proactively even when the client is idle. With a normal 3,600-second token
and default configuration, renewal starts 60 seconds before expiry.

Use one coordinated renewal operation per client, atomic token replacement,
bounded authentication attempts, safe shared state, and clean shutdown. Treat
the constructor context as initialization-only after `New` succeeds. Callers
stop the background lifecycle through `Close`, not through a completed startup
context. A failed renewal must not erase an unexpired token.

## 5. Models, validation, and response handling

- Export typed SMS titles: `TitleOTP`, `TitleInfo`, `TitlePromotion`, `TitleNews`.
  Values must be exactly `OTP`, `Info`, `Promotion`, `News`.
- `SMSRequest` has `Title`, `Phone`, and `Message` with documented JSON names.
  Require a permitted title, ten ASCII digits starting with `20`, and a nonblank
  message. Do not trim or rewrite the transmitted message.
- Phone validation is a local formatting rule based on the supplied examples,
  not operator support or proof that a number is allocated. Do not silently
  normalize `+856` or other input formats.
- `TopupRequest` has `Phone` and integer `Amount` (prefer `int64`, LAK). Send
  amount as a JSON number. Require at least 10,000 following the documented
  lower bound. Do not invent denominations or maximum amounts. Record the
  whole-kip integer choice as SDK policy.
- Auth success is flat: `accessToken`, `expire`, `code`, `message`, `success`.
- Service success is nested: `response` plus `key.partitionKey` and
  `key.rangeKey`. Service failure is flat: `code`, `message`, `success`, `detail`.
- Parse both documented response shapes. Distinguish a missing success field
  from explicit `false`. Handle HTTP 200 provider failures as errors.
- Preserve provider code, message, detail, and transaction key fields. Unknown
  provider codes must remain usable; do not reject them solely for being new.
- Recognize documented codes: `SUCCESS_OPERATION`, `SENDING_SMS_INPROGRESS`,
  `TOPUP_BALANCE_INPROGRESS`, `UNSUPPORTED_HEADER`, `SEND_SMS_FAIL`, `REQUEST_FAIL`.
- In-progress results mean accepted submissions, never confirmed SMS delivery
  or confirmed airtime credit. Transaction-key uniqueness is unspecified.

Define inspectable errors usable with `errors.Is` and `errors.As`: validation,
closed client, authentication, provider rejection, transport, and malformed
response. Preserve underlying context/transport causes. HTTP/provider errors
must expose HTTP status and provider code where available. Default error strings
must not include raw request/response bodies, authorization, message text, phone,
or provider free text that could echo sensitive inputs. Structured provider text
must be documented as potentially sensitive.

For ambiguous service outcomes, provide an inspectable indication, such as
`OutcomeUnknown`, separate from retry advice. Timeout, mid-flight cancellation,
connection loss after submission, or an untrustworthy success response can leave
outcome unknown. Do not claim every transport failure means no transaction took
place. Local validation or failure before service submission is not an unknown
service outcome.

## 6. Transport and retry behavior

Centralize endpoint construction, JSON encoding/decoding, authentication,
response-body closure, size limits, and errors. Use the standard library where
practical. Reject malformed base URLs, URL credentials, query strings, and
fragments. Allow HTTP only for an explicitly configured local test/development
base URL; document that credentials require HTTPS in production.

Service operations perform one HTTP submission. Disable automatic redirect
following for SDK calls without modifying the caller's HTTP client; this also
prevents forwarding bearer credentials and replaying POSTs on 307/308 redirects.
Do not attach idempotency headers without a provider contract. Do not install
generic retry middleware around service POSTs. An injected custom RoundTripper
must not retry billable POSTs; document that consumer responsibility.

A 401 may trigger token invalidation and coordinated renewal for future calls,
but must return the current service error without replaying the operation. A late
401 from an old token must not invalidate a newer token. Never interpret generic
`REQUEST_FAIL` or all 403 responses as proven token expiry.

Authentication retries may be bounded for transient failures; never repeatedly
retry definite invalid credentials. Describe which statuses/errors are treated
as transient without pretending the manual defines exact HTTP mappings.

## 7. Documentation and repository completion

Create or finish every file below. Preserve existing compatible content. This
handoff's root `START_HERE.md` is a handoff guide; create a separate SDK README.

| Deliverable | Required content |
|---|---|
| `README.md` | Unofficial status, install command, Go minimum, credentials, complete quick start, SMS/top-up examples, auto-renewal and Close, accepted-vs-completed semantics, links |
| `doc.go` | Package documentation, authentication lifecycle, concurrency and cleanup contract |
| `docs/USAGE.md` | Configuration table/defaults, contexts, client reuse, HTTP client/base URL customization, examples |
| `docs/AUTHENTICATION.md` | Initial auth, seconds-based expiry, automatic/manual renewal, failures, short lifetime, shutdown, Mermaid flow |
| `docs/API.md` | All exported SDK types/methods and documented provider routes, request/response shape, result semantics |
| `docs/ERROR_HANDLING.md` | `errors.Is/As` examples, provider vs HTTP errors, sensitive fields, ambiguous outcomes, no automatic service replay |
| `docs/TESTING.md` | Mock tests, test commands, deterministic lifecycle tests, no live billing in CI |
| `docs/RELEASING.md` | SemVer, pre-release readiness, checks, tags/module-path rules, changelog; preparation does not publish |
| `docs/IMPLEMENTATION_NOTES.md` | SDK design choices, source conflicts/unknowns, verified Go support, validation policy |
| `AGENTS.md` | Keep agent rules accurate for final layout and commands |
| `CONTRIBUTING.md` | Setup, workflow, tests, API changes, fixtures, PR requirements |
| `SECURITY.md` | Sensitive reporting route, supported-version policy stated truthfully, credential handling |
| `CHANGELOG.md` | Only implemented changes under Unreleased; no fabricated release or dates |
| `LICENSE` | Preserve existing license; if absent, record that owner's selection is pending instead of assigning one |
| `.gitignore` | Appropriate Go artifacts and secret-bearing `.env` files; retain `.env.example` |
| `.env.example` | Empty `TELBIZ_CLIENT_ID` and `TELBIZ_CLIENT_SECRET`; explanatory comments, no real secrets |
| `.github/workflows/ci.yml` | Formatting, tests, race tests, vet, example compilation on documented supported Go versions |
| `.github/PULL_REQUEST_TEMPLATE.md` | Concrete behavior, validation, docs, auth/concurrency/retry impact |
| `.github/ISSUE_TEMPLATE/` | Bug and feature templates that explicitly prohibit posting secrets or SMS/phone data |
| `examples/` | Standalone runnable SMS, top-up, manual renewal, and cancellation/lifecycle examples; note billable effects |
| `go.mod` and source/tests | Correct module path, coherent public API, required test coverage |

All new user-facing SDK documentation is English. Examples use environment
variables and placeholder phone numbers. No demo should automatically execute
live SMS/top-up during test or CI. Include at least two Mermaid diagrams: token
lifecycle and SDK service flow. Clearly label provider downstream behavior as
conceptual. Relative Markdown links and example signatures must match real files
and exported names.

Select the minimum Go version by inspecting existing repository policy and the
toolchains actually available; document and test it. Do not copy an assumed
version from this brief. Avoid dependency additions that only duplicate simple
standard-library behavior.

## 8. Execution sequence and completion

1. Inspect the repository, existing API, module, license, instructions, and tests.
2. Resolve wire-contract models and record uncertainties in implementation notes.
3. Implement transport/errors and eager authentication with automatic lifecycle.
4. Implement typed SMS and top-up operations with one submission per invocation.
5. Complete the mock and concurrency tests in `docs/TEST_PLAN.md`.
6. Create all documentation, examples, repository support files, and CI.
7. Run required verification and finish `docs/DELIVERY_CHECKLIST.md` with evidence.

Do not stop after a scaffold, plan, happy-path implementation, or README. Complete
the entire authorized local delivery. If external access or tools block checks,
continue independent work and report precisely what remains unverified. Release
readiness remains pending if licensing is unresolved; that need not block local
implementation and documentation.

## 9. Delivered implementation

The public API above is implemented in the root `telbiz` package with no external
dependencies. The pre-existing-at-implementation `go.mod` specifies Go 1.27.1 and
was preserved. All five client entry points, typed request/result models, safe
inspectable errors, single shared renewal, capped backoff, and no service replay
are implemented. Token validity is checked both during response processing and
at installation/admission.

The [API reference](docs/API.md), [usage guide](docs/USAGE.md),
[authentication guide](docs/AUTHENTICATION.md), [error guide](docs/ERROR_HANDLING.md),
[test mapping](docs/TESTING.md), [implementation notes](docs/IMPLEMENTATION_NOTES.md),
and [release guide](docs/RELEASING.md) describe the final behavior. Four standalone
examples cover SMS, top-up, manual renewal, and cancellation/lifecycle. Package
examples use local mocks. CI compiles examples without running live operations.
The existing MIT license and repository templates are preserved.

See [delivery evidence](docs/DELIVERY_CHECKLIST.md) for exact local checks and
remaining external verification. No live compatibility, final fulfillment,
remote CI success, or release publication is claimed.
