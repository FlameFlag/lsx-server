---
kicker: LSX implementation notes
title: How Lemonade Tycoon 2 talks to LSX
updated: 2026-06-04
---

The LSX server only needs to recreate the protocol the game actually calls. The useful code is small: the unpacked runtime builds two plain HTTP requests, the embedded browser wrapper opens a local HTML file, and the server answers with the exact tokens the client scans for.

Use the language tabs on each implementation sample to switch between Go, C, Python, and TypeScript for that specific block. Recovered-code evidence, HTTP examples, shell commands, and flow diagrams stay visible around those samples.

| Item               | Finding                                          |
| ------------------ | ------------------------------------------------ |
| Main resource file | `Lemonade2.rb`, a 27 MB binary container         |
| Active host        | `gt.jamdat.ca`                                   |
| Browser entry      | `Lsx\CheckConnection.html`                       |
| Connectivity asset | `/img/lsx2/connection.gif`                       |
| Leaderboard page   | `/lsx2.php`                                      |
| Account endpoint   | `/createaccount.php` returns `ACCEPT`            |
| Upload endpoint    | `/syncgame.php?game=lemonade2` returns `SUCCESS` |
| Upload method      | HTTP GET over port 80                            |

## Runtime behavior {#runtime-behavior}

The unpacked executable contains the LSX request builders, checksum helper, date helper, and local browser entry path. These symbols are the main evidence for the server contract.

| Item                                                   | Meaning                                                  |
| ------------------------------------------------------ | -------------------------------------------------------- |
| `lemonade2.unpacked.lsx_upload` at `004073C0`          | builds and sends the score upload request                |
| `lemonade2.unpacked.lsx_account` at `0045FB70`         | builds and sends the account request                     |
| `lemonade2.unpacked.lsx_checksum` at `00410030`        | maps to `lt2_lsx_compute_checksum`                       |
| `lemonade2.unpacked.packed_date` at `00418FF0`         | maps to `lt2_lsx_packed_date_scalar`                     |
| `lemonade2.unpacked.lsx_connection_page` at `00420F10` | builds the local `Lsx\CheckConnection.html` path         |
| `teneon.load_url` at `10001210`                        | browser DLL navigation call, not the score upload client |

The important implementation detail is the request-completion flag. The client marks the request as handled when the response contains the expected token, but it also marks it handled when the HTTP helper itself fails. The case that stays visibly bad is a successful HTTP response without the token.

```c-source
int lt2_lsx_client_sets_request_flag(int request_result, const char *body,
    const char *token)
{
    if (body != NULL && token != NULL && strstr(body, token) != NULL) {
        return 1;
    }

    return request_result != LT2_LSX_HTTP_RESULT_OK;
}
```

The compatible server should therefore return the expected token for accepted operations. A `200 OK` response without `SUCCESS` is worse for the old client than a tiny success response.

```http
HTTP/1.1 200 OK
Content-Type: text/plain; charset=utf-8

SUCCESS
```

Here is the same token decision as a small portable helper.

```go
func clientSetsRequestFlag(requestOK bool, body string, token string) bool {
	if token != "" && strings.Contains(body, token) {
		return true
	}
	return !requestOK
}
```

```c
#include <stdbool.h>
#include <string.h>

bool client_sets_request_flag(bool request_ok, const char *body, const char *token)
{
    if (body != NULL && token != NULL && strstr(body, token) != NULL) {
        return true;
    }
    return !request_ok;
}
```

```python
def client_sets_request_flag(request_ok: bool, body: str | None, token: str | None) -> bool:
    if body is not None and token and token in body:
        return True
    return not request_ok
```

```ts
function clientSetsRequestFlag(requestOk: boolean, body: string | null, token: string): boolean {
  if (body !== null && token.length > 0 && body.includes(token)) {
    return true;
  }
  return !requestOk;
}
```

## Request builders {#request-builders}

The LSX protocol is simpler than a browser form. The runtime constructs raw query strings, sends GET requests, and searches the returned text. There is no JSON body, XML body, cookie session, browser POST, or CSRF token in the score path.

