# Contributing

Thanks for improving Royale Scout.

1. Do not commit API tokens, local databases, cached images, logs, or screenshots containing personal information.
2. Create a focused branch and keep changes small enough to review.
3. Run `npm ci --prefix frontend`, `npm run build --prefix frontend`, `go test ./...`, and `go vet ./...` on Windows.
4. Explain user-visible changes and relevant tests in the pull request.
5. Use private vulnerability reporting for security issues as described in `SECURITY.md`.

For UI-only checks, `npm run dev --prefix frontend` supports `?preview=setup`, `?preview=ready`, and `?preview=error`. Preview responses exist only in Vite development mode and are removed from production builds.

Contributions must comply with Supercell's Fan Content Policy. Do not add automation that controls the game client, screen capture, or claims of official affiliation.
