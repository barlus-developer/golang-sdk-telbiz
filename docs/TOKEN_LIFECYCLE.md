# Access-Token Lifecycle Contract

This defines SDK behavior. Telbiz's supplied contract defines the token endpoint
and lifetime; it does not guarantee overlapping-token validity or internal token
issuance behavior. Record this as an integration uncertainty.

## Initialization and expiry

1. `New(config ...Config) *Client` accepts zero or one config, validates locally,
   stores any configuration error for operations, and makes no network request.
   With no config, capture environment credentials once. Explicit config never
   falls back to environment. More than one config reports `ErrValidation`,
   field `Config`. No worker or timer runs before a token-requiring operation.
2. `NewWithContext(ctx, config) (*Client, error)` validates and authenticates
   immediately within the initialization context and configured request timeout.
   Return errors directly and leave no active worker/timer on failure.
3. Lazy first use joins/starts the same auth flight used by renewal and manual
   refresh. Shared auth uses the client-owned context; caller cancellation ends
   only its wait. An earlier caller deadline wins for its wait or service request,
   without canceling authentication needed by other callers.
4. Require provider success, nonempty access token, and a positive integer
   lifetime in seconds. Reject missing, fractional, zero, negative, or overflowing
   lifetimes. Conservatively compute `expiresAt = authRequestStartedAt + lifetime`;
   reject responses exhausted on arrival or installation.
5. Install the token/deadlines atomically and start exactly one lifecycle loop
   after first success. Register workers while shutdown is excluded by the state
   lock, so concurrent lazy initialization and `Close` cannot leak workers or race
   with `WaitGroup.Wait`. Closing during auth prevents token installation.
6. `NewWithContext`'s context controls initialization only. Both constructors use
   an independent cancelable lifecycle context stopped by `Close`. `Config.Timeout`
   bounds every HTTP request, including injected clients without mutation; zero
   uses the existing 30-second default and negative values are invalid.

Default refresh lead is 60 seconds. Effective lead is
`min(configuredLead, lifetime / 2)` so a short-lived token cannot produce an
immediate renewal loop. Normal one-hour tokens still use the full 60-second lead.
`refreshAt = expiresAt - effectiveLead`. If that point passed during the auth
request but the token is still valid, schedule a bounded renewal promptly; never
spin in an immediate success/renew loop. Use a positive minimum scheduling delay
and document its behavior for exceptionally slow auth responses.

Successful renewal recomputes both deadlines from the new response and resets
the scheduler. Renew even with no service traffic. Use timers rather than a
periodic busy poll. Tests should control time without waiting an hour.

## Concurrency and request acquisition

- All token reads/replacements must be race-free. Do not hold a state mutex while
  making HTTP calls or waiting for another operation.
- At most one auth operation is in flight per client across background renewal,
  request-triggered renewal, and `RefreshToken`.
- Service calls may use the current token while it remains unexpired, including
  while proactive renewal is running. Snapshot its generation with the token.
- If no valid token exists, join or start the coordinated auth operation and wait
  with the service caller's context. Submit no service request on auth failure.
- Caller cancellation stops that caller's wait, not a renewal needed by other
  callers. Shared auth work uses lifecycle context plus a bounded timeout.
- `RefreshToken` forces renewal unless an operation is already in progress, in
  which case it joins it. Respect cancellation without canceling shared work.
- Prevent sequential failure storms as well as concurrent duplication: keep a
  shared retry/cooldown deadline so new requests cannot bypass transient backoff.
- If a 401 came from generation N, only invalidate N if it is still current.
  Never invalidate N+1 because an older request finished later.
- Renewal failure never replaces or clears a still-valid token. Token snapshots
  sent to a request cannot be changed midway through that request.

## Renewal failures

Use finite auth request timeouts and capped backoff for transient failures.
Use delays of 1, 2, 4, 8, 16, then 30 seconds, retaining the 30-second cap.
Retry authentication only, never SMS/top-up.

Before first lazy authentication success, there is no automatic retry worker.
Transient failures establish cooldown; later calls can attempt after cooldown.
Terminal failures require explicit manual recovery. After success, the idle
lifecycle retries transient failures automatically.

When a retry is scheduled before expiry, account for the remaining validity; after
expiry, retain bounded backoff instead of a zero-delay loop. A renewed token must
be valid when installed. Tests must prove no tight-loop behavior at expiry.

Definitive credential rejection stops automatic retry churn. Retain an unexpired
old token until its deadline, expose the subsequent authentication failure to
callers, and allow an explicit `RefreshToken` attempt. Do not classify an unknown
provider code as transient or terminal without an explicit documented SDK policy.

If a valid old token exists, service calls may proceed; if none exists, they wait
for an allowed refresh attempt or return an inspectable authentication/context
error. They must never submit with an expired token. Explain this behavior in
`docs/AUTHENTICATION.md`, including how background failures become visible when
authentication is required. Optional diagnostic hooks must be secret-free and
must not block the renewal loop indefinitely.

## Shutdown

`Close()` is idempotent and safe before first use and under concurrent calls.
Closing an unused client prevents any later authentication. It marks the client
closed, cancels pending lifecycle auth work, stops the timer, unblocks refresh
waiters, and joins the client-owned lifecycle goroutine. No new renewal or service
submission may start after closure. Already-started service calls may complete
under their own context and finite timeout; document this distinction. Calls
after closure return a recognizable closed-client error.

Do not close an injected caller-owned HTTP client or transport. Close idle
connections only for SDK-owned transport resources. Document that custom
RoundTrippers must honor cancellation and must not retry service POSTs.

## Lifecycle flow

```mermaid
flowchart TD
    New["New: capture config; no network"] --> First["First token-requiring operation"]
    First --> Auth["One shared initial auth request"]
    Auth --> Initial{"Valid auth response?"}
    Eager["NewWithContext: authenticate now"] --> Initial
    Initial -->|No| Fail["Return error; no worker"]
    Initial -->|Yes| Ready["Cache token and expiry"]
    Ready --> Wait["Wait until expiry minus lead"]
    Wait --> Refresh["One coordinated auth request"]
    Refresh --> Result{"Renewal result?"}
    Result -->|Success| Ready
    Result -->|Transient failure| Retry["Retain valid token; bounded backoff"]
    Retry --> Refresh
    Result -->|Credentials rejected| Halt["Retain valid token; stop retry churn"]
    Halt -->|Explicit refresh| Refresh
    New -->|Close before use| Stop["Cancel work and stop"]
    Auth -->|Close| Stop
    Wait -->|Close| Stop
    Refresh -->|Close| Stop
    Retry -->|Close| Stop
    Halt -->|Close| Stop
```