The account request only needs `username` and `password`. The upload request carries the career state and `checksumclient`.

```http
GET /createaccount.php?username=<username>&password=<password> HTTP/1.1
Accept: text/plain
Host: gt.jamdat.ca
```

```http
GET /syncgame.php?game=lemonade2
  &username=<username>
  &password=<password>
  &companyname=<url-encoded company>
  &ceoname=<url-encoded ceo>
  &gamemode=<int>
  &gamegoal=<int>
  &gamestartingdate=<int>
  &lifespan=<int>
  &stands=<int>
  &cupssold=<int>
  &cashassets=<int>
  &stockassets=<int>
  &standsassets=<int>
  &upgradesassets=<int>
  &retainedearnings=<int>
  &revenues=<int>
  &checksumclient=<int> HTTP/1.1
Accept: text/plain
Host: gt.jamdat.ca
```

The handlers below show the whole compatibility surface: parse query fields, store or inspect what arrived, compute checksum evidence for uploads, and answer with the token the old client scans for.

```go
func handleCreateAccount(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	storeAccount(r.Form.Get("username"), r.Form.Get("password"), r.URL.RawQuery)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ACCEPT\n"))
}

func handleSyncGame(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	fields := map[string]string{}
	for _, name := range syncFields {
		fields[name] = r.Form.Get(name)
	}
	computed := computeChecksum(fields)
	storeSubmission(fields, computed)
	_, _ = w.Write([]byte("SUCCESS\n"))
}
```

```c
typedef struct {
    const char *name;
    const char *value;
} QueryField;

const char *handle_create_account(const QueryField *query, int count)
{
    store_account(field(query, count, "username"),
        field(query, count, "password"));
    return "ACCEPT\n";
}

const char *handle_sync_game(const QueryField *query, int count)
{
    int32_t computed = compute_checksum(query, count);
    store_submission(query, count, computed);
    return "SUCCESS\n";
}
```

```python
def handle_create_account(query: dict[str, str]) -> bytes:
    store_account(query.get("username", ""), query.get("password", ""))
    return b"ACCEPT\n"


def handle_sync_game(query: dict[str, str]) -> bytes:
    fields = {name: query.get(name, "") for name in SYNC_FIELDS}
    computed = compute_checksum(fields)
    store_submission(fields, computed)
    return b"SUCCESS\n"
```

```ts
type Query = URLSearchParams;

function handleCreateAccount(query: Query): Response {
  storeAccount(query.get("username") ?? "", query.get("password") ?? "");
  return new Response("ACCEPT\n", { headers: { "content-type": "text/plain; charset=utf-8" } });
}

function handleSyncGame(query: Query): Response {
  const fields = Object.fromEntries(SYNC_FIELDS.map((name) => [name, query.get(name) ?? ""]));
  const computed = computeChecksum(fields);
  storeSubmission(fields, computed);
  return new Response("SUCCESS\n", { headers: { "content-type": "text/plain; charset=utf-8" } });
}
```

The recovered client behavior and the server implementation line up on one rule: compatibility is token-driven. The checksum is recorded because it is valuable evidence, but it is not required to make the game show a successful transfer unless strict mode is turned on.

## Checksum and date math {#checksum-date}

The checksum helper is a straight signed 32-bit arithmetic expression. The original x86 code wraps signed 32-bit arithmetic; implementations should intentionally preserve that behavior.

```c-source
int32_t lt2_lsx_compute_checksum(const Lt2LsxChecksumFields *fields)
{
    int32_t value;

    value =
        fields->gamestartingdate *
        (fields->revenues - fields->stands * 100) *
        fields->lifespan +
        fields->gamegoal * 5 -
        fields->standsassets -
        fields->cupssold +
        fields->gamemode * 7 +
        fields->retainedearnings +
        fields->upgradesassets +
        fields->cashassets;

    return value;
}
```

