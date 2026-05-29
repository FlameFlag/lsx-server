<p align="center">
  <img src="go/web/static/project/lt2_logo_text_only.avif" alt="Lemonade Tycoon 2 New York Edition" width="520">
</p>

<h1 align="center">LSX Server</h1>

<p align="center">
  <strong>A fan preservation server for the Lemonade Tycoon 2 Lemonade Stock Exchange.</strong><br>
  Recreates the old game client's account checks, score uploads, LSX pages, leaderboard data, admin cleanup, live TUI, and Discord alerts.
</p>

<p align="center">
  <img src=".github/lemonade-tycoon-2-lemon-faces.png" alt="" width="320">
</p>

> [!IMPORTANT]
> The original game client expects plain HTTP on TCP `80` and the host `gt.jamdat.ca`. If `/syncgame.php` returns text containing `SUCCESS`, the client treats the upload as accepted.

## What It Does

Lemonade Tycoon 2 still contains client code for the old Lemonade Stock Exchange, but the original endpoints are gone. This fan project recreates the parts the game still calls: account checks, score syncs, the LSX leaderboard, the connection image, and recovered score-detail pages.

It keeps the preservation work close to the running service. The server stores uploads in SQLite, exposes a small JSON API, includes admin cleanup tools, can send Discord alerts, and documents the recovered client behavior in the project findings page.

<table>
  <tr>
    <th width="25%" align="center">Game Client</th>
    <th width="25%" align="center">LSX Pages</th>
    <th width="25%" align="center">Findings</th>
    <th width="25%" align="center">Operations</th>
  </tr>
  <tr>
    <td width="25%" align="center">
      <img src="go/web/static/project/lt2_icon_play.avif" alt="" width="96"><br>
      Recreates the endpoints and tiny assets the recovered game client expects.
    </td>
    <td width="25%" align="center">
      <img src="go/web/static/project/lt2_icon_lsx.avif" alt="" width="96"><br>
      Serves recovered LSX pages and stores fresh score uploads in SQLite.
    </td>
    <td width="25%" align="center">
      <img src="go/web/static/project/lt2_icon_findings.avif" alt="" width="96"><br>
      Documents the protocol, checksum, date scalar, upload queue, and page shape.
    </td>
    <td width="25%" align="center">
      <img src="go/web/static/admin/pitcher.avif" alt="" width="96"><br>
      Provides admin cleanup, a live TUI, and Discord sync alerts.
    </td>
  </tr>
</table>

## Quick Start

```sh
git clone https://github.com/FlameFlag/lsx_server_go.git
cd lsx_server_go
cd go
go run . --addr 127.0.0.1:8080
```

Open `http://127.0.0.1:8080/`.

Configuration can also come from environment variables. On startup, the server loads a local `.env` file when one exists, then command-line flags override those values.

```sh
cp ../.env.example ../.env
LSX_ADDR=127.0.0.1:8080 go run .
```

Run with the plain terminal UI or seed data:

```sh
go run . --plain --addr 127.0.0.1:8080
go run . --seed --addr 127.0.0.1:8080
```

Use the build scripts for release binaries. They generate embedded browser assets, run checks by default, and write optimized binaries under `dist/`.

```sh
./build.sh
./build.sh --target linux/amd64
./build.sh --target windows/amd64 --skip-checks
```

On Windows PowerShell:

```powershell
.\build.ps1
.\build.ps1 -Target windows/amd64
.\build.ps1 -Target linux/amd64 -SkipChecks
```

To use the original game client, run the server where the game can reach it as `gt.jamdat.ca` on port `80`. For a local test, run:

```sh
./lsx-server --addr 127.0.0.1:8080 --data ./data/lsx.sqlite3
```

Then point `gt.jamdat.ca` at this machine and forward TCP `80` to the local port if needed.

## VPS Deploy

The Docker setup listens on port `80` by default, which matches the original game client.

```sh
git clone https://github.com/FlameFlag/lsx_server_go.git
cd lsx_server_go
cp .env.example .env
docker compose up -d --build
docker compose ps
curl http://127.0.0.1/healthz
```

Open inbound TCP `80` in the VPS firewall or cloud security group. Make sure no other service, such as nginx, Apache, or Caddy, is already bound to port `80`.

For Lemonade Tycoon 2, point `gt.jamdat.ca` at the VPS public IP. The game uses plain HTTP on TCP `80`. You can add HTTPS separately for modern browsers, but keep the HTTP compatibility path available for the game.

Runtime data lives in `./data/lsx.sqlite3` through the `./data:/app/data` bind mount. Back up that file before rebuilding the VPS or removing the project directory.

Docker Compose reads `.env` for build and runtime settings. `LSX_HTTP_PORT` controls the host port mapping; `LSX_ADDR` controls the address inside the container and should normally stay `:80`.

Common Docker operations:

```sh
docker compose logs -f
docker compose pull
docker compose up -d --build
docker compose down
```

## Game Routes

The recovered client does not need a modern API. It sends plain HTTP requests and scans the response body for simple tokens.

