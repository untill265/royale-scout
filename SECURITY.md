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
- GitHub Actions runs tests, `npm audit`, and `govulncheck` before packaging.
- CodeQL scans the Go, JavaScript, and TypeScript source on changes and every week.
- The exact release artifact must pass a ClamAV malware scan before publication.
- Every release includes SHA-256 checksums and an SPDX JSON software bill of materials (SBOM).
- GitHub Actions creates signed build-provenance and SBOM attestations for the release ZIP.

These independent layers reduce risk and make the build process auditable. They do not prove that an unsigned desktop binary is universally safe. Users should download releases only from this repository and verify both the checksum and attestation.

## Verify a release

Download the Windows ZIP and `SHA256SUMS.txt` from the same release, then compare the ZIP hash:

```powershell
Get-FileHash .\RoyaleScout-v0.2.1-windows-x64.zip -Algorithm SHA256
```

With [GitHub CLI](https://cli.github.com/) installed, verify that GitHub Actions produced the exact ZIP from this repository:

```powershell
gh attestation verify .\RoyaleScout-v0.2.1-windows-x64.zip --repo untill265/royale-scout
```

The public workflow logs show the dependency audits, CodeQL results, checksum verification, malware scan, and release gate. The accompanying SPDX SBOM can be inspected with standard software-composition-analysis tools.

## Report a vulnerability

Please use GitHub's private vulnerability reporting flow: **Security → Advisories → New draft security advisory**. Do not open a public issue for a suspected vulnerability.

Include the affected version, reproduction steps, expected impact, and any suggested mitigation. Never include a real Clash Royale API token, private key, password, or unnecessary personal data.

If private reporting is unavailable, open a minimal issue asking the maintainer to enable a private contact channel without publishing exploit details.

## Scope

Useful reports include token exposure, requests escaping the host allowlist, unintended persistence of search data, unsafe release packaging, or a bypass of local-data deletion. Availability problems in Supercell's service and policy decisions by third parties are outside this project's direct control.