```go
func computeChecksum(fields map[string]string) int32 {
	i32 := func(name string) int32 {
		n, _ := strconv.ParseInt(fields[name], 10, 32)
		return int32(n)
	}
	return i32("gamestartingdate")*(i32("revenues")-i32("stands")*100)*i32("lifespan") +
		i32("gamegoal")*5 -
		i32("standsassets") -
		i32("cupssold") +
		i32("gamemode")*7 +
		i32("retainedearnings") +
		i32("upgradesassets") +
		i32("cashassets")
}
```

```c
int32_t compute_checksum(const QueryField *query, int count)
{
    int32_t date = field_i32(query, count, "gamestartingdate");
    int32_t revenues = field_i32(query, count, "revenues");
    int32_t stands = field_i32(query, count, "stands");
    int32_t lifespan = field_i32(query, count, "lifespan");

    return date * (revenues - stands * 100) * lifespan +
        field_i32(query, count, "gamegoal") * 5 -
        field_i32(query, count, "standsassets") -
        field_i32(query, count, "cupssold") +
        field_i32(query, count, "gamemode") * 7 +
        field_i32(query, count, "retainedearnings") +
        field_i32(query, count, "upgradesassets") +
        field_i32(query, count, "cashassets");
}
```

```python
def i32(value: int) -> int:
    value &= 0xFFFFFFFF
    return value - 0x100000000 if value & 0x80000000 else value


def compute_checksum(fields: dict[str, str]) -> int:
    n = lambda name: i32(int(fields.get(name, "0") or "0"))
    value = (
        n("gamestartingdate") * (n("revenues") - n("stands") * 100) * n("lifespan")
        + n("gamegoal") * 5
        - n("standsassets")
        - n("cupssold")
        + n("gamemode") * 7
        + n("retainedearnings")
        + n("upgradesassets")
        + n("cashassets")
    )
    return i32(value)
```

```ts
function i32(value: number): number {
  return value | 0;
}

function fieldI32(fields: Record<string, string>, name: string): number {
  return i32(Number.parseInt(fields[name] || "0", 10) || 0);
}

function computeChecksum(fields: Record<string, string>): number {
  const n = (name: string) => fieldI32(fields, name);
  return i32(
    n("gamestartingdate") * (n("revenues") - n("stands") * 100) * n("lifespan") +
      n("gamegoal") * 5 -
      n("standsassets") -
      n("cupssold") +
      n("gamemode") * 7 +
      n("retainedearnings") +
      n("upgradesassets") +
      n("cashassets")
  );
}
```

The term that looks most suspicious is `gamestartingdate`. It is not a Unix timestamp. The game converts its packed date structure to a fixed-calendar scalar with 360-day years and 30-day months, then multiplies that scalar into the checksum.

```go
type PackedDate struct {
	Year, Month, Day, Hour, Minute, Second, Millisecond int32
}

func packedDateScalar(d PackedDate) int32 {
	return d.Year*0x3df16000 +
		d.Month*-0x65813800 +
		(((d.Day*24+d.Hour)*60+d.Minute)*60+d.Second)*1000 +
		d.Millisecond
}
```

```c
typedef struct {
    int32_t year, month, day, hour, minute, second, millisecond;
} PackedDate;

int32_t packed_date_scalar(PackedDate d)
{
    return d.year * (int32_t)0x3df16000 +
        d.month * (int32_t)-0x65813800 +
        (((d.day * 24 + d.hour) * 60 + d.minute) * 60 + d.second) * 1000 +
        d.millisecond;
}
```

```python
def packed_date_scalar(date: dict[str, int]) -> int:
    value = (
        date["year"] * 0x3DF16000
        + date["month"] * -0x65813800
        + (((date["day"] * 24 + date["hour"]) * 60 + date["minute"]) * 60 + date["second"]) * 1000
        + date["millisecond"]
    )
    return i32(value)
```

```ts
type PackedDate = {
  year: number;
  month: number;
  day: number;
  hour: number;
  minute: number;
  second: number;
  millisecond: number;
};

function packedDateScalar(date: PackedDate): number {
  return i32(
    date.year * 0x3df16000 +
      date.month * -0x65813800 +
      (((date.day * 24 + date.hour) * 60 + date.minute) * 60 + date.second) * 1000 +
      date.millisecond
  );
}
```

