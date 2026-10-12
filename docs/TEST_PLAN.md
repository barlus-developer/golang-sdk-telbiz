# Required Verification

Use `httptest` mock endpoints and deterministic time/timer control. No real Telbiz
traffic, credentials, SMS delivery, or airtime charges. Do not export testing-only
clock hooks as consumer API unless justified.

| Area | Required evidence |
|---|---|
| Constructor | Lazy `New` with zero/one config and no network; eager `NewWithContext`; explicit/environment credentials; deferred inspectable config/auth errors; invalid arity; correct wire contract; first success starts lifecycle |
| Request timeout | Configured/default bounds on auth, renewal, SMS/top-up; negative timeout invalid; earlier caller deadlines win; injected clients not mutated |
| Default timing | `expire: 3600` renews at the 60-second lead, including while idle |
| Variable timing | Returned lifetime drives deadlines; configurable lead; short lifetime clamps to half; delayed response cannot cause an auth spin |
| Token replacement | Subsequent calls use new bearer token; old token remains usable during renewal; failed renewal preserves old deadline/token |
| Concurrency | Background, request, and manual refresh share one in-flight auth call; many callers with no valid token coordinate safely |
| Wait cancellation | One canceled caller does not cancel shared auth or break other callers; waiting callers honor their context |
| Failure backoff | Transient failures are bounded; expiry does not cause tight loops; concurrent and sequential calls cannot bypass cooldown |
| Credential failure | Definitive rejection does not retry endlessly; invalid credentials surface clearly; explicit refresh remains possible |
| Expired token | No expired token is attached to a new service submission; failed auth causes zero service POSTs |
| Old 401 | A late response for an older token does not invalidate a newly installed token |
| Service routes | Exact route capitalization, JSON fields, JSON number for amount, bearer header, and one submission |
| Input validation | Valid titles/phone/messages/amounts accepted; invalid inputs rejected before service submission; no invented SMS length limit |
| Response envelopes | Nested service success, flat errors, HTTP 200 `success:false`, missing success, unknown codes, empty transaction keys |
| Malformed responses | Empty/truncated/invalid JSON, HTML errors, body size bound, absent token, invalid/overflowed expire, missing required envelope |
| Error inspection | `errors.Is/As`, underlying context causes, HTTP status/code, provider rejection vs ambiguous outcome, safe default error text |
| No replay | SMS/top-up each get one submission on 401, 403, 429, 5xx, timeout, connection loss, and 307/308 redirects |
| Redirect security | No auth/bearer/body follows a redirect; injected client is not mutated |
| Resource cleanup | Response bodies close on success/failure; retries do not leak resources; caller-owned transport not closed |
| Shutdown | Close before/during first authentication, race first use with shutdown, Close idle/in-flight renewal, concurrent repeated Close, waiters unblocked, closed-client errors, no lifecycle goroutine/timer left |
| Examples/docs | Examples compile; package examples run against mocks only; documented names/config/defaults agree with exported API |

Required local checks after implementation:

```bash
go test ./...
go test -race ./...
go vet ./...
go build ./examples/...
```

Check formatting with `gofmt` over Go sources. CI must fail if tracked source is
unformatted. Verify each documented supported Go version; do not claim support
for versions never tested. Test behavior and invariants rather than mirroring
private implementation details. No timing test should sleep for real token life.

Record commands, pass/fail/block status, and a concise reason for blocked checks
in the final implementation handoff. Mock success is not live provider validation.
