# Telbiz Go SDK

An unofficial Go client for Telbiz authentication, SMS submission, and mobile
airtime top-up. Module: `github.com/barlus-developer/golang-sdk-telbiz`; package:
`telbiz`. This project is not affiliated with Telbiz.

Requires **Go 1.27.1**, matching the existing module directive and the verified
local toolchain. The SDK has no third-party dependencies. The existing
[MIT license](LICENSE) is preserved.

## Install

Once these sources are available on the remote module:

```bash
go get github.com/barlus-developer/golang-sdk-telbiz
```

For a local checkout, run `go build ./...`. No release or remote push is included
in this implementation.

## Quick start

Obtain a Client ID and Secret from your Telbiz account and provide them as
`TELBIZ_CLIENT_ID` and `TELBIZ_CLIENT_SECRET` in your backend environment. The SDK
does not load `.env` files. See [.env.example](.env.example).

Running this program with `TELBIZ_PHONE` and `TELBIZ_MESSAGE` submits a **billable
SMS**. Set the intended recipient explicitly; `2012345678` is an illustrative
format, not a guaranteed test number.

```go
package main

import (
    "context"
    "fmt"
    "os"
    "time"

    telbiz "github.com/barlus-developer/golang-sdk-telbiz"
)

func main() {
    if err := run(); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}

func run() error {
    phone, message := os.Getenv("TELBIZ_PHONE"), os.Getenv("TELBIZ_MESSAGE")
    if phone == "" || message == "" {
        return fmt.Errorf("set TELBIZ_PHONE and TELBIZ_MESSAGE before sending")
    }
    ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
    defer cancel()
    client := telbiz.New() // Reads credentials; makes no network request.
    defer client.Close()
    _, err := client.SendSMS(ctx, telbiz.SMSRequest{
        Title: telbiz.TitleOTP, Phone: phone, Message: message,
    })
    if err != nil {
        return err
    }
    fmt.Println("SMS accepted; final delivery is unconfirmed.")
    return nil
}
```

## Top-up and runnable examples

The [top-up example](examples/topup/main.go) submits a **billable** top-up using
`TELBIZ_PHONE` and `TELBIZ_AMOUNT` (whole LAK, at least 10,000). Given an existing
client and context, its operation is:

```go
result, err := client.Topup(ctx, telbiz.TopupRequest{
    Phone: "2012345678", // Illustrative format; use the intended recipient.
    Amount: 10000,
})
```

| Example | Command | Effect when explicitly run |
|---|---|---|
| [SMS](examples/sms/main.go) | `go run ./examples/sms` | Billable SMS submission |
| [Top-up](examples/topup/main.go) | `go run ./examples/topup` | Billable airtime submission |
| [Manual renewal](examples/refresh/main.go) | `go run ./examples/refresh` | Authentication only |
| [Lifecycle/cancellation](examples/lifecycle/main.go) | `go run ./examples/lifecycle` | Authentication and idle renewal until shutdown |

## Client behavior

`New()` reads `TELBIZ_CLIENT_ID` and `TELBIZ_CLIENT_SECRET` once at construction.
`New(config)` uses only the explicit credentials. Both return a client without
network activity; the first `SendSMS`, `Topup`, or `RefreshToken` authenticates.
Invalid configuration and authentication failures are returned by operations;
use `errors.Is`/`errors.As` to inspect them. More than one config is invalid.

```go
client := telbiz.New(telbiz.Config{
    ClientID: "YOUR_CLIENT_ID",
    ClientSecret: "YOUR_CLIENT_SECRET",
    Timeout: 15 * time.Second,
})
defer client.Close()
```

For immediate validation and authentication with an initialization context:

```go
client, err := telbiz.NewWithContext(ctx, config)
if err != nil {
    return err
}
defer client.Close()
```

**Compatibility change:** migrate old `New(ctx, config)` calls to
`NewWithContext(ctx, config)` to retain eager authentication. Switching to `New`
also moves initialization errors to the first operation. Adding `Config.Timeout`
requires callers using positional config literals to migrate to named fields.

Reuse one client across goroutines. Concurrent first calls share authentication;
canceling a caller ends its wait without canceling shared work. After the first
successful authentication, the token renews while idle, normally 60 seconds
before expiry. The lead is clamped to half the returned lifetime. Renewal uses
the same credentials. `RefreshToken(ctx)` forces renewal or joins work in progress,
subject to transient failure cooldown.

`NewWithContext`'s context controls initialization only. Always call `Close()`;
it is idempotent and safe before first use, prevents later authentication, and
stops client-owned workers. Already admitted service calls may finish under their
own contexts. `Config.Timeout` bounds each authentication, renewal, SMS, and
top-up HTTP request; zero uses the existing 30-second `RequestTimeout` default,
and negative values are invalid. Earlier caller deadlines and injected client
timeouts win. The bound also applies to injected HTTP clients without mutation.

SMS and top-up calls submit once. They are never replayed automatically after
401s, redirects, transport failures, or provider errors. Inspect errors and
`Error.OutcomeUnknown` when a response cannot establish the submission outcome.
Accepted/in-progress responses confirm neither SMS delivery nor airtime credit.
Provider free-text fields may be sensitive; the SDK performs no logging.

## Documentation

- [Usage and configuration](docs/USAGE.md)
- [Authentication and renewal](docs/AUTHENTICATION.md)
- [Public API and wire models](docs/API.md)
- [Error handling](docs/ERROR_HANDLING.md)
- [Testing](docs/TESTING.md), [test plan](docs/TEST_PLAN.md), and [delivery evidence](docs/DELIVERY_CHECKLIST.md)
- [Implementation notes and provider unknowns](docs/IMPLEMENTATION_NOTES.md)
- [Provider reference snapshot](docs/TELBIZ_API_REFERENCE.md)
- [Contributing](CONTRIBUTING.md), [security](SECURITY.md), [releasing](docs/RELEASING.md), and [changelog](CHANGELOG.md)

Validation uses local mocks only. Live Telbiz compatibility, final delivery, and
crediting have not been verified.