Field-by-field, the checksum behaves like this:

| Query field        | Checksum role                                   |
| ------------------ | ----------------------------------------------- |
| `gamestartingdate` | main date multiplier                            |
| `lifespan`         | main lifespan multiplier                        |
| `revenues`         | revenue term inside the main product            |
| `stands`           | subtracts `stands * 100` from revenues          |
| `cupssold`         | subtracted directly                             |
| `cashassets`       | added directly                                  |
| `stockassets`      | uploaded and rendered, but not used in checksum |
| `standsassets`     | subtracted directly                             |
| `upgradesassets`   | added directly                                  |
| `retainedearnings` | added directly                                  |
| `gamemode`         | adds `gamemode * 7`                             |
| `gamegoal`         | adds `gamegoal * 5`                             |

That means `stockassets` can affect the visible market cap without changing the recovered checksum. That is not a server bug; it matches the client helper.

## Browser boundary {#browser-boundary}

The embedded browser DLL is only the LSX window. It exposes navigation-style calls such as `LoadURL`, `Back`, `Forward`, `Refresh`, `Show`, and `Hide`. It does not submit score data.

The game opens a local file first:

```text
Lsx\CheckConnection.html
Lsx\CheckConnection.html?username=<username>
```

That local page checks whether the remote GIF loads and then navigates to the leaderboard. This is why the server includes the image endpoint even though the actual score upload path does not depend on it.

```text
Lsx\CheckConnection.html
  -> http://gt.jamdat.ca/img/lsx2/connection.gif
  -> http://gt.jamdat.ca/lsx2.php

Lsx\CheckConnection.html?username=<username>
  -> http://gt.jamdat.ca/img/lsx2/connection.gif
  -> http://gt.jamdat.ca/lsx2.php?username=<username>
```

A compatibility server only has to serve a valid GIF at that path.

```go
func handleConnectionGIF(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "image/gif")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(gif1x1)
}
```

```c
void handle_connection_gif(Response *res)
{
    response_header(res, "Content-Type", "image/gif");
    response_header(res, "Cache-Control", "no-store");
    response_write(res, gif1x1, sizeof(gif1x1));
}
```

```python
def handle_connection_gif() -> tuple[int, dict[str, str], bytes]:
    return 200, {"Content-Type": "image/gif", "Cache-Control": "no-store"}, GIF_1X1
```

```ts
function handleConnectionGif(): Response {
  return new Response(GIF_1X1, {
    headers: { "content-type": "image/gif", "cache-control": "no-store" }
  });
}
```

Modern browsers and the old embedded IE window are split at render time. If the request looks like a modern browser, `/lsx2.php` can show the project app. If it looks like legacy IE, the route returns the compact recovered leaderboard page.

## Resource container {#resource-container}

The installer contains compressed payloads for the main resource file and the embedded browser DLL. Carving those streams is enough to recover the data the game installs.

| Offset     | Payload      | Installed role                   |
| ---------- | ------------ | -------------------------------- |
| `0xFEA4C`  | bzip2 stream | expands to `Lemonade2.rb`        |
| `0x8B1F24` | bzip2 stream | expands to `TeneonIERelease.dll` |
| `0x8B18C6` | GIF data     | installer image strip            |
| `0x2D280`  | bzip2 stream | small DIB-style bitmap           |

```bash
dd if="$INPUT" of=lemonade2_rb.bz2 bs=1 skip=1043020 count=8072295 status=none
dd if="$INPUT" of=teneon_ie_release.bz2 bs=1 skip=9117476 count=172707 status=none

bzip2 -dc lemonade2_rb.bz2 > Lemonade2.rb
bzip2 -dc teneon_ie_release.bz2 > TeneonIERelease.dll
```

`Lemonade2.rb` starts with a segment chain. The useful high-level map is stable: segment type `1` holds strings, type `2` holds bitmap records, and type `8` holds FastTracker II music.

