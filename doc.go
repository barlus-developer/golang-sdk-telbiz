// Package telbiz is an unofficial client for Telbiz authentication, SMS submission
// and mobile airtime top-up. It uses the provider contract recorded in
// docs/TELBIZ_API_REFERENCE.md; mock tests do not establish live compatibility.
//
// New accepts zero or one Config without network activity. New() captures
// TELBIZ_CLIENT_ID and TELBIZ_CLIENT_SECRET; New(config) uses explicit credentials
// without environment fallback. Operations report deferred configuration errors
// and authenticate on first token use. Concurrent callers share authentication;
// canceling one caller cancels only its wait. NewWithContext validates and
// authenticates immediately with an initialization-only context.
//
// After the first successful authentication, the client renews while idle,
// normally 60 seconds before expiry, using the same credentials. Renewal does
// not rotate the Client ID or Secret. Share clients across goroutines and call
// Close when finished, including before first use. Close is idempotent and
// prevents later authentication.
//
// Config.Timeout bounds each HTTP request, including injected clients; zero uses
// RequestTimeout (30 seconds). Earlier caller deadlines win. Service POSTs submit
// once, with redirects disabled. A 401 affects future token acquisition but never
// replays the current service operation. Accepted results do not confirm delivery
// or airtime credit. Inspect Error.OutcomeUnknown before reconciling ambiguous
// outcomes; the provider snapshot contains no reconciliation endpoint.
package telbiz
