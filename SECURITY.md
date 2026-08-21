# Security Policy

## Supported versions

Security fixes are provided for the latest release only.

## Built-in protections

- API tokens are encrypted with Windows DPAPI for the current Windows user.
- A token is verified against the official Clash Royale API before it replaces the saved token.
- The saved token is never returned to or displayed by the frontend.
- Backend HTTP clients allow only HTTPS, port 443, and exact official Supercell hosts.
- The embedded interface uses a restrictive Content Security Policy and does not load remote scripts.
- The release package is assembled from an explicit file list and never includes local configuration, databases, credentials, or caches.
- GitHub Actions runs tests and publishes a SHA-256 checksum for release archives.

These controls reduce risk but do not make an unsigned desktop binary universally trusted. Users should download releases only from this repository and verify checksums.

## Report a vulnerability

Please use GitHub's private vulnerability reporting flow: **Security → Advisories → New draft security advisory**. Do not open a public issue for a suspected vulnerability.

Include the affected version, reproduction steps, expected impact, and any suggested mitigation. Never include a real Clash Royale API token, private key, password, or unnecessary personal data.

If private reporting is unavailable, open a minimal issue asking the maintainer to enable a private contact channel without publishing exploit details.

## Scope

Useful reports include token exposure, requests escaping the host allowlist, unintended persistence of search data, unsafe release packaging, or a bypass of local-data deletion. Availability problems in Supercell's service and policy decisions by third parties are outside this project's direct control.
