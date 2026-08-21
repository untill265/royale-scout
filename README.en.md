# Royale Scout

**Royale Scout** is a local Windows app for manually finding a Clash Royale opponent by Ranked medals and a nickname fragment. It displays up to three recent Ranked decks and highlights true spells separately.

[![CI](https://github.com/untill265/royale-scout/actions/workflows/ci.yml/badge.svg)](https://github.com/untill265/royale-scout/actions/workflows/ci.yml)
[![CodeQL](https://github.com/untill265/royale-scout/actions/workflows/codeql.yml/badge.svg)](https://github.com/untill265/royale-scout/actions/workflows/codeql.yml)
[![Verified Release](https://github.com/untill265/royale-scout/actions/workflows/release.yml/badge.svg)](https://github.com/untill265/royale-scout/actions/workflows/release.yml)

[Русская версия](README.md)

## Highlights

- no Royale Scout accounts or hosted backend;
- no OCR, screen capture, game control, or modification of the Clash Royale client;
- no analytics or search history;
- the personal API token is encrypted with Windows DPAPI and stays on the user's PC;
- network requests are restricted to `api.clashroyale.com` and `api-assets.clashroyale.com`;
- Russian and English UI.

## Install

1. Open **Releases** in this repository.
2. Download the Windows x64 ZIP and `SHA256SUMS.txt` from the same release.
3. Optionally verify the checksum with `Get-FileHash` in PowerShell.
4. Extract the ZIP and run `RoyaleScout.exe`.

The app is not yet signed with a paid code-signing certificate, so Microsoft Defender SmartScreen may warn about an unknown publisher. Download only from this repository's Releases and verify the SHA-256 checksum.

For an additional provenance check, install [GitHub CLI](https://cli.github.com/) and run:

```powershell
gh attestation verify .\RoyaleScout-v0.2.0-windows-x64.zip --repo untill265/royale-scout
```

Every release also includes an `sbom.spdx.json` file containing a machine-readable inventory of build components and dependencies.

## First launch and API token

The app needs a free personal key from the official Clash Royale API. It does not need your game login, password, or player tag.

1. Open the [Clash Royale Developer Portal](https://developer.clashroyale.com/).
2. Sign in or create an account.
3. In **My Account**, choose **Create New Key**.
4. Add your current public IPv4 to the allowed addresses and create the key.
5. Copy the complete token, paste it into Royale Scout, and select **Verify and save**.

The app verifies the key directly with the official API and encrypts it for the current Windows account. A saved token is never displayed again; it can only be replaced in **Settings**.

If your public IP changes, update the key on the portal or create a new key, then use **Settings → Replace token**.

## Privacy

See [PRIVACY.md](PRIVACY.md) for the complete data-flow summary. Search text is not stored. The ranking database, card cache, and encrypted token are kept in `%LOCALAPPDATA%\RoyaleScout` and can be removed from inside the app.

Every release must pass tests, `npm audit`, `govulncheck`, and a ClamAV scan of the finished archive. Source changes are also analyzed by CodeQL. GitHub signs the ZIP provenance attestation, while SHA-256 checksums and an SBOM are published with the download. All steps are visible in **Actions**.

These checks improve transparency but are not an absolute security guarantee. Security reporting and design notes are in [SECURITY.md](SECURITY.md). Never post your API token in an issue, screenshot, or log.

## Build from source

Windows 10/11 x64, Go 1.25+, Node.js 24+, npm, and Wails CLI 2.15.0 are required.

```powershell
npm ci --prefix frontend
npm run build --prefix frontend
go test ./...
go vet ./...
wails build -clean -trimpath -platform windows/amd64 -o RoyaleScout.exe
```

Run `.\scripts\package-release.ps1 -Version 0.2.0` to create a local release ZIP and checksum.

## Legal

Royale Scout is an unofficial fan project. It uses the official API and does not modify or control the game. Tournament use depends on each organiser's rules.

This material is unofficial and is not endorsed by Supercell. For more information see [Supercell's Fan Content Policy](https://supercell.com/en/fan-content-policy/).

Royale Scout source code is available under the [MIT License](LICENSE). Supercell names, card images, and other assets remain the property of their respective owners and are not licensed by this repository.
