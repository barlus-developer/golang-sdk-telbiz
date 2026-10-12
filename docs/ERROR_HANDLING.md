# Error handling

Handle `New` errors before using a client. Service errors never trigger an SDK
replay. Match categories with `errors.Is` and inspect fields with `errors.As`.

```go
package integration

import (
    "context"
    "errors"
    "fmt"

    telbiz "github.com/barlus-developer/golang-sdk-telbiz"
)

// Describe returns safe text; provider fields are deliberately not logged.
func Describe(err error) string {
    if err == nil {
        return "submission accepted; completion unconfirmed"
    }
    if errors.Is(err, telbiz.ErrClosed) {
        return "client closed"
    }
    if errors.Is(err, telbiz.ErrValidation) {
        var e *telbiz.Error
        if errors.As(err, &e) {
            return "invalid field: " + e.Field
        }
        return "invalid input"
    }
    var e *telbiz.Error
    if errors.As(err, &e) && e.OutcomeUnknown {
        return "submission outcome unknown; reconcile before repeating"
    }
    if errors.Is(err, context.Canceled) {
        return "caller canceled"
    }
    if errors.Is(err, context.DeadlineExceeded) {
        return "deadline exceeded"
    }
    if errors.Is(err, telbiz.ErrAuthentication) {
        return "authentication unavailable"
    }
    if errors.As(err, &e) {
        return fmt.Sprintf("operation failed (HTTP %d)", e.HTTPStatus)
    }
    return "operation failed"
}
```

## Categories and inspection

| Match | Meaning |
|---|---|
| `ErrValidation` | Invalid configuration, request field, or nil context; no service submission |
| `ErrClosed` | Client has been closed; no new submission |
| `ErrAuthentication` | Initial/renewal authentication failed, or token wait canceled; no service submission by the affected waiting call |
| `ErrProvider` | HTTP failure or explicit provider rejection, even with HTTP 200 |
| `ErrTransport` | Network/request/body-read failure; inspect wrapped cause |
| `ErrMalformedResponse` | Invalid JSON, missing required envelope/success, oversized body, or unusable authentication response |

Authentication errors wrap their underlying category. For example, a rejected
token request matches both `ErrAuthentication` and `ErrProvider`. Context causes
remain matchable with `errors.Is`. `errors.As(err, &sdkError)` returns an `*Error`
with `HTTPStatus`, `Code`, `Message`, `Detail`, `Operation`, `Field`, and
`OutcomeUnknown` where applicable. A zero HTTP status means none was available.

Unknown provider codes are preserved. An HTTP error can have no provider code
when its body is HTML or otherwise unusable. A provider rejection on HTTP 200 is
still `ErrProvider`; a nested success on HTTP 500 is not successful SDK acceptance.

## Sensitive fields

`Error.Error()` contains only a fixed SDK category, excluding provider code/text,
raw bodies, credentials, authorization, recipient, and message content.
`Message`, `Detail`, and even untrusted provider `Code` may echo sensitive inputs.
Returned result fields and unwrapped transport causes also need careful handling.
Do not log complete errors as structured objects, request/config/response objects,
or underlying causes without application-specific redaction. The SDK has no
logger and emits no request/response logs.

## Unknown outcomes

`OutcomeUnknown` describes uncertainty, not permission or advice to retry.
Once a service request is handed to HTTP transport, transport errors (including
mid-flight cancellation, timeout, connection loss, and body-read failure) are
conservatively unknown. Some may have failed before transmitting bytes, but the
SDK cannot reliably establish that for arbitrary injected transports.
Malformed/untrustworthy success responses and non-2xx responses without an
explicit trustworthy rejection are also unknown. A valid `success:false` result
is a provider rejection. Local validation, canceled contexts before submission,
closure, and authentication failures do not mark a service outcome unknown.

Keep application records of unresolved submissions and use a provider-supported
way to investigate before repeating them. The supplied contract provides no
reconciliation, delivery-status, transaction-key lookup, or idempotency endpoint.

SMS/top-up are never automatically replayed after 401, 403, 429, 5xx, redirects,
timeouts, cancellation, or connection failures. A 401 can renew for future calls
without repeating the current operation. A custom transport must uphold the same
rule. [Authentication retry policy](AUTHENTICATION.md#failures-and-retry-policy)
applies solely to token requests.
