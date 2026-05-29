---
kicker: LSX implementation notes
title: How Lemonade Tycoon 2 talks to LSX
updated: 2026-05-28
---

The LSX revival only needs to imitate the parts of the old service the game actually touches. The client is plain, direct, and much smaller than the surrounding installer and asset format make it look.

Four pieces matter: the installer payload, `Lemonade2.rb`, the embedded browser wrapper, and the Winsock HTTP code in the game runtime. The browser shows LSX pages. The game executable sends account checks and career uploads.

| Item               | Finding                                          |
| -------------------| ------------------------------------------------ |
| Main resource file | `Lemonade2.rb`, a 27 MB binary container         |
| Active host        | `gt.jamdat.ca`                                   |
| Browser check      | `/img/lsx2/connection.gif`                       |
| Leaderboard page   | `/lsx2.php`                                      |
| Account endpoint   | `/createaccount.php` returns `ACCEPT`            |
| Upload endpoint    | `/syncgame.php?game=lemonade2` returns `SUCCESS` |
| Upload method      | HTTP GET over port 80                            |

## The installer contains the useful payloads {#installer-payloads}

The Clickteam installer contains compressed payloads for the main resource file and the embedded browser DLL. That means recovery starts by carving the setup executable, not by chasing an installer manifest.

| Offset     | Payload      | Installed role                   |
| ---------- | ------------ | -------------------------------- |
| `0xFEA4C`  | bzip2 stream | expands to `Lemonade2.rb`        |
| `0x8B1F24` | bzip2 stream | expands to `TeneonIERelease.dll` |
| `0x8B18C6` | GIF data     | installer image strip            |
| `0x2D280`  | bzip2 stream | small DIB-style bitmap           |

```bash title="Carve the two important streams"
dd if="$INPUT" of=lemonade2_rb.bz2 bs=1 skip=1043020 count=8072295 status=none
dd if="$INPUT" of=teneon_ie_release.bz2 bs=1 skip=9117476 count=172707 status=none

bzip2 -dc lemonade2_rb.bz2 > Lemonade2.rb
bzip2 -dc teneon_ie_release.bz2 > TeneonIERelease.dll
```

## Lemonade2.rb is a resource container {#what-rb-is}

`Lemonade2.rb` starts with a small segment chain. The useful high-level map is stable: segment 1 holds strings, segment 2 holds bitmap records, and segment 8 holds FastTracker II music.

| Segment        | Type                            | Contents             |
| -------------- | ------------------------------- | -------------------- |
| 0              | `1`                             | 704 CP1252 strings   |
| 1              | `2`                             | 233 bitmap records   |
| 7              | `8`                             | two XM modules       |
| Other segments | `3, 4, 5, 6, 7, 10, 11, 12, 13` | additional game data |

```python title="Segment chain parser"
import struct

SEGMENT_TABLE_OFFSET = 0x1038

def u32(buf, off):
    return struct.unpack_from("<I", buf, off)[0]

def parse_segments(buf, start=SEGMENT_TABLE_OFFSET):
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

## The endpoint evidence comes from the runtime {#runtime-endpoints}

The string table includes old UI copy and a historical `hexacto.com` LSX URL. The running client path points at `gt.jamdat.ca` and builds requests for `/createaccount.php`, `/syncgame.php?game=lemonade2`, `/lsx2.php`, and `/img/lsx2/connection.gif`.

That split is the important server rule. The resource container explains menus, messages, and old links. The runtime tells us the compatibility contract.

| String or path                                       | Seen in                     | Server meaning                |
| ---------------------------------------------------- | --------------------------- | ----------------------------- |
| `Your game has been successfully transfered to LSX!` | `Lemonade2.rb` string table | upload success message        |
| `The database is not accessible.`                    | `Lemonade2.rb` string table | upload failure message        |
| `gt.jamdat.ca`                                       | runtime references          | active service host           |
| `/createaccount.php?`                                | runtime references          | account probe                 |
| `/syncgame.php?game=lemonade2`                       | runtime references          | career upload                 |
| `/img/lsx2/connection.gif`                           | browser check page          | visible LSX connectivity test |

## The browser is only the window {#browser-boundary}

The DLL exposes browser-style calls such as `LoadURL`, `Back`, `Forward`, `Refresh`, `Show`, and `Hide`. Its navigation call passes no post data and no custom headers.

The visible LSX panel starts at a local file, checks whether the remote GIF loads, then redirects to the leaderboard. That behavior needs server support, but it is separate from account and score transfer.

```title="Browser-side flow"
Lsx\CheckConnection.html
  -> http://gt.jamdat.ca/img/lsx2/connection.gif
  -> http://gt.jamdat.ca/lsx2.php

