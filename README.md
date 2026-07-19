# intervals-mcp — Intervals.icu MCP server for Claude & other AI assistants

> A fast, self-hosted **[Model Context Protocol (MCP)](https://modelcontextprotocol.io)** server for
> **[Intervals.icu](https://intervals.icu)**. Give Claude, Claude Code, and any MCP-compatible AI
> assistant safe read/write access to your training calendar, planned workouts, activities, and
> wellness data — running entirely on your own device.

![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)
![Model Context Protocol](https://img.shields.io/badge/MCP-compatible-6E56CF)
![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Linux%20%7C%20Windows-lightgrey)
![Status](https://img.shields.io/badge/status-stable-brightgreen)
![License](https://img.shields.io/badge/license-MIT-blue)

Ask your AI assistant things like *"how did my training load trend this week?"*, *"add a 3×10min
threshold bike workout to Thursday"*, or *"summarise my sleep and HRV before tomorrow's session"* —
and it talks to your real Intervals.icu account through this server.

---

## Contents

- [Why intervals-mcp](#why-intervals-mcp)
- [What you can do](#what-you-can-do-tools)
- [Requirements](#requirements)
- [Install](#install)
- [Get your Intervals.icu API key](#get-your-intervalsicu-api-key)
- [Use it on your device (Claude Desktop / Claude Code)](#use-it-on-your-device)
- [Configuration reference](#configuration-reference)
- [The write tools are safe by design](#the-write-tools-are-safe-by-design)
- [Self-host over HTTPS (optional)](#self-host-over-https-optional)
- [Build from source & test](#build-from-source--test)
- [FAQ](#faq)

---

## Why intervals-mcp

- **Runs on your machine.** Your API key never leaves your device; there is no third-party cloud in
  the middle.
- **One athlete, zero setup ceremony.** No OAuth server, no database — just an API key and an
  athlete id.
- **Read *and* write.** Not just dashboards: your assistant can create planned workouts and manage
  calendar events, with strict safety rails (see below).
- **Single static binary.** Written in Go; no runtime, no dependencies to install.
- **Works with any MCP client** — Claude Desktop, Claude Code, and other Model Context Protocol
  hosts.

## What you can do (tools)

| Tool | Kind | What it does |
|------|------|--------------|
| `get_athlete_profile` | read | Your athlete profile & settings |
| `get_activities` | read | Recent activities (rides, runs, swims…) |
| `get_activity_detail` | read | Full detail for one activity |
| `get_events` | read | Calendar events (planned workouts, notes, races) |
| `get_wellness` | read | Wellness metrics (sleep, HRV, resting HR, fatigue…) |
| `create_workout` | write | Add a planned/structured workout to your calendar |
| `update_event` | write | Modify a calendar event (confirm-gated) |
| `delete_event` | write | Delete a single calendar event (confirm-gated) |

## Requirements

- An **[Intervals.icu](https://intervals.icu)** account and its **API key** (free — see below).
- Either **[Go 1.25+](https://go.dev/dl/)** to install with one command, **or** a prebuilt binary.
- An MCP client such as **[Claude Desktop](https://claude.ai/download)** or
  **[Claude Code](https://claude.com/claude-code)**.

## Install

### Option A — `go install` (recommended)

```sh
go install github.com/goosegit97/intervals-mcp/cmd/intervals@latest
```

This drops an `intervals` (or `intervals.exe` on Windows) binary in your `$(go env GOPATH)/bin`.
Add that directory to your `PATH` if it isn't already.

### Option B — build from source

```sh
git clone https://github.com/goosegit97/intervals-mcp.git
cd intervals-mcp
go build -o intervals ./cmd/intervals
```

## Get your Intervals.icu API key

1. Sign in to [intervals.icu](https://intervals.icu).
2. Go to **Settings** → **Developer** and copy your **API key**.
3. Note your **athlete id** — it's the `iNNNNNN` value in your profile URL
   (`https://intervals.icu/athlete/i123456/...`). You can also use `0` for the authenticated
   athlete.

Keep the API key private — anyone with it can read and write your Intervals.icu data.

## Use it on your device

The server speaks MCP over **stdio** by default, which is exactly what desktop MCP clients expect —
no ports, no tokens, no network exposure.

### Claude Desktop

Open your Claude Desktop config file:

- **macOS:** `~/Library/Application Support/Claude/claude_desktop_config.json`
- **Windows:** `%APPDATA%\Claude\claude_desktop_config.json`
- **Linux:** `~/.config/Claude/claude_desktop_config.json`

Add an `intervals` server (use the **full path** to the binary from the install step):

```json
{
  "mcpServers": {
    "intervals": {
      "command": "/Users/you/go/bin/intervals",
      "env": {
        "INTERVALS_API_KEY": "your-api-key-here",
        "INTERVALS_ATHLETE_ID": "i123456"
      }
    }
  }
}
```

Save, fully **restart Claude Desktop**, and you'll see the intervals tools appear. Ask *"what does
my Intervals.icu calendar look like this week?"* to confirm it's wired up.

### Claude Code

One command registers it (stdio, local):

```sh
claude mcp add intervals \
  --env INTERVALS_API_KEY=your-api-key-here \
  --env INTERVALS_ATHLETE_ID=i123456 \
  -- intervals
```

Then run `claude` and your assistant can use the tools immediately.

### Try it locally first (optional)

```sh
INTERVALS_API_KEY=... INTERVALS_ATHLETE_ID=i123456 intervals
```

With no `INTERVALS_LISTEN_ADDR` set it serves over stdio and waits for an MCP client — that's the
normal, correct behaviour.

## Configuration reference

| Environment variable | Required | Meaning |
|----------------------|----------|---------|
| `INTERVALS_API_KEY` | ✅ | Your Intervals.icu API key (HTTP Basic password) |
| `INTERVALS_ATHLETE_ID` | ✅ | Athlete id, e.g. `i123456` (`0` = authenticated athlete) |
| `INTERVALS_LISTEN_ADDR` | — | Set (e.g. `127.0.0.1:8081`) to serve HTTP instead of stdio |
| `MCP_BEARER_TOKEN` | HTTP mode | Shared bearer token clients must present over HTTP |

A local `.env` file is also read in development — copy `.env.example` to `.env`.

## The write tools are safe by design

The write tools mutate your **real** Intervals.icu calendar, and an LLM is the caller, so they hold
a deliberately strict posture:

- **Single-id only** — no bulk or date-range deletes/updates, ever.
- **Confirm before mutating** — `update_event` / `delete_event` fetch and return the current event
  first, and only apply with an explicit `confirm=true`. They support `dry_run=true`.
- **Read-modify-write** updates so fields are never silently blanked.
- **Idempotent creates** via `external_id`.

## Self-host over HTTPS (optional)

Prefer to run it on a VPS and connect a remote MCP client? Set `INTERVALS_LISTEN_ADDR` and
`MCP_BEARER_TOKEN` to serve Streamable HTTP behind a bearer token (bind loopback; terminate TLS
with Caddy or similar). A hardened Debian 12 Ansible deploy — base + DevSec hardening + firewall +
Caddy + this one service — lives in [`ansible/`](ansible/). Every request then needs
`Authorization: Bearer <token>`; `/healthz` stays open for probes.

## Build from source & test

```sh
go build ./...
go vet ./...
go test ./...
```

## FAQ

**Is my API key sent anywhere?** No. In the default stdio mode the binary runs locally and talks
only to `intervals.icu` over HTTPS. Nothing is sent to any third party.

**Does it work with ChatGPT / other assistants?** Any host that supports the Model Context Protocol
can use it. Setup mirrors the Claude examples above.

**Can it delete my data by accident?** The write tools are single-id and confirm-gated, and there
is no bulk-delete path — see [the safety section](#the-write-tools-are-safe-by-design).

---

## License

Released under the [MIT License](LICENSE).

---

<sub>Keywords: Intervals.icu MCP server · Model Context Protocol · Claude · Anthropic · training
calendar · structured workouts · cycling · running · triathlon · wellness · HRV · self-hosted · Go.</sub>
