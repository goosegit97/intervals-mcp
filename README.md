# intervals-mcp

A single-user, self-hostable [Model Context Protocol](https://modelcontextprotocol.io) server
wrapping the [Intervals.icu](https://intervals.icu) API. One deployment serves one athlete's
account. Read tools are unrestricted; the write tools are single-id and confirm-gated.

It began as one service inside the `gaggle` monorepo and was extracted to stand alone: the
multi-tenant OAuth2 front door is replaced by a single shared bearer token, so the whole thing is
one Go binary plus a slim Ansible deploy.

## Tools

| Tool | Kind | Notes |
|------|------|-------|
| `get_athlete_profile` | read | |
| `get_activities` | read | |
| `get_activity_detail` | read | |
| `get_events` | read | calendar events |
| `get_wellness` | read | |
| `create_workout` | write | idempotent via `external_id` |
| `update_event` | write | read-modify-write; `confirm`/`dry_run` |
| `delete_event` | write | single-id; fetch-then-`confirm` |

The write tools mutate a real Intervals.icu calendar and an LLM is the caller, so they hold a
strict safety posture: single-id only (no bulk/range), confirm-before-mutate with `dry_run`
support, read-modify-write updates, and test only on far-future disposable data.

## Build & test

```sh
go build ./...
go vet ./...
go test ./...
```

## Run locally (stdio)

With no `INTERVALS_LISTEN_ADDR` set, the server speaks MCP over stdio — the usual mode for a local
MCP client. Copy `.env.example` to `.env` and fill in your Intervals.icu API key + athlete id:

```sh
go run ./cmd/intervals
```

## Run over HTTP (bearer token)

Set `INTERVALS_LISTEN_ADDR` and `MCP_BEARER_TOKEN` to serve Streamable HTTP. The listener binds
loopback; put TLS-terminating Caddy (or similar) in front. Every request must carry
`Authorization: Bearer <token>`; `/healthz` is unauthenticated for probes.

```sh
INTERVALS_LISTEN_ADDR=127.0.0.1:8081 \
MCP_BEARER_TOKEN=$(openssl rand -hex 32) \
INTERVALS_API_KEY=... INTERVALS_ATHLETE_ID=i123456 \
go run ./cmd/intervals
```

## Environment

| Var | Required | Meaning |
|-----|----------|---------|
| `INTERVALS_LISTEN_ADDR` | no | e.g. `127.0.0.1:8081`; unset ⇒ stdio |
| `MCP_BEARER_TOKEN` | HTTP mode | shared bearer token clients must present |
| `INTERVALS_API_KEY` | yes | Intervals.icu API key (HTTP Basic password) |
| `INTERVALS_ATHLETE_ID` | yes | athlete id, e.g. `i123456` (`0` = authenticated athlete) |

## Deploy

A hardened Debian 12 VPS deploy lives in `ansible/` (base + DevSec hardening + firewall + Caddy +
the one app service). Stage the linux binary, then run the playbook:

```sh
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o ansible/roles/app/files/intervals ./cmd/intervals
cd ansible
ansible-galaxy collection install -r requirements.yml -p collections
# copy inventory.ini.example -> inventory.ini and fill in the box;
# create the encrypted vault (see group_vars/vault.example.yml):
ansible-vault create group_vars/all/vault.yml
ansible-playbook site.yml            # full provision
ansible-playbook site.yml --tags app # app-only refresh
```

Secrets never enter git: the API key, athlete id, and bearer token live in ansible-vault
(`group_vars/all/vault.yml`), deployed as a 0600 systemd `EnvironmentFile`.
