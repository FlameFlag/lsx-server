---
kicker: Reverse-engineering write-up
title: What the Lemonade Tycoon 2 LSX files actually do
updated: 2026-05-06
---

The Lemonade Stock Exchange is not one web feature. In the recovered Windows build, four parts share the work: a Clickteam installer stub, the `Lemonade2.rb` resource container, an embedded Internet Explorer wrapper, and a small Winsock HTTP client inside the unpacked game runtime.

The `Lemonade2.rb` name is the first trap. It is not Ruby source. It is a 27 MB binary resource container with strings, bitmap records, music modules, and other game data.

Each section states the finding, the compatibility or preservation impact, and the evidence visible on this page.

| Item                 | Finding                                                     |
| -------------------- | ----------------------------------------------------------- |
| Installer            | embedded bzip2 payloads, not a discovered remote downloader |
| Main file            | Lemonade2.rb, a 27 MB binary resource container             |
| String table         | 704 CP1252 strings in segment 1                             |
| Graphics stream      | 233 sequential bitmap records in segment 2                  |
| Primary color format | little-endian RGB565, sometimes with a separate alpha byte  |
| Music                | two FastTracker II XM modules                               |
| LSX host             | gt.jamdat.ca                                                |
| Upload result token  | SUCCESS                                                     |

## The setup EXE already carries the game payloads {#installer-payloads}

Summary: The installer has network-capable code, but this sample already contains the game payloads.

Takeaway: Missing download URLs are not the blocker here. The useful files can be carved from the setup EXE.

The installer is a PE32 Clickteam Install Creator-style stub. Static analysis found WinINet imports and Clickteam URLs, so a remote download was a plausible first theory. The embedded payloads rule that out for this sample: the game resources, one DLL, one GIF strip, and one small bitmap payload are already inside the installer.

The most important embedded stream starts at file offset 0xFEA4C. It expands into the 27 MB resource payload that installs as Lemonade2.rb. A second bzip2 stream at 0x8B1F24 expands to TeneonIERelease.dll, the embedded browser wrapper. A small bzip2 stream at 0x2D280 expands to a DIB-style bitmap, and a 600x38 GIF begins at 0x8B18C6.

Ordinary archive tools and URL searches miss the useful path. The installer has downloader-shaped code, but LSX/server and asset recovery starts with the embedded payloads and the unpacked runtime image, not with a remote setup manifest.

The carve offsets below name the embedded payloads directly. The decisive point is that the installer contains complete compressed streams for the main resource file and browser DLL; it is not merely pointing to a remote manifest.

| Offset   | Recovered item                             | Why it matters                                                           |
| -------- | ------------------------------------------ | ------------------------------------------------------------------------ |
| 0x2D280  | bzip2 stream to small DIB payload          | Confirms the installer stores small visual payloads inline               |
| 0xFEA4C  | bzip2 stream to the 27 MB resource payload | Becomes Lemonade2.rb, the main source for strings, images, and music     |
| 0x8B18C6 | 600x38 GIF                                 | One ordinary image asset is directly carved from the installer           |
| 0x8B1F24 | bzip2 stream to TeneonIERelease.dll        | Provides the embedded browser wrapper, but not the score upload protocol |

## Lemonade2.rb is a resource container {#what-rb-is}

Summary: The `.rb` extension is misleading. The file is binary container data, not source code.

Takeaway: Start with the 12-record segment chain. After that, the strings, graphics, and music stop looking random.

The installed game directory contains Lemonade2.rb beside the executable. The name looks like Ruby, but the bytes do not contain Ruby syntax. The file starts with container data and uses a 12-record segment chain beginning at offset 0x1038.

Each segment record gives a type, the next segment offset, and a count or flag value. The file behaves like a small archive format, not a script. Segment 1 holds 704 CP1252 strings. Segment 2 holds the main graphics stream. Segment 8 holds the tracker music.

The practical extraction strategy is to parse the segment chain first, then decode each segment by type. Random PNG or JPEG signature scans give a false negative here; most recovered art uses custom RGB565 bitmap records, and some visible runtime art is still unresolved.

The segment table below gives the recovered type, offset range, and content for the important segments. The parser snippet shows the 12-byte segment-record shape used to reproduce that map.

