# VoidPaste CLI

Command-line client for [VoidPaste](https://voidpaste.com).

## Public install (recommended)

The installable Go module lives in the **public** repo
[`fleames/voidpaste-cli`](https://github.com/fleames/voidpaste-cli)
(MIT). Outsiders cannot `go install` from this private monorepo.

```bash
go install github.com/fleames/voidpaste-cli/cmd/voidpaste@v0.3.0

# Optional short alias (same binary)
ln -sf "$(which voidpaste)" "$(dirname "$(which voidpaste)")/vp"
```

Docs: https://voidpaste.com/docs/cli · Integrations: https://voidpaste.com/docs/integrations

Both `voidpaste` and `vp` (when symlinked) run the same CLI. There is no separate
`vp` install path yet — do not invent commands that are not implemented.

## This directory (monorepo)

`apps/cli` is the **development copy** for VoidPaste contributors who have
access to the private monorepo. Keep it in sync with
`github.com/fleames/voidpaste-cli` when changing CLI behavior.

- **Canonical for releases / `go install`:** `fleames/voidpaste-cli`
- **Canonical for day-to-day product work:** this tree (then publish a sync)

### Build locally

```bash
go build -C apps/cli -o voidpaste ./cmd/voidpaste
# Windows: go build -C apps/cli -o voidpaste.exe ./cmd/voidpaste
```

### Publish a sync to the public repo

Copy `cmd/voidpaste/*`, `go.mod` (module path
`github.com/fleames/voidpaste-cli`), and `go.sum` into a checkout of
`fleames/voidpaste-cli`, run `go test ./...`, commit, tag, and push.
Bump the install help string in `commands.go` if it still points at the
private path.

## Auth

1. Sign in at https://voidpaste.com and create a key under **Dashboard → API Keys**.
2. Default scopes (`pastes:read`, `pastes:write`) are required for list/create/delete.

```bash
voidpaste auth login --key vp_live_…
voidpaste whoami
```

Credentials are stored in:

| Platform | Path |
|----------|------|
| Linux/macOS | `~/.config/voidpaste/config.json` |
| Windows | `%AppData%\voidpaste\config.json` |

Override with env:

| Variable | Meaning |
|----------|---------|
| `VP_API_KEY` | Bearer API key (preferred) |
| `VP_TOKEN` | Alias of `VP_API_KEY` |
| `VP_API` | API origin (default `https://voidpaste.com`) |
| `VP_CONFIG_DIR` | Config directory override |

Self-hosters: `voidpaste --api https://paste.example.com …` or store the origin via `voidpaste auth login --api https://paste.example.com --key …`.

## Commands

```bash
# Anonymous or authenticated create
echo 'hello' | voidpaste create --language go --expiration 1d
voidpaste create ./notes.md --title "Notes" --visibility unlisted

# Fetch
voidpaste get PASTE_ID
voidpaste raw PASTE_ID
voidpaste download PASTE_ID -o out.txt
voidpaste raw PASTE_ID --password 'secret'   # password pastes

# Account pastes (API key required)
voidpaste list
voidpaste delete PASTE_ID --yes

# Collections / versions (API key required)
voidpaste collection list
voidpaste collection create --name "Snippets" --visibility private
voidpaste collection add COLLECTION_ID PASTE_ID
voidpaste versions list PASTE_ID
voidpaste versions restore PASTE_ID 2

# Health / identity
voidpaste status
voidpaste whoami
```

Paste create flags: `--visibility`, `--expiration` / `--expires`, `--password`, `--burn`, `--language` / `--lang`, `--title`, `--stdin`, `--quiet` / `-q` (URL only), `--idempotency-key`, `--json` (`{paste, warnings}`).

## API surface used

| CLI | HTTP |
|-----|------|
| `status` | `GET /health`, optional `GET /api/v1/auth/me` |
| `whoami` | `GET /api/v1/auth/me` |
| `create` | `POST /api/v1/pastes` |
| `get` | `GET /api/v1/pastes/{id}` |
| `raw` | `GET /raw/{id}` |
| `download` | `GET /download/{id}` |
| `list` | `GET /api/v1/me/pastes` |
| `delete` | `DELETE /api/v1/pastes/{id}` |
| `collection *` | `/api/v1/collections…` |
| `versions *` | `/api/v1/pastes/{id}/versions…` |

Auth header: `Authorization: Bearer vp_live_…`. Password pastes: `X-Paste-Password`.

Not implemented (no fake stubs): billing, moderation — use the [API docs](https://voidpaste.com/docs/api) or web dashboard.

## Tests

```bash
cd apps/cli && go test ./...
```
