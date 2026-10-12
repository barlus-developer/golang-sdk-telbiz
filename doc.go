// Package telbiz is an unofficial client for Telbiz authentication, SMS submission
// and mobile airtime top-up. It uses the provider contract recorded in
// docs/TELBIZ_API_REFERENCE.md; mock tests do not establish live compatibility.
//
// New authenticates eagerly. The client caches its access token and renews it
// proactively with the same credentials, including while idle. Renewal does not
// rotate the Client ID or Secret. Share a client across goroutines and call Close
// when finished. The constructor context governs initialization only.
//
// Every HTTP request is bounded by RequestTimeout. Service POSTs are submitted
// once, with redirects disabled. A 401 affects future token acquisition but never
// replays the current service operation. Accepted results do not confirm delivery
// or airtime credit. Inspect Error.OutcomeUnknown before reconciling ambiguous
// outcomes; the provider snapshot contains no reconciliation endpoint.
package telbiz