| Segment        | Type                          | Offset range                                           | What it contains                             |
| -------------- | ----------------------------- | ------------------------------------------------------ | -------------------------------------------- |
| 0              | 1                             | 0x1038 to 0x7288                                       | String table metadata and 704 CP1252 strings |
| 1              | 2                             | 0x7288 to 0x1472D22                                    | 233 sequential bitmap records                |
| 7              | 8                             | 0x14FBC9A to 0x1A98452                                 | XM music data                                |
| Other segments | 3, 4, 5, 6, 7, 10, 11, 12, 13 | Smaller game data blocks that still need deeper naming |                                              |

```title="Segment chain parser"
import struct

SEGMENT_TABLE_OFFSET = 0x1038

def u32(buf, off):
    return struct.unpack_from('<I', buf, off)[0]

def parse_segments(buf, start=SEGMENT_TABLE_OFFSET):
    segments = []
    off = start

    while off + 12 <= len(buf):
        seg_type = u32(buf, off)
        next_off = u32(buf, off + 4)
        count_or_flags = u32(buf, off + 8)

        if next_off <= off or next_off > len(buf):
            raise ValueError(f'bad segment chain at 0x{off:x}')

        segments.append((seg_type, off, next_off, count_or_flags))
        off = next_off
        if off == len(buf):
            break

    return segments
```

## The strings explain the feature boundaries {#string-breadcrumbs}

Summary: The string table separates UI copy, old web leads, active endpoints, and local browser files.

Takeaway: Strings are evidence, but they are not all equal. A URL in a resource table is a lead; an executable code reference to that URL is stronger.

Segment 1 maps the game-facing LSX text. It contains length-prefixed messages for successful transfers, database failures, missing career uploads, and the confirmation prompt for uploading a career game to the LSX database.

Those UI messages live in the installed `Lemonade2.rb` resource container. They do not appear as plain ASCII strings in the installed `Lemonade2.exe` or `TeneonIERelease.dll`; the EXE-side endpoint evidence comes from the unpacked runtime image and the decompiled LSX passes.

One resource string names `http://www.hexacto.com/lsx2.php`, but the unpacked runtime image points the active account and score flow at `gt.jamdat.ca`. The recovered executable references `gt.jamdat.ca`, `/createaccount.php?`, `/syncgame.php?game=lemonade2`, `/lsx2.php`, and `/img/lsx2/connection.gif`.

That split controls the server work. The old Hexacto URL is historically meaningful, but the compatibility server must implement the endpoints that the executable actually calls.

The table below separates resource strings from runtime endpoint strings. That distinction is what makes `gt.jamdat.ca`, not the old Hexacto URL, the compatibility target.

| String or path                                     | Where it was observed                       | Interpretation                                                           |
| -------------------------------------------------- | ------------------------------------------- | ------------------------------------------------------------------------ |
| Your game has been successfully transfered to LSX! | Lemonade2.rb string table                   | Player-facing success copy                                               |
| The database is not accessible.                    | Lemonade2.rb string table                   | Player-facing failure copy                                               |
| <http://www.hexacto.com/lsx2.php>                  | Lemonade2.rb string table                   | Legacy or resource-era LSX page pointer                                  |
| gt.jamdat.ca                                       | Unpacked runtime executable references      | Active host for account checks, score upload, and browser LSX pages      |
| Lsx\CheckConnection.html                           | Installed game files and runtime references | Local browser bridge that checks the remote LSX image before redirecting |

## The graphics are packed bitmap records {#graphics-format}

Summary: Segment 2 is not a folder of PNGs or JPEGs. It is a stream of custom bitmap records.

Takeaway: A graphics record is 12 bytes of header followed by raw pixel bytes. The flags field tells you how many bytes each row consumes.

The graphics segment contains exactly 233 records. The extractor starts at segment offset plus 12 and walks record by record until it reaches the next segment offset. That final position check is important: it proves the decoder consumed the stream cleanly instead of merely finding plausible image-shaped data.

Each bitmap record starts with width, height, flags, and a fourth dword. For normal mode 7 records, that fourth dword is usually not needed. For 0x20660 records, the fourth dword behaves like a background or transparent-color hint.

The mode is encoded inside the flags value as `(flags >> 8) & 0x1f`. Modes 4, 5, and 6 use two bytes per pixel. Mode 7 uses three bytes per pixel. Mode 3 uses one byte per pixel and is best handled as a mask or grayscale image until a specific UI role is identified.

One count is easy to mix up: the older `rb_rgb565a_candidates` pass has 82 likely RGB565+alpha records, while the complete segment-2 walk has 233 records. The 233 number comes from `rb_graphics_all`, not from the older candidate directory.

The record format, flag modes, and row-size code below are enough to reproduce the full walk. The clean-stop check is the proof point: 233 decoded records consume the segment exactly.

