# Testing

Use Go 1.27.1, matching `go.mod` and the locally verified toolchain. There are no
external test dependencies. Tests use `httptest` local servers and fabricated
credentials/inputs; no Telbiz traffic or billable requests are made.

```bash
gofmt -l $(git ls-files -- '*.go')
go test ./...
go test -race ./...
go vet ./...
go build ./examples/...
```

Formatting output must be empty. Before committing newly created files, also
check untracked Go sources, for example `gofmt -l .`. CI checks all tracked Go
sources and fails on formatting output. Example builds compile programs; they
do not execute their authentication or billable operations. The runnable package
example uses a local mock and is executed by `go test`.

## Deterministic lifecycle tests

`lifecycle_test.go` uses standard-library `testing/synctest` with fake time and
in-memory RoundTrippers. Clock advances and `synctest.Wait` synchronize workers
without sleeping through real token lifetimes. No testing-only public hooks are
exported. `client_test.go` also uses real local servers for request/response wire,
redirect, and connection-loss tests.

| Contract from [TEST_PLAN](TEST_PLAN.md) | Test evidence |
|---|---|
| Constructor/schema/expiry validation | `TestWireContract`, `TestConfigurationValidation`, `TestConstructorResponses`, `TestContextTimeouts` |
| Default/configurable/short-lifetime idle renewal | `TestIdleRenewalSchedule`, `TestRenewalUsesNewLifetime` |
| Conservative expiry, delayed-auth scheduling, and expiry at installation | `TestSlowAuthenticationScheduling`, `TestExpiredTokenNotInstalled` |
| Old token remains usable, atomic replacement | `TestSharedRenewalAndWaitCancellation`, `TestTransientFailureBackoff`, `TestIdleRenewalSchedule` |
| Background/manual/service single shared auth | `TestSharedRenewalAndWaitCancellation` |
| Caller wait cancellation | `TestSharedRenewalAndWaitCancellation`, `TestManualCancellationDoesNotCancelSharedRenewal` |
| Cooldown, no expiry spin, sequential/concurrent callers | `TestTransientFailureBackoff`, `TestBackoffCapAndAuthenticationPolicy` |
| Terminal rejection and manual recovery | `TestTerminalFailureAndExplicitRecovery` |
| Old 401/current 401 handling; generic rejections retain token | `TestLate401AndFutureRefresh`, `TestServiceRejectionsDoNotInvalidateTokens` |
| Routes, JSON names, amount, headings, message preservation | `TestWireContract`, `TestInputValidation` |
| Flat/nested envelopes, unknown codes, empty keys | `TestServiceEnvelopes` |
| Malformed/large responses and safe causes/errors | `TestConstructorResponses`, `TestResponseCleanupAndSafeErrors`, `TestContextTimeouts` |
| One submission on errors/redirects/connection loss | `TestNoServiceReplay`, `TestConnectionLossNoReplay`, `TestContextTimeouts` |
| Redirect security/client immutability | `TestAuthenticationRedirect`, `TestNoServiceReplay` |
| Body closure and caller resource ownership | `TestResponseCleanupAndSafeErrors`, `TestAuthenticationAttemptBodiesClose`, `TestCallerTransportNotClosed` |
| Close and in-flight waiters/services; expiry/closure at admission | `TestCloseInFlightAndConcurrentWaiters`, `TestCloseAllowsAlreadyStartedServiceToFinish`, `TestServiceAdmissionChecksExpiryAndClosure` |
| Executable package example | `ExampleClient_SendSMS` |

The configured [CI workflow](../.github/workflows/ci.yml) runs formatting, tests,
race detection, vet, and example compilation. Its Go version comes from `go.mod`.
Do not supply provider credentials in CI. No remote CI run is claimed by local
verification. See [delivery evidence](DELIVERY_CHECKLIST.md) for actual commands,
environment restrictions, and results. Mock success is not live compatibility.