| Segment        | Type                            | Contents             |
| -------------- | ------------------------------- | -------------------- |
| 0              | `1`                             | 704 CP1252 strings   |
| 1              | `2`                             | 233 bitmap records   |
| 7              | `8`                             | two XM modules       |
| Other segments | `3, 4, 5, 6, 7, 10, 11, 12, 13` | additional game data |

The parser only needs to follow `(type, next_offset, count_or_flags)` records until the chain reaches EOF or an invalid next pointer.

```go
type Segment struct {
	Type, Offset, Next, CountOrFlags uint32
}

func parseSegments(buf []byte, start uint32) ([]Segment, error) {
	var out []Segment
	for off := start; int(off)+12 <= len(buf); {
		segType := binary.LittleEndian.Uint32(buf[off:])
		next := binary.LittleEndian.Uint32(buf[off+4:])
		flags := binary.LittleEndian.Uint32(buf[off+8:])
		if next <= off || int(next) > len(buf) {
			return nil, fmt.Errorf("bad segment chain at 0x%x", off)
		}
		out = append(out, Segment{segType, off, next, flags})
		off = next
		if int(off) == len(buf) {
			break
		}
	}
	return out, nil
}
```

```c
typedef struct {
    uint32_t type, offset, next, count_or_flags;
} Segment;

bool parse_segments(const uint8_t *buf, size_t size, uint32_t start,
    Segment *out, size_t out_cap, size_t *out_count)
{
    uint32_t off = start;
    *out_count = 0;
    while ((size_t)off + 12 <= size && *out_count < out_cap) {
        uint32_t type = u32le(buf + off);
        uint32_t next = u32le(buf + off + 4);
        uint32_t flags = u32le(buf + off + 8);
        if (next <= off || next > size) return false;
        out[(*out_count)++] = (Segment){type, off, next, flags};
        off = next;
        if (off == size) break;
    }
    return true;
}
```

```python
import struct


def u32(buf: bytes, off: int) -> int:
    return struct.unpack_from("<I", buf, off)[0]


def parse_segments(buf: bytes, start: int = 0x1038) -> list[tuple[int, int, int, int]]:
    segments = []
    off = start
    while off + 12 <= len(buf):
        seg_type = u32(buf, off)
        next_off = u32(buf, off + 4)
        count_or_flags = u32(buf, off + 8)
        if next_off <= off or next_off > len(buf):
            raise ValueError(f"bad segment chain at 0x{off:x}")
        segments.append((seg_type, off, next_off, count_or_flags))
        off = next_off
        if off == len(buf):
            break
    return segments
```

```ts
type Segment = {
  type: number;
  offset: number;
  next: number;
  countOrFlags: number;
};

function parseSegments(buf: Uint8Array, start = 0x1038): Segment[] {
  const view = new DataView(buf.buffer, buf.byteOffset, buf.byteLength);
  const segments: Segment[] = [];
  for (let off = start; off + 12 <= buf.length; ) {
    const type = view.getUint32(off, true);
    const next = view.getUint32(off + 4, true);
    const countOrFlags = view.getUint32(off + 8, true);
    if (next <= off || next > buf.length) throw new Error(`bad segment chain at 0x${off.toString(16)}`);
    segments.push({ type, offset: off, next, countOrFlags });
    off = next;
    if (off === buf.length) break;
  }
  return segments;
}
```

The resource file explains menus, messages, bundled links, and local LSX files. The unpacked executable defines the network contract. Keeping those two sources separate avoids treating stale UI strings as live endpoints.

## Endpoint mapping {#runtime-endpoints}

The recovered strings include both resource-file UI copy and executable request paths. The server follows the runtime paths.

| String or path                                       | Seen in                     | Server meaning         |
| ---------------------------------------------------- | --------------------------- | ---------------------- |
| `Your game has been successfully transfered to LSX!` | `Lemonade2.rb` string table | upload success message |
| `The database is not accessible.`                    | `Lemonade2.rb` string table | upload failure message |
| `gt.jamdat.ca`                                       | unpacked runtime references | active service host    |
| `/createaccount.php?`                                | unpacked runtime references | account probe          |
| `/syncgame.php?game=lemonade2`                       | unpacked runtime references | career upload          |
| `Lsx\CheckConnection.html`                           | unpacked runtime references | local browser entry    |
| `/img/lsx2/connection.gif`                           | local browser page          | connectivity image     |