| Flags   | Mode | Bytes per pixel | Meaning                                         |
| ------- | ---- | --------------- | ----------------------------------------------- |
| 0x8760  | 7    | 3               | RGB565 color plus one alpha byte                |
| 0x0660  | 6    | 2               | Opaque RGB565                                   |
| 0x20660 | 6    | 2               | RGB565 with a background/transparent-color hint |
| 0xC300  | 3    | 1               | One-byte mask or grayscale-style image          |

```title="Bitmap record shape"
struct BitmapRecord {
    uint16_t width;
    uint16_t height;
    uint32_t flags;
    uint32_t background_or_zero;
    uint8_t pixels[row_bytes * height];
};
```

```title="Row size from flags"
def bitmap_row_bytes(width, flags):
    mode = (flags >> 8) & 0x1f

    if mode in (1, 2, 3):
        return mode, width
    if mode in (4, 5, 6):
        return mode, width * 2
    if mode == 7:
        return mode, width * 3
    if mode == 8:
        return mode, width * 4

    raise ValueError(f'unsupported bitmap mode {mode}')
```

## RGB565 is the key color format {#rgb565}

Summary: Most decoded art uses little-endian RGB565. Mode 7 adds a separate alpha byte after each color word.

Takeaway: Read two bytes as a little-endian 16-bit color. Expand 5 red bits, 6 green bits, and 5 blue bits into normal 8-bit RGBA output.

RGB565 packs one pixel into 16 bits: five bits of red, six bits of green, and five bits of blue. Because the file stores that 16-bit value little-endian, the first byte is the low byte of the color.

For mode 7, every pixel is three bytes: `uint16le rgb565` followed by `uint8 alpha`. Transparent pixels commonly show up as color 0xffff with alpha 0, but the alpha byte is the actual transparency source.

For mode 6, every pixel is just the RGB565 word. Those images should be exported with alpha 255 unless the record uses the 0x20000 background-color flag, in which case matching pixels can be treated as transparent.

```title="RGB565 expansion"
def rgb565_to_rgba(value, alpha=255):
    r5 = (value >> 11) & 0x1f
    g6 = (value >> 5) & 0x3f
    b5 = value & 0x1f

    r = (r5 << 3) | (r5 >> 2)
    g = (g6 << 2) | (g6 >> 4)
    b = (b5 << 3) | (b5 >> 2)
    return r, g, b, alpha
```

```title="Decode one bitmap record"
import struct
from PIL import Image

def decode_bitmap_record(buf, off):
    width, height, flags, bg = struct.unpack_from('<HHII', buf, off)
    mode, row_bytes = bitmap_row_bytes(width, flags)
    pos = off + 12
    pixels = []

    if mode == 7:
        for _ in range(width * height):
            color = struct.unpack_from('<H', buf, pos)[0]
            alpha = buf[pos + 2]
            pixels.append(rgb565_to_rgba(color, alpha))
            pos += 3
    elif mode in (4, 5, 6):
        bg_rgb565 = bg & 0xffff if flags & 0x20000 else None
        for _ in range(width * height):
            color = struct.unpack_from('<H', buf, pos)[0]
            alpha = 0 if color == bg_rgb565 else 255
            pixels.append(rgb565_to_rgba(color, alpha))
            pos += 2
    elif mode in (1, 2, 3):
        for _ in range(width * height):
            v = buf[pos]
            pixels.append((v, v, v, 255))
            pos += 1
    else:
        raise ValueError(f'unsupported mode {mode}')

    image = Image.new('RGBA', (width, height))
    image.putdata(pixels)
    return image, off + 12 + row_bytes * height
```

## Extract assets from Lemonade2.rb {#extract-assets}

Summary: The extractor should produce reviewable PNGs and preserve the raw bytes.

Takeaway: PNGs are for inspection. Raw records are for the next decoder fix.

The safest workflow extracts the installed game files from the setup archive, then parses Lemonade2.rb directly. Once segment 2 is decoded, each bitmap should be saved as a PNG with its index, offset, dimensions, mode, and flags in the filename.

Raw record bytes keep later decoder fixes cheap. If a later pass improves transparency handling or identifies a new mode, you can reprocess the records without carving the original container again.

A contact sheet also helps. Hundreds of tiny UI strips are hard to inspect one file at a time; one sheet exposes duplicates, masks, large backgrounds, and unknown art families faster.

