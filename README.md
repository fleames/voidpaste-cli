# VoidPaste CLI

Command-line client for [VoidPaste](https://voidpaste.com) — create, fetch, list, and delete pastes against the production API (or any self-hosted origin).

Docs: https://voidpaste.com/docs/cli

## Install

Requires [Go](https://go.dev/dl/) 1.23+.

```bash
go install github.com/fleames/voidpaste-cli/cmd/voidpaste@latest
```

Or build from this repo:

```bash
git clone https://github.com/fleames/voidpaste-cli.git
cd voidpaste-cli
go build -o voidpaste ./cmd/voidpaste
```

Put `voidpaste` on your `PATH`.

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

# Health / identity
voidpaste status
voidpaste whoami
```

Paste create flags: `--visibility`, `--expiration` / `--expires`, `--password`, `--burn`, `--language` / `--lang`, `--title`, `--stdin`, `--json`.

## Development

The VoidPaste monorepo (`fleames/voidpaste`, private) may keep a working copy under `apps/cli` for contributors. **This public repo is the installable source of truth** for `go install` and tagged releases.

```bash
go test ./...
```

## License

MIT