The route table mirrors that split. Old paths are mounted exactly, while modern project pages reuse `/lsx2.php` only when the user agent makes that safe.

```go
router.HandleFunc("/lsx2.php", s.withServer((*Server).handleEntryPage))
router.HandleFunc("/lsx2_detail.php", s.withServer((*Server).handleDetail))
router.HandleFunc("/syncgame.php", s.withServer((*Server).handleSync))
router.HandleFunc("/createaccount.php", s.withServer((*Server).handleCreateAccount))
router.HandleFunc("/img/lsx2/connection.gif", s.withServer((*Server).handleConnectionGIF))
```

```c
route(router, "/lsx2.php", handle_entry_page);
route(router, "/lsx2_detail.php", handle_detail);
route(router, "/syncgame.php", handle_sync_game);
route(router, "/createaccount.php", handle_create_account);
route(router, "/img/lsx2/connection.gif", handle_connection_gif);
```

```python
routes = {
    "/lsx2.php": handle_entry_page,
    "/lsx2_detail.php": handle_detail,
    "/syncgame.php": handle_sync_game,
    "/createaccount.php": handle_create_account,
    "/img/lsx2/connection.gif": handle_connection_gif,
}
```

```ts
const routes: Record<string, Handler> = {
  "/lsx2.php": handleEntryPage,
  "/lsx2_detail.php": handleDetail,
  "/syncgame.php": handleSyncGame,
  "/createaccount.php": handleCreateAccount,
  "/img/lsx2/connection.gif": handleConnectionGif
};
```

## Leaderboard reconstruction {#leaderboard}

The public LSX page is a compact table with rank, company, CEO, lifespan, and market cap. It accepts controls such as `pagenum`, `sort`, `gamemode`, `gamegoal`, `ranktype`, and `username`.

The server converts stored submissions into leaderboard rows with the same field names the upload sent. Market cap is cash plus stock plus stand assets plus upgrade assets. If that total is zero, retained earnings is used as a fallback so incomplete recovered rows still display sensibly.

```go
func rowFromSubmission(fields map[string]string, checksumValid bool) LeaderboardRow {
	cash := cents(fields["cashassets"])
	stock := cents(fields["stockassets"])
	stands := cents(fields["standsassets"])
	upgrades := cents(fields["upgradesassets"])
	retained := cents(fields["retainedearnings"])
	market := cash + stock + stands + upgrades
	if market == 0 {
		market = retained
	}
	return LeaderboardRow{
		Company: first(fields["companyname"], "(unnamed company)"),
		CEO: first(fields["ceoname"], "(unknown)"),
		MarketCents: market,
		ChecksumValid: checksumValid,
	}
}
```

```c
LeaderboardRow row_from_submission(const QueryField *fields, int count, bool checksum_valid)
{
    int64_t cash = cents(fields, count, "cashassets");
    int64_t stock = cents(fields, count, "stockassets");
    int64_t stands = cents(fields, count, "standsassets");
    int64_t upgrades = cents(fields, count, "upgradesassets");
    int64_t retained = cents(fields, count, "retainedearnings");
    int64_t market = cash + stock + stands + upgrades;
    if (market == 0) market = retained;

    return (LeaderboardRow){
        first(field(fields, count, "companyname"), "(unnamed company)"),
        first(field(fields, count, "ceoname"), "(unknown)"),
        market,
        checksum_valid
    };
}
```

```python
def row_from_submission(fields: dict[str, str], checksum_valid: bool) -> dict[str, object]:
    cash = cents(fields.get("cashassets", "0"))
    stock = cents(fields.get("stockassets", "0"))
    stands = cents(fields.get("standsassets", "0"))
    upgrades = cents(fields.get("upgradesassets", "0"))
    retained = cents(fields.get("retainedearnings", "0"))
    market = cash + stock + stands + upgrades or retained
    return {
        "company": fields.get("companyname") or "(unnamed company)",
        "ceo": fields.get("ceoname") or "(unknown)",
        "market_cents": market,
        "checksum_valid": checksum_valid,
    }
```