```title="Walk the segment-2 bitmap stream"
def extract_graphics(rb_bytes, segment, out_dir):
    seg_type, start, end, count = segment
    assert seg_type == 2

    pos = start + 12
    for index in range(count):
        image, next_pos = decode_bitmap_record(rb_bytes, pos)
        image.save(out_dir / f'rb_bitmap_{index:03d}_off_{pos:08x}.png')

        raw = rb_bytes[pos:next_pos]
        (out_dir / 'raw_records' / f'rb_bitmap_{index:03d}.bin').write_bytes(raw)
        pos = next_pos

    if pos != end:
        raise ValueError(f'bitmap stream ended at 0x{pos:x}, expected 0x{end:x}')
```

```title="Find XM modules in the music segment"
def find_xm_modules(buf, segment):
    seg_type, start, end, _ = segment
    assert seg_type == 8

    needle = b'Extended Module: '
    starts = []
    pos = start
    while True:
        hit = buf.find(needle, pos, end)
        if hit < 0:
            break
        starts.append(hit)
        pos = hit + 1

    for i, xm_start in enumerate(starts):
        xm_end = starts[i + 1] if i + 1 < len(starts) else end
        yield xm_start, xm_end
```

## Reproduce the extraction {#reproduce-workflow}

Summary: The headline counts can be regenerated from the setup executable with a few command-line tools and the inline parsers on this page.

Takeaway: A useful reproduction should produce the same payload offsets, 12 segment records, 704 strings, 233 bitmap records, and two XM modules.

These commands assume `file`, `bzip2`, `strings`, `rg`, Python 3, Pillow, and Go are installed. `binwalk` helps with the first signature scan, but it is optional for the rest of the reproduction because the carved offsets are listed below.

Set `INPUT` to the original setup executable. The output folder can be any scratch directory. The page includes the critical parsers inline: the segment-chain parser, bitmap decoder, graphics walker, XM finder, HTTP request shapes, checksum formula, and date scalar.

The first pass records basic metadata. The second pass carves the embedded payloads. The final checks compare regenerated artifacts against the headline numbers in this write-up.

```bash title="Metadata and signature scan"
INPUT="/path/to/Lemonade Tycoon 2 - New York City.exe"
OUT="$PWD/lt2_repro"
mkdir -p "$OUT/payloads/compressed" "$OUT/payloads/decompressed" "$OUT/reports"

sha256sum "$INPUT" > "$OUT/reports/sha256.txt"
file "$INPUT" > "$OUT/reports/file.txt"
if command -v binwalk >/dev/null 2>&1; then
  binwalk "$INPUT" > "$OUT/reports/binwalk.txt"
else
  printf 'binwalk not installed; using documented carve offsets\n' > "$OUT/reports/binwalk.txt"
fi
strings -a -n 5 "$INPUT" > "$OUT/reports/installer_strings.txt"
```

```bash title="Carve the embedded installer payloads"
dd if="$INPUT" of="$OUT/payloads/compressed/stream_0x2D280.bz2" bs=1 skip=184960 count=14372 status=none
dd if="$INPUT" of="$OUT/payloads/compressed/stream_0xFEA4C.bz2" bs=1 skip=1043020 count=8072295 status=none
dd if="$INPUT" of="$OUT/payloads/compressed/stream_0x8B1F24.bz2" bs=1 skip=9117476 count=172707 status=none
dd if="$INPUT" of="$OUT/payloads/decompressed/tail_0x8B18C6.gif" bs=1 skip=9115846 count=702 status=none

bzip2 -dc "$OUT/payloads/compressed/stream_0x2D280.bz2" > "$OUT/payloads/decompressed/stream_0x2D280.decompressed"
bzip2 -dc "$OUT/payloads/compressed/stream_0xFEA4C.bz2" > "$OUT/payloads/decompressed/stream_0xFEA4C.decompressed"
bzip2 -dc "$OUT/payloads/compressed/stream_0x8B1F24.bz2" > "$OUT/payloads/decompressed/stream_0x8B1F24.decompressed"
```

```bash title="Expected reproduction counts"
segments=12
strings=704
rgb565a_image_candidates=82
bitmap_records=233
xm_modules=2
```

```bash title="Expected compatibility responses"
/createaccount.php -> response text contains ACCEPT
/syncgame.php      -> response text contains SUCCESS
```

## The browser DLL is only the window {#browser-boundary}

Summary: TeneonIERelease.dll owns the browser window, not the score uploader.

Takeaway: If you are looking for the score protocol, do not stop at the browser DLL. The browser displays LSX pages; the game code sends account and score requests itself.

