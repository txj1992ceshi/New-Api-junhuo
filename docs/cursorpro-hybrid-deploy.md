# CursorPro Hybrid Deploy

This note describes the current recommended deployment shape for the
`new-api + CursorPro 3` integration:

- Public frontend on a server
- `new-api` backend on the local mac that also runs CursorPro 3

## Recommended topology

- Frontend host: Hong Kong server
- Backend host: local macOS machine
- CursorPro 3 host: same local macOS machine as `new-api`

This keeps the Codex export directory and the CursorPro replacement control
service on the same machine as the token pool logic.

## Backend environment

Configure these values on the mac that runs `new-api`:

```env
PORT=3000
CURSORPRO_CODEX_EXPORT_DIR=~/Library/Application Support/CursorPro3/exports/codex
CURSORPRO_CONTROL_URL=http://127.0.0.1:18765
```

Optional but strongly recommended for public use:

```env
FRONTEND_BASE_URL=https://app.example.com
```

Also set `ServerAddress` inside the admin settings UI to the public API origin,
for example:

```text
https://api.example.com
```

## Frontend build configuration

The frontend already supports a dedicated API base URL through Vite.

Create a production env file on the frontend build machine:

```env
VITE_REACT_APP_SERVER_URL=https://api.example.com
```

Then build:

```bash
cd web
npm run build
```

Deploy the generated `web/dist` contents to the Hong Kong server.

## Public exposure

The frontend can be served from the Hong Kong server directly.

The local mac backend must be reachable from the public frontend. In the
current phase, use one of these:

- Cloudflare Tunnel
- Tailscale Funnel
- reverse proxy + public IP + HTTPS

The backend and CursorPro 3 should remain on the same mac host.

## Pre-launch checks

Before opening access to users, verify:

1. `GET /api/channel/:id/codex/pool_health` returns live data
2. `GET /api/channel/:id/codex/replacement_status` reaches the local control service
3. the CursorPro export directory updates after a replacement trigger

## Git workflow recommendation

Push your modified `new-api` to your own GitHub fork or private repository
before public deployment.

Recommended order:

1. commit local changes
2. push to your own GitHub repository
3. deploy from that tracked revision
4. smoke test the production path

Do not push directly to the upstream `QuantumNous/new-api` remote unless that
is explicitly intended.
