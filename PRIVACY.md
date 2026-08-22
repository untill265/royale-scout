# Privacy

Last updated: 22 August 2026

Royale Scout is designed as a local-only desktop utility. It has no user accounts, hosted backend, advertising, analytics, telemetry, or search-history service.

## Data stored on the PC

Royale Scout stores the following under `%LOCALAPPDATA%\RoyaleScout`:

- the user's Clash Royale API token, encrypted with Windows Data Protection API (DPAPI) for the current Windows account;
- a local SQLite snapshot of the public Ranked leaderboard, including the current top 250 players' public clan names and badge identifiers;
- cached card images received from Supercell's official CDN.

The interface language is stored in the embedded browser's local storage. Search text, medal counts, candidate selections, and viewed decks are not persisted as history.

All app data can be deleted through **Settings → Delete all local data**. This removes the token, database, and cache and returns the app to first launch.

## Network requests

The desktop app contains a host allowlist and permits its own API/image requests only to:

- `https://api.clashroyale.com` — token verification, public leaderboard data, cards, and public battle logs requested for the selected opponent or visible Top 250 players;
- `https://api-assets.clashroyale.com` — card images referenced by the official API.

The API token is included only in authenticated requests to `api.clashroyale.com`. Nickname fragments, medal counts, and Top 250 filters are matched locally. After a user selects an opponent, that player's public tag is sent to the official API to request the battle log. In Top 250, public tags are requested in small batches only for rows near the viewport; those deck results are held temporarily in memory and are not stored as history.

When the user explicitly selects a link in the app, Windows opens the official developer portal or Fan Content Policy page in the default browser. Those browser visits are governed by the destination site's own privacy terms.

## What the project operator receives

There is no Royale Scout server, so the project operator does not receive API tokens, searches, local databases, or card caches. Supercell receives direct API requests according to its own terms and privacy policy. GitHub may process repository visits, downloads, issues, or discussions according to GitHub's policies.

## Logs and support

The release app does not upload logs. If a user chooses to report a problem, they should remove API tokens, player tags, Windows usernames, and other personal information before attaching text or screenshots.

## Changes

Material privacy changes will be documented in this file and in the relevant release notes.