TeneonIERelease.dll identifies itself as an MFC/C++ wrapper around an embedded Internet Explorer control. Its exports include `IEBrowserContainer::LoadURL`, `Back`, `Forward`, `Refresh`, `Print`, `Show`, and `Hide`, and its PDB path points to an `IEWebBrowser` project.

The browser wrapper receives a URL and navigates to it. The navigation call uses null post data and null custom headers. That rules out a hidden browser POST for the recovered score-upload mechanism.

This boundary explains the two web behaviors: the browser page shows/checks LSX, while the direct HTTP client handles account and score transfer. The DLL displays a page; the unpacked Lemonade2.exe runtime builds and sends the account and score GET requests.

The export names and simplified `LoadURL` behavior below show the DLL's role. It navigates to URLs with no post data and no custom headers, so the score upload has to live elsewhere.

```title="LoadURL behavior, simplified"
int LoadURL(container, url) {
    if (container->browser == NULL) return error;
    if (url == "") return error;

    return browser->Navigate(
        url,
        flags = 0,
        target_frame = 0,
        post_data = 0,
        headers = 0
    );
}
```

## The LSX browser page is a connection bridge {#browser-check-page}

Summary: The embedded browser starts with a local HTML file, checks one remote GIF, then redirects to the leaderboard.

Takeaway: A compatible server should serve the page endpoints and the tiny connection image even though score upload does not depend on browser POST data.

The game-side browser setup loads either `Lsx\CheckConnection.html` or `Lsx\CheckConnection.html?username=<username>` into the embedded browser. That local file tests whether `http://gt.jamdat.ca/img/lsx2/connection.gif` can load.

If the image check succeeds, the browser redirects to `http://gt.jamdat.ca/lsx2.php` or `http://gt.jamdat.ca/lsx2.php?username=<username>`. This is the visible LSX web experience: service check plus leaderboard. It is separate from the raw Winsock account and score transfer routines.

For compatibility, the server should implement `/img/lsx2/connection.gif` and `/lsx2.php` in addition to `/createaccount.php` and `/syncgame.php`. Without the image and leaderboard paths, the game can still send raw score requests, but the LSX browser panel will look broken.

The browser-side flow below gives the local file, remote image, and final LSX page URLs. That is the complete visible-browser contract needed by a replacement server.

```title="Browser-side flow"
game loads local file:
  Lsx\CheckConnection.html
  Lsx\CheckConnection.html?username=<username>

local file checks remote image:
  http://gt.jamdat.ca/img/lsx2/connection.gif

on image load, redirect browser to:
  http://gt.jamdat.ca/lsx2.php
  http://gt.jamdat.ca/lsx2.php?username=<username>
```

## The LSX protocol is plain HTTP {#lsx-protocol}

Summary: The game talks to `gt.jamdat.ca` with plain HTTP GET requests.

Takeaway: A compatible server does not need JSON, cookies, XML, or PHP source parity. The client only looks for ACCEPT and SUCCESS in raw response text.

The account check builds a GET request to `/createaccount.php`. The score upload builds a larger GET request to `/syncgame.php?game=lemonade2`. Both send `Accept: text/plain` and `Host:gt.jamdat.ca`.

The transport is not WinINet and not the embedded browser. The unpacked runtime uses a small Winsock helper: initialize Winsock, open a TCP socket, resolve `gt.jamdat.ca`, connect to port 80, send a prebuilt request buffer, receive roughly 4000 bytes, then shut the socket down.

Company and CEO names use a small URL encoder before they are appended to the query string. Alphanumeric bytes pass through unchanged, spaces become `+`, and other bytes become uppercase `%XX` escapes.

The response parser is loose. For account creation, any response containing `ACCEPT` is accepted. For score upload, any response containing `SUCCESS` is accepted. The client does not parse status codes, JSON, XML, cookies, or a structured validation object.

The raw request templates and response-token checks below show the full client-facing protocol. The server only has to satisfy those strings for the recovered client path.

```title="Low-level HTTP helper shape"
SendHttpRequestToHost(host, request_response_buffer):
    WSAStartup()
    sock = socket(AF_INET, SOCK_STREAM, 0)
    addr = gethostbyname(host)
    connect(sock, addr, port=80)
    send(sock, request_response_buffer)
    recv(sock, request_response_buffer, about_4000_bytes)
    shutdown(sock)
    closesocket(sock)
    WSACleanup()
```

