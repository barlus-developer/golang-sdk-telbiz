# Changelog

## Unreleased

- Change `New` to `New(config ...Config) *Client`: no network request, environment
  credentials only when called without config, deferred configuration errors,
  shared first-use authentication, and safe Close before/during initialization.
- Add `NewWithContext(ctx, config) (*Client, error)` for immediate authentication;
  migrate existing `New(ctx, config)` calls to this constructor for eager behavior.
- Add `Config.Timeout` for every HTTP request (zero defaults to 30 seconds;
  negative values invalid), including injected clients without mutation.
  Positional Config literals must migrate to named fields.
- Implement the unofficial `telbiz` client with lazy/eager authentication, cached
  tokens, idle renewal, coordinated manual/request/background refresh, bounded
  failure backoff, and concurrent idempotent shutdown.
- Add typed SMS headings, validated SMS and whole-LAK airtime top-up submissions,
  nested/flat response parsing, preserved provider fields and transaction keys,
  safe inspectable errors, and unknown-outcome reporting.
- Bound HTTP calls and responses; disable redirects and automatic service replay.
- Add local mock and deterministic lifecycle tests, runnable environment-based
  examples, package/usage/API/auth/error documentation, CI, and delivery evidence.

No release version or publication is recorded here.