```ts
type LeaderboardRow = {
  company: string;
  ceo: string;
  marketCents: number;
  checksumValid: boolean;
};

function rowFromSubmission(fields: Record<string, string>, checksumValid: boolean): LeaderboardRow {
  const cash = cents(fields.cashassets);
  const stock = cents(fields.stockassets);
  const stands = cents(fields.standsassets);
  const upgrades = cents(fields.upgradesassets);
  const retained = cents(fields.retainedearnings);
  const market = cash + stock + stands + upgrades || retained;
  return {
    company: fields.companyname || "(unnamed company)",
    ceo: fields.ceoname || "(unknown)",
    marketCents: market,
    checksumValid
  };
}
```

Sorting follows the recovered page controls. The default and sort `4`/`14` rank by market cap descending; sort `1` is company name, sort `2` is CEO name, and sort `3` is lifespan descending with market cap as a tie breaker.

Company rows open a detail page through `d1` through `d18` query parameters. The detail page is not a second database lookup in the old shape; the table row carries the values the detail frame needs.

| Detail parameter | Meaning                       |
| ---------------- | ----------------------------- |
| `d1`             | company                       |
| `d2`             | CEO                           |
| `d3`             | mode                          |
| `d4`             | goal label                    |
| `d5`             | rank                          |
| `d6`             | total entries in current view |
| `d7`             | title                         |
| `d8`             | lifespan                      |
| `d9`             | stands                        |
| `d10`            | cups sold                     |
| `d11`            | market cap                    |
| `d12`            | revenues                      |
| `d13`            | retained earnings             |
| `d14`            | percent field                 |
| `d15`            | cash assets                   |
| `d16`            | stock assets                  |
| `d17`            | stand assets                  |
| `d18`            | upgrade assets                |

```go
func detailURL(row LeaderboardRow, rank, total int) string {
	q := url.Values{}
	q.Set("d1", row.Company)
	q.Set("d2", row.CEO)
	q.Set("d5", strconv.Itoa(rank))
	q.Set("d6", strconv.Itoa(total))
	q.Set("d11", formatCents(row.MarketCents))
	return "lsx2_detail.php?" + q.Encode()
}
```

```c
void detail_url(char *out, size_t cap, LeaderboardRow row, int rank, int total)
{
    snprintf(out, cap,
        "lsx2_detail.php?d1=%s&d2=%s&d5=%d&d6=%d&d11=%s",
        url_escape(row.company), url_escape(row.ceo), rank, total,
        format_cents(row.market_cents));
}
```

```python
from urllib.parse import urlencode


def detail_url(row: dict[str, object], rank: int, total: int) -> str:
    return "lsx2_detail.php?" + urlencode({
        "d1": row["company"],
        "d2": row["ceo"],
        "d5": rank,
        "d6": total,
        "d11": format_cents(row["market_cents"]),
    })
```

```ts
function detailUrl(row: LeaderboardRow, rank: number, total: number): string {
  const query = new URLSearchParams({
    d1: row.company,
    d2: row.ceo,
    d5: String(rank),
    d6: String(total),
    d11: formatCents(row.marketCents)
  });
  return `lsx2_detail.php?${query}`;
}
```

## Compatibility contract {#compatible-server}

A small compatible server is enough because the game does not negotiate a complex protocol. The required behavior is:

- Return text containing `ACCEPT` from `/createaccount.php`.
- Store uploads sent to `/syncgame.php?game=lemonade2`.
- Return text containing `SUCCESS` for accepted upload attempts.
- Serve `/img/lsx2/connection.gif` so the embedded browser connection check passes.
- Render `/lsx2.php` and detail pages in the compact legacy shape for the embedded browser.
- Compute the checksum for admin review and optional strict mode.

The main failure mode is overbuilding the response. The old client is happier with a tiny token response than with a modern API envelope.