```title="Account check request"
GET /createaccount.php?username=<username>&password=<password> HTTP/1.1
Accept: text/plain
Host:gt.jamdat.ca
```

```title="Career upload request"
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

```title="Response token checks"
account_ok = http_request_ok && strstr(response, "ACCEPT") != NULL;
upload_ok  = http_request_ok && strstr(response, "SUCCESS") != NULL;
```

```title="URL encoding rule"
if byte is ASCII letter or digit:
    append byte
elif byte == ' ':
    append '+'
else:
    append '%' + uppercase_hex(byte)
```

## The upload checksum and date scalar {#checksum-date}

Summary: The upload sends `checksumclient`, but the client only needs the success token.

Takeaway: stockassets is sent but not part of the recovered checksum formula.

The score object carries the values that become query parameters. The client computes `checksumclient` from most of those values with normal x86 32-bit integer arithmetic. Overflow behavior is part of compatibility, not a bug to clean up.

`gamestartingdate` is not a Unix timestamp. It is a packed game date converted with a fixed 360-day year and 30-day month calendar. The same scalar is sent in the query and used in the checksum.

The source date occupies eight 16-bit words in the score object. The scalar helper uses year, month, day, hour, minute, second, and millisecond; one word between hour and minute is copied in the structure but not used by the recovered scalar formula.

The surprising field is `stockassets`. The upload sends it to the server, and the leaderboard detail page renders stock assets later, but the recovered checksum formula does not include it.

The checksum table, formula, fixed-calendar scalar, and sample request later in the dynamic-probes section show the recovered arithmetic directly. In that sample, the server-computed checksum is `46000`.

| Query field      | Checksum role                        |
| ---------------- | ------------------------------------ |
| gamemode         | adds gamemode * 7                    |
| gamegoal         | adds gamegoal * 5                    |
| gamestartingdate | main date multiplier                 |
| lifespan         | main lifespan multiplier             |
| stands           | subtracts stands * 100 from revenues |
| cupssold         | subtracted                           |
| cashassets       | added                                |
| stockassets      | sent but not used                    |
| standsassets     | subtracted                           |
| upgradesassets   | added                                |
| retainedearnings | added                                |
| revenues         | input to the revenue term            |

```title="Checksum formula"
checksum =
    gamestartingdate *
    (revenues - stands * 100) *
    lifespan
  + gamegoal * 5
  - standsassets
  - cupssold
  + gamemode * 7
  + retainedearnings
  + upgradesassets
  + cashassets;
```

```title="Fixed-calendar date scalar"
gamestartingdate =
    (((year * 360 + month * 30 + day) * 24 + hour) * 60 + minute)
    * 60 * 1000
    + second * 1000
    + millisecond;
```

```title="Packed date words"
score + 0x32:
  +0x00 year
  +0x02 month
  +0x04 day
  +0x06 hour
  +0x08 copied date word, not used by scalar helper
  +0x0a minute
  +0x0c second
  +0x0e millisecond
```

## Saved careers can be queued for upload {#upload-queue}

Summary: The game can upload the current career or queued saved careers, but both paths end at the same request builder.

Takeaway: The queue changes when uploads happen, not what the server receives.

The queue producer scans `*.dat` career save files, skips directories and dot-prefixed entries, validates the LT2 save header, copies each valid save into a 0x70-byte score summary, and appends that summary pointer to the LSX state object.

The save loader rejects header flag/type 1 and accepts normal saves written with flag 0. For LSX compatibility, that flag is a local queueing filter rather than a server field. A rejected save never reaches the pending upload vector.

The queue consumer walks the vector and calls the same upload routine used by the direct current-career path. It filters entries whose recovered `dwGameMode` field is zero before calling `UploadCareerScoreToLsx`.

The score summary contains two trailing copied dwords at +0x68 and +0x6c. They are copied by the direct and queued snapshot paths, and queued save loads default their source values to 10 and 10, but they are not emitted in the recovered `/syncgame.php` query.

The direct and queued upload flows below both end at `UploadCareerScoreToLsx`. That is the server-relevant fact: the queue changes timing, not the request format.

```title="Direct upload path"
post-score screen
  -> upload confirmation
  -> UploadCurrentCareerDirect
  -> UploadCareerScoreToLsx
```

```title="Queued upload path"
prepare queue:
    scan *.dat career saves
    load valid save
    copy save into LsxScoreSummary
    append summary pointer to upload vector

process queue:
    for each queued summary:
        if summary.dwGameMode == 0:
            UploadCareerScoreToLsx(summary)
