# Security

## Reporting a vulnerability

Do not report sensitive vulnerabilities through public issues. Use the
repository's GitHub **Security → Report a vulnerability** feature when enabled.
If it is unavailable, obtain a private reporting channel from the repository
owner before transmitting vulnerability details. Do not invent an owner email.

Include the affected SDK version, a minimal reproduction using fabricated inputs,
impact, and mitigation suggestions. Never include credentials, access tokens,
real phone numbers, or SMS/OTP content.

## Supported versions

This implementation does not establish a published-release support policy. The
maintainer must update this section when releases and maintenance policy exist.
The locally verified toolchain is Go 1.27.1. No response time or support commitment
is implied.

## Credential and transport handling

Use Client ID and Secret only in trusted backend environments. Keep credentials
out of source control and logs. Access-token renewal does not replace or revoke
the Client ID or Secret. Credential rotation is managed through Telbiz/account
administration, outside the documented SDK token lifecycle.

Use HTTPS for production. Custom HTTP clients/transports must preserve TLS
verification, honor cancellation, and avoid replaying SMS/top-up POSTs. Treat
provider free-text error fields as potentially sensitive. Release examples and
CI must use placeholders and mock endpoints.