Lsx\CheckConnection.html?username=<username>
  -> http://gt.jamdat.ca/img/lsx2/connection.gif
  -> http://gt.jamdat.ca/lsx2.php?username=<username>
```

## The LSX protocol is plain HTTP {#lsx-protocol}

Summary: The game sends GET requests and scans raw response text for one token.

The account path accepts a username and password. If the response contains `ACCEPT`, the client treats the account check as good.

The career upload path sends the score fields as query parameters. If the response contains `SUCCESS`, the client shows the success message. It does not require JSON, XML, cookies, a session, or a browser POST.

```http title="Account request"
GET /createaccount.php?username=<username>&password=<password> HTTP/1.1
Accept: text/plain
Host:gt.jamdat.ca
```

```http title="Career upload request"
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
Host:gt.jamdat.ca
```

```c title="Response checks"
account_ok = http_request_ok && strstr(response, "ACCEPT") != NULL;
upload_ok  = http_request_ok && strstr(response, "SUCCESS") != NULL;
```

## Upload validation is optional for compatibility {#checksum-date}

The checksum is still worth computing because it lets the server flag bad or synthetic submissions. It should not block baseline compatibility unless strict mode is deliberately enabled.

`gamestartingdate` is a fixed-calendar scalar, not a Unix timestamp. The game uses 360-day years and 30-day months.

| Query field        | Checksum role                           |
| ------------------ | --------------------------------------- |
| `gamestartingdate` | main date multiplier                    |
| `lifespan`         | main lifespan multiplier                |
| `revenues`         | revenue term                            |
| `stands`           | subtracts `stands * 100` from revenues  |
| `cupssold`         | subtracted                              |
| `cashassets`       | added                                   |
| `stockassets`      | sent, not used in the recovered formula |
| `standsassets`     | subtracted                              |
| `upgradesassets`   | added                                   |
| `retainedearnings` | added                                   |
| `gamemode`         | adds `gamemode * 7`                     |
| `gamegoal`         | adds `gamegoal * 5`                     |

```python title="Checksum formula"
checksum = (
    gamestartingdate
    * (revenues - stands * 100)
    * lifespan
    + gamegoal * 5
    - standsassets
    - cupssold
    + gamemode * 7
    + retainedearnings
    + upgradesassets
    + cashassets
)
```

```python title="Date scalar"
gamestartingdate = (
    (((year * 360 + month * 30 + day) * 24 + hour) * 60 + minute)
    * 60 * 1000
    + second * 1000
    + millisecond
)
```

## The leaderboard shape is recoverable {#leaderboard}

The public LSX page is a compact table with rank, company, CEO, lifespan, and market cap. It accepts controls such as `pagenum`, `sort`, `gamemode`, `gamegoal`, `ranktype`, and `username`.

Company rows open a detail page through `d1` through `d18` query parameters. One correction matters for rendering: `d6` is the total number of entries in the current leaderboard view.

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

## A small compatible server is enough {#compatible-server}

The core server behavior is direct:

- Return text containing `ACCEPT` from `/createaccount.php`.
- Store uploads sent to `/syncgame.php?game=lemonade2`.
- Return text containing `SUCCESS` for accepted upload attempts.
- Serve `/img/lsx2/connection.gif` so the embedded browser connection check passes.
- Render `/lsx2.php` and detail pages in the compact legacy shape.
- Compute the recovered checksum for admin review and optional strict mode.

```go title="Minimum account handler"
func handleCreateAccount(query url.Values) []byte {
    storeAccountProbe(query.Get("username"), query.Get("password"))
    return []byte("ACCEPT\n")
}
```

```go title="Minimum upload handler"
func handleSyncGame(query url.Values) []byte {
    computed := computeRecoveredChecksum(query)
    storeSubmission(query, computed)
    return []byte("SUCCESS\n")
}
```