```

## The public leaderboard shape is recoverable {#leaderboard}

Summary: The PHP source is gone, but archived rendered pages recover the leaderboard contract.

Takeaway: The replacement server should preserve the visible page contract, even without the original ranking internals.

Wayback captures of `http://gt.jamdat.ca/lsx2.php` from 2004 through 2009 show the public LSX page as a compact 675 px-style table on a pale green background. The visible columns are rank, company name, CEO, lifespan, and market cap.

The page accepted controls such as pagenum, sort, gamemode, gamegoal, ranktype, and username. That recovers the visible web interface even though the original ranking implementation is still unavailable.

Company rows opened a detail endpoint with d1 through d18 query parameters. A replay of an archived detail page leaked PHP notices from `D:\www\Hexacto.com\gt\lsx2_detail.php`, which helped recover the detail field layout. One important correction: d6 is the total number of entries in the current leaderboard view, not the game's start-date scalar.

The visible columns, accepted controls, detail endpoint shape, and d1 through d18 field map below describe the recovered page contract directly.

| Detail parameter | Meaning                       |
| ---------------- | ----------------------------- |
| d1               | company                       |
| d2               | CEO                           |
| d3               | mode                          |
| d4               | goal label                    |
| d5               | rank                          |
| d6               | total entries in current view |
| d7               | title                         |
| d8               | lifespan                      |
| d9               | stands                        |
| d10              | cups sold                     |
| d11              | market cap                    |
| d12              | revenues                      |
| d13              | retained earnings             |
| d14              | percent field                 |
| d15              | cash assets                   |
| d16              | stock assets                  |
| d17              | stand assets                  |
| d18              | upgrade assets                |

## A compatible server can stay small {#compatible-server}

Summary: A replacement server does not need to clone the old PHP application.

Takeaway: Store what the game sends, return the token it expects, and render a leaderboard in the recovered shape.

For account checks, the game only needs the server to store the username/password probe and return `ACCEPT`. For score uploads, the recovered client path only needs the server to store the query fields and return `SUCCESS`.

Checksum validation is useful, but pure compatibility should not require it. The original client does not parse a server validation object. It only checks whether the response contains the success token.

The minimum server behavior follows directly from the visible client checks: return text containing `ACCEPT` for account probes and text containing `SUCCESS` for score uploads. Checksum validation can be layered on top of that without changing the compatibility response.

The handler shapes below show the minimum compatible behavior: accept account probes, store score submissions, optionally compute the checksum, and return the token the client scans for.

```title="Account handler shape"
func handleCreateAccount(query) Response {
    storeAccountProbe(query["username"], query["password"])
    return text("ACCEPT\n")
}
```

```title="Score handler shape"
func handleSync(query) Response {
    computed := computeRecoveredChecksum(query)
    storeSubmission(query, computed)

    if strictChecksum && query["checksumclient"] != computed {
        return text("FAIL\n")
    }
    return text("SUCCESS\n")
}
```

## The account check is tied to options.dat {#account-options}

Summary: LSX account state also lives in `options.dat`.

Takeaway: The first account request can happen before any score upload because startup/account initialization reads options.dat and may call the account endpoint immediately.

The account object stores the LSX username at offset +0x14, password at +0x20, and a status flag at +0x2c. The account check sets that flag to accepted only when the response contains ACCEPT.

The installer-provided options file contains placeholder credentials: username QQQQQQQQQQQQQQQQQ and password WWWWWWWWWWWWWWWWW. That explains why an account probe can appear even in a passive startup run.

This matters for server testing: seeing `/createaccount.php` does not prove a player clicked an LSX upload button. It can simply mean the game initialized account state.

The recovered option fields and captured startup request below show why `/createaccount.php` can appear during startup, before a manual score upload.

```title="Recovered account option fields"
struct LsxAccountOptions {
    /* +0x14 */ string username;
    /* +0x20 */ string password;
    /* +0x2c */ bool account_check_failed;
};
```

```title="Startup account check shape"
load options.dat
if options missing or account not accepted:
    GET /createaccount.php?username=<saved>&password=<saved>
    if response contains ACCEPT:
        account_check_failed = false
```

## The upload callback byte is not a clean success flag {#callback-quirk}

Summary: The upload callback byte is easy to misread.

Takeaway: For server compatibility, the reliable rule is still the response token: return text containing SUCCESS.

The upload routine dispatches completion callbacks after the HTTP helper returns. The byte forwarded to those callbacks is 1 when SUCCESS appears in the response, 0 when the HTTP helper completes but SUCCESS is absent, and also 1 when the HTTP helper itself fails.

