# Public API

Import path: `github.com/barlus-developer/golang-sdk-telbiz`, package `telbiz`.
Go 1.27.1 is the module minimum. Authentication and transport coordination remain
private; there are no exported token strings or clock hooks.

## Client and configuration

```go
func New(ctx context.Context, cfg Config) (*Client, error)
func (*Client) SendSMS(ctx context.Context, req SMSRequest) (*SMSResponse, error)
func (*Client) Topup(ctx context.Context, req TopupRequest) (*TopupResponse, error)
func (*Client) RefreshToken(ctx context.Context) error
func (*Client) Close() error
```

`New` validates and eagerly authenticates; successful clients renew automatically.
Service methods return a nonnil response only on trustworthy acceptance. They
perform one service submission per invocation. `RefreshToken` joins/forces one
renewal subject to cooldown. `Close` is idempotent. Clients support concurrent use;
their zero value is unusable and initialized clients must not be copied.

`Config` fields: `ClientID string`, `ClientSecret string`, `BaseURL string`,
`HTTPClient *http.Client`, `RefreshBefore time.Duration`.
See [configuration defaults and ownership](USAGE.md#configuration).

Constants: `DefaultBaseURL = "https://api.telbiz.la"`,
`RequestTimeout = 30 * time.Second`.

## Request and result models

```go
type SMSTitle string
// TitleOTP="OTP", TitleInfo="Info",
// TitlePromotion="Promotion", TitleNews="News".

type SMSRequest struct {
    Title SMSTitle `json:"title"`
    Phone string `json:"phone"`
    Message string `json:"message"`
}
type TopupRequest struct {
    Phone string `json:"phone"`
    Amount int64 `json:"amount"`
}
type Result struct {
    Code string `json:"code"`
    Message string `json:"message"`
    Success bool `json:"success"`
    Detail string `json:"detail"`
}
type TransactionKey struct {
    PartitionKey string `json:"partitionKey"`
    RangeKey string `json:"rangeKey"`
}
type SMSResponse struct {
    Response Result `json:"response"`
    Key TransactionKey `json:"key"`
}
type TopupResponse struct {
    Response Result `json:"response"`
    Key TransactionKey `json:"key"`
}
```

`Result.Success` means accepted/in progress, not confirmed delivery or airtime
credit. Provider message/detail fields may contain sensitive text. Unknown codes
are preserved. Empty transaction-key values are valid; uniqueness and lookup
semantics are unspecified. See [validation rules](USAGE.md#request-rules).

| Exported code constant | Value |
|---|---|
| `CodeSuccessOperation` | `SUCCESS_OPERATION` |
| `CodeSendingSMSInProgress` | `SENDING_SMS_INPROGRESS` |
| `CodeTopupBalanceInProgress` | `TOPUP_BALANCE_INPROGRESS` |
| `CodeUnsupportedHeader` | `UNSUPPORTED_HEADER` |
| `CodeSendSMSFail` | `SEND_SMS_FAIL` |
| `CodeRequestFail` | `REQUEST_FAIL` |

## Errors

Sentinels: `ErrValidation`, `ErrClosed`, `ErrAuthentication`, `ErrProvider`,
`ErrTransport`, and `ErrMalformedResponse`. Use `errors.Is`.

`Error` has fields `Kind error`, `Operation string`, `Field string`,
`HTTPStatus int`, `Code string`, `Message string`, `Detail string`,
`OutcomeUnknown bool`. `Field` identifies an invalid configuration/request field;
`HTTPStatus` is zero when no response status is available. `Kind` is a sentinel
category. Methods: `Error() string`, `Is(target error) bool`, `Unwrap() error`.
The error string contains only a fixed category description. `Unwrap` retains
underlying context/transport causes; these causes can themselves contain sensitive
text. An authentication error wraps its underlying category, so both
`errors.Is(err, ErrAuthentication)` and an underlying match can succeed.
See [error handling](ERROR_HANDLING.md) for complete inspection guidance.

## Provider wire contract

| Operation | Method | Exact path | JSON request |
|---|---|---|---|
| Authentication | POST | `/api/v1/Connect/token` | `clientID`, `secret`, `grantType` |
| SMS | POST | `/api/v1/SMSService/newtransaction` | `title`, `phone`, `message` |
| Airtime | POST | `/api/v1/TopupService/newtransaction` | `phone`, numeric `amount` |

Requests have `Content-Type: application/json`. Service requests use
`Authorization: bearer <accessToken>`. Auth success is flat:
`accessToken`, positive integer `expire` in seconds, `code`, `message`, `success`.
Service success is nested: `response` containing result fields plus `key`.
Service failures are flat: `code`, `message`, `success:false`, `detail`.

The parser distinguishes absent success from explicit false. HTTP 200 with
`success:false` is an error. Successful service responses must contain the nested
response and a nonnull key object. Non-2xx statuses always produce errors,
including redirects. Malformed successful responses have unknown outcomes.

Only these documented operations are implemented. There is no refresh-token,
data-package, webhook, final-status, sandbox, delivery-query, or idempotency API.
See the [supplied provider reference](TELBIZ_API_REFERENCE.md) for examples and
source conflicts, and [implementation notes](IMPLEMENTATION_NOTES.md) for policy.