| Route | Server behavior |
| --- | --- |
| `/createaccount.php` | Accepts account probes and returns `ACCEPT`. |
| `/syncgame.php` | Stores uploaded scores, computes the recovered checksum, and returns `SUCCESS` by default. |
| `/lsx2.php`, `/board`, `/leaderboard` | Serves the recovered LSX board for old clients and the modern project page for browsers. |
| `/lsx2_detail.php` | Serves recovered score detail frames. |
| `/img/lsx2/connection.gif` | Returns the tiny connection image the game checks before opening LSX. |
| `/admin` or the configured admin path | Provides score, account, and request-log cleanup when admin credentials are enabled. |

Compatibility notes:

- Old IE-style `MSIE`/`Trident` user agents, empty user agents, and `?legacy=1` get the recovered board.
- Modern browsers get the project page at `/` and `/lsx2.php`.
- Default checksum mode accepts mismatches because the recovered client only checks for `SUCCESS`.
- `--strict-checksum` returns `FAIL` when `checksumclient` does not match.
- Checksum math follows recovered x86 signed 32-bit overflow.
- `gamestartingdate` is the client's packed 360-day-year/30-day-month scalar, not a real calendar timestamp.
- SQLite defaults to `data/lsx.sqlite3`.
- Tables: `submissions`, `accounts`, `request_events`.

## HTTP API

The server exposes a small read-only JSON API for integrations that need leaderboard data without scraping the recovered HTML pages. The API uses the same ranking rules as `/board`.

The contract is documented in [go/openapi.yaml](go/openapi.yaml). Routing uses `github.com/go-chi/chi/v5`, a pure Go `net/http` router with no CGO dependency.

### `GET /api/v1/leaderboard`

Returns leaderboard rows as JSON.

```sh
curl 'http://127.0.0.1:8080/api/v1/leaderboard?page=1&page_size=10&sort=market'
```

Query parameters:

| Name | Default | Description |
| --- | --- | --- |
| `page` | `1` | Positive page number. Values past the last page return the last page. |
| `page_size` | `10` | Positive page size, capped at `100`. |
| `sort` | `market` | One of `market`, `company`, `ceo`, or `lifespan`. |
| `username` | none | Case-insensitive exact username filter. |
| `gamemode` | none | Game mode filter matching the recovered client values. |
| `gamegoal` | none | Game goal filter matching the recovered client values. |

Example response:

```json
{
  "data": [
    {
      "rank": 1,
      "company": "Fast Citrus",
      "ceo": "CEO",
      "mode": "Challenge",
      "goal": "Cash challenge",
      "title": "Vendor",
      "lifespan": 3,
      "market_cents": 5000,
      "revenue_cents": 500,
      "retained_cents": 0,
      "stands": 1,
      "cups_sold": 6,
      "username": "fast",
      "date_scalar": "1",
      "source": "local #2",
      "checksum_valid": false
    }
  ],
  "pagination": {
    "page": 1,
    "page_size": 10,
    "total_items": 1,
    "total_pages": 1
  },
  "filters": {},
  "sort": "market"
}
```

`HEAD /api/v1/leaderboard` returns the same success headers without a response body.

Errors use `application/problem+json`:

```json
{
  "type": "about:blank",
  "title": "Bad Request",
  "status": 400,
  "detail": "page_size must be an integer from 1 to 100"
}
```

Unsupported methods return `405 Method Not Allowed` with `Allow: GET, HEAD`.

## Recovered Activation Key Proof

The repo includes a reproducible recovery path for the Armadillo ShortV3 key
material used by `/api/v1/keygen`:

```sh
nix develop -c make recover-shortv3
```

That derives the mapper seed and validation seed from the packed game, then uses
Sage to recover the ShortV3 private exponent from the embedded public
certificate and verifies the result.

## Documentation

Project docs are centralized under [docs/](docs/). Start with
[docs/README.md](docs/README.md) for the tool reference and current
reverse-engineering notes.

## Operations

### Admin

```sh
go run . --addr 127.0.0.1:8080 \
  --admin-user admin \
  --admin-password admin \
  --admin-path /back-office
```

Open `http://127.0.0.1:8080/back-office`.

Omit `--admin-path` to use `/admin`. `LSX_ADMIN_PATH`, `LSX_ADMIN_USER`, and `LSX_ADMIN_PASSWORD` work too. Sessions use signed HTTP-only cookies and CSRF tokens.

### Discord Alerts

```sh
./lsx-server --discord-webhook "$DISCORD_WEBHOOK_URL"
./lsx-server --discord-events sync,sync_rejected
./lsx-server --discord-icon embedded
```

Default events: `sync`, `sync_rejected`, `sync_error`, `account`, `account_error`.

## Credits

<table>
  <tr>
    <td width="120" align="center">
      <img src="go/web/static/project/lt2_asset_credits.avif" alt="" width="96">
    </td>
    <td>
      <ul>
        <li>Revival/server work: <a href="https://github.com/FlameFlag">FlameFlag</a></li>
        <li>Original LSX revival idea and domain name: <a href="https://github.com/TiZmSpectrum">TiZmSpectrum</a></li>
        <li>Historical behavior and seed rows: recovered client behavior plus archived <code>lsx2.php</code> pages from the <a href="https://web.archive.org/">Wayback Machine</a></li>
        <li>Original game and Lemonade Tycoon 2 branding: JAMDAT Mobile / EA Mobile</li>
      </ul>
    </td>
  </tr>
</table>

> [!WARNING]
> Unofficial fan preservation project. Not affiliated with, endorsed by, or sponsored by EA, JAMDAT, or the original Lemonade Tycoon 2 developers.