The callback value is not a clean server-accepted flag. The meaningful server contract is the substring check inside the upload builder: the response must contain SUCCESS.

The callback pseudocode below shows why the callback byte cannot be treated as a clean success flag.

```title="Callback byte behavior"
if SendHttpRequestToHost(host, response) != 1:
    callback_byte = 1
elif strstr(response, "SUCCESS") != NULL:
    callback_byte = 1
else:
    callback_byte = 0
```

## Dynamic probes confirm the startup path {#dynamic-evidence}

Summary: Runtime probes confirmed startup account traffic and the low-level HTTP helper. A natural click-through score upload would still help, but the server contract does not depend on it.

Takeaway: Static recovery is strong enough for compatibility, but confirmed live behavior and inferred UI timing should stay separate.

A Wine/Xvfb startup probe captured the game sending `/createaccount.php` with the placeholder credentials from `options.dat`. That proves the recovered account function is live and explains why an account request can appear before the player manually uploads a score.

A later runtime helper call sent a synthetic `/syncgame.php` request from inside the Wine-hosted process. The captured request below confirms the raw helper, endpoint shape, query fields, and checksum behavior in a running game process.

That synthetic score request was rejected only because the injected `checksumclient=0` did not match the recovered formula; the server computed 46000 for those sample fields. This supports the checksum implementation, but it is not the same as a natural click-through upload from the normal UI.

Several UI-driving attempts reached a career or opened LSX but still captured only `/createaccount.php`. The endpoint, query shape, and response token are recovered; a natural high-level `/syncgame.php` capture would mainly add sample values and timing.

The captured startup request and synthetic helper score request below show the live account path, the score query fields, and the computed checksum for one sample.

```title="Captured startup account request"
GET /createaccount.php?username=QQQQQQQQQQQQQQQQQ&password=WWWWWWWWWWWWWWWWW
Accept: text/plain
Host: gt.jamdat.ca
```

```title="Synthetic helper score request"
game=lemonade2
username=helperprobe
password=helperprobe
companyname=HelperProbeCo
ceoname=HelperProbeCEO
gamemode=0
gamegoal=0
gamestartingdate=0
lifespan=1
stands=1
cupssold=0
cashassets=49000
stockassets=0
standsassets=3000
upgradesassets=0
retainedearnings=0
revenues=0
checksumclient=0

server computed checksum: 46000
```

## Some visible art is still not explained {#missing-art}

Summary: Some visible game art still has no clean decoded source.

Takeaway: The 233 decoded records are real, but they are not the end of the asset story.

Screenshots of the Windows release show main menu art, city scenes, weather strips, stand controls, and large CEO portraits. Several of those visible elements do not match the currently decoded segment-2 bitmap set.

This was checked from several angles. TeneonIERelease.dll has no PE resources. Direct PNG, GIF, and JPEG carving did not find the portraits. Row-run checks for captured portrait crops did not match Lemonade2.rb, Lemonade2.exe, or TeneonIERelease.dll in common RGB, BGR, RGBA, BGRA, or RGB565 layouts.

That does not mean the art is missing from the game. It means the current decoder has not found the right source path. The portraits may be composed at runtime, stored in a still-unnamed segment, or encoded in a format this pass does not handle.

That is why the extraction workflow preserves raw records, contact sheets, and runtime portrait crops. The decoded PNGs are the current interpretation; the raw bytes keep later passes possible.

The negative checks are listed in the paragraphs above: no PE resources in the browser DLL, no direct PNG/GIF/JPEG portrait carve, and no row-run match for captured portrait crops in common color layouts.

## What is still unknown {#open-questions}

Summary: The compatibility contract is solid enough to implement, but the preservation story is not finished.

The original server-side PHP for account creation, score upload, and ranking has not been recovered. Exact Wayback checks for `syncgame.php` and `createaccount.php` came back empty, while broader Jamdat/Hexacto PHP searches did not expose source. Exact validation rules, database schema, and ranktype behavior are still inferred from the client and rendered pages.

A natural in-game score-upload capture would still be useful for sample values and timing. It is not needed to identify the endpoint, query shape, response token, or checksum formula.

The large CEO portraits visible in runtime screenshots are still not explained by the decoded 233-record graphics stream. They may use another encoding path, another runtime composition path, or data that has not been identified yet.

Several smaller resource-container segments have stable offsets but still lack good semantic names. They are not required for LSX server compatibility, but they are still part of the preservation story.
