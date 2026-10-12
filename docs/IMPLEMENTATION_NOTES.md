# Implementation notes

## Repository and design

The repository initially contained documentation, an MIT license, security and
contribution guidance, and issue/PR templates. Git was clean during discovery.
Before implementation a new untracked `go.mod` appeared, specifying the correct
module and `go 1.27.1`. It was preserved verbatim. The available local toolchain
reports `go1.27.1 linux/amd64`; no older-version compatibility is claimed.

The SDK uses one root `telbiz` package with private transport, token state,
clock/timer abstraction, and lifecycle coordination. It uses only the standard
library. The owner-requested `New(config ...Config)` and `NewWithContext(ctx,
Config)` replace the earlier build specification's eager `New(ctx, Config)`.
The installed Go design patterns skill's general functional-options preference
does not override this API. The existing SDK was modified in place on `alpha-0.1`;
discovery for this constructor update found a clean working tree. There is no application framework or exported auth coordinator.

One state mutex protects token/deadline/generation replacement and admission.
After first authentication success, one lifecycle worker schedules timers;
at most one shared auth worker runs
at a time. Waiters cancel independently. Worker registration and closure are
serialized so `Close` can safely join all client-owned work. No state mutex is
held during HTTP I/O. A service is considered started at its successful admission
check; calls admitted before `Close` can finish afterward under their own context.
This allows resource cleanup without taking ownership of caller service contexts.

Lazy creation captures environment credentials only when no config is provided.
Configuration validation errors are retained on the client for operations to
report. It starts no worker until first token use. Initial lazy auth uses the
same flight as renewal; its lifetime is independent of waiter contexts. A failed
first auth retains the existing terminal/cooldown policy but starts no idle retry
worker. Registering lifecycle workers under the state lock after first success
prevents concurrent Close from missing workers. Eager initialization authenticates
within the supplied context before publishing the client and cleans up on errors.

HTTP calls use a copied/internally created client, disabled redirects, a configured
per-request context timeout (zero uses the existing 30-second default), nonreplayable POST bodies, and a 1-MiB response limit. Only private
SDK-owned transport idle connections are closed. Consumer transports must honor
cancellation and avoid retrying service POSTs. See [authentication](AUTHENTICATION.md)
for exact renewal/backoff policy and [errors](ERROR_HANDLING.md) for uncertainty.

## Wire conflicts and deliberate policies

The [supplied provider snapshot](TELBIZ_API_REFERENCE.md) remains unchanged.
No provider capability is inferred from other SDKs.

| Topic | Resolution |
|---|---|
| Earlier eager `New(ctx, Config)` build/lifecycle requirement | Owner-requested API revision supersedes it; `New` is lazy, `NewWithContext` eager; no provider change inferred |
| Fixed 30-second request timeout | Preserve it as the zero-value default; allow positive `Config.Timeout` per HTTP request |
| Overview `3600 ms` vs one-hour/auth seconds | Interpret actual positive `expire` as integer seconds; never hardcode 3,600 |
| Top-up string sample vs numeric schema | Send a JSON number; `int64` whole-kip choice is SDK policy |
| Top-up minimum | Enforce 10,000 from the field description; assume no maximum/denomination |
| Phone examples | Local rule: ten ASCII digits starting `20`; no normalization/allocation guarantee |
| SMS heading inconsistency | Follow permitted title values and nested success example |
| Provider HTTP mappings incomplete | Treat 408/429/5xx and transport deadlines as transient auth failures by SDK policy; no invented provider mapping |
| Unknown provider rejection codes | Preserve fields; stop automatic auth retries unless the HTTP status is transient under the explicit policy |
| Base URL interpretation | Accept origins only; reject path prefixes to preserve exact documented routes; allow HTTP only for explicit loopback development |
| Extremely slow auth/short lifetime | Conservative start-based expiry and minimum 100-ms scheduling delay prevent spins; exhausted responses rejected |
| Submission uncertainty | Conservatively mark transport errors after HTTP admission unknown; this is independent of retry advice |

Successful service responses require the documented nested result and nonnull
key object. Empty key strings are valid. Explicit false is an error even on HTTP
200. Unknown successful result codes remain usable when the envelope is sound.
Error strings omit provider-controlled text, including codes; fields preserve it
for deliberate inspection. No logging, token getter, or diagnostic callback exists.

## Provider unknowns

Overlapping-token validity, final SMS delivery/airtime credit, key uniqueness and
lookup, idempotency, rate limits, complete HTTP mappings, SMS length/encoding and
segment billing, supported operators, and allowed top-up denominations remain
unspecified. The SDK has no refresh-token, data-package, webhook, final-status,
delivery-query, sandbox, or reconciliation interface.

## Verification limits

Tests use fabricated inputs, local mock servers, and deterministic `synctest`
time. They validate SDK behavior against the snapshot. They do not establish live
Telbiz compatibility. Local socket restrictions require approved execution outside
the filesystem/network sandbox for `httptest` checks. See the
[delivery checklist](DELIVERY_CHECKLIST.md) for exact final results.

CI uses the documented `v7` major actions from the official
[checkout](https://github.com/actions/checkout) and
[setup-go](https://github.com/actions/setup-go) repositories, with read-only
contents permission and no publication step. Remote execution is pending.
