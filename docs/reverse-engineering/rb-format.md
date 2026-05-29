# Lemonade2.rb Format Reconstruction

Status: byte-level container map complete for the recovered
`decompiled/local/lt2_install/Lemonade2.rb` payload. The top-level RB format,
segment chain, segment-local record boundaries, image/string/audio/music
payloads, image-info table, font/glyph tables, and nested zlib splash screen
are now structurally accounted for. A few gameplay-table field names remain
semantic labels rather than loader-symbol names because the protected game
runtime has not yielded the original RB-loader function.

## Evidence

- File: `./lsx-server/decompiled/local/lt2_install/Lemonade2.rb`
- Size: `27,886,734` bytes (`0x01A9848E`)
- Installer source stream: bzip2 at installer offset `0xFEA4C`, length
  `8,072,295`
- Ghidra MCP check, 2026-05-29: the loaded `Lemonade2.exe` program still has
  one analyzed function, the protected entry stub. Its imports include
  `CreateFileA`, `ReadFile`, and `CreateDIBitmap`, but no decompilable RB loader
  is currently available.

## Top-Level Layout

`Lemonade2.rb` is a little-endian resource bundle with a fixed pre-segment area
followed by a segment chain.

| Range | Meaning | Notes |
| --- | --- | --- |
| `0x00000000..0x00001037` | application/static numeric table | Starts with `ff ff 20 03 58 02 10 00`; includes the segment-chain offset at `0x10` and a 1,032-dword numeric table. |
| `0x00001038..EOF` | segment chain | Each segment starts with a 12-byte header. |

The pre-segment area starts with a 24-byte header:

```c
struct Lt2RbPreamble {
    int16_t sentinel;          /* -1 */
    int16_t width;             /* 800 */
    int16_t height;            /* 600 */
    int16_t format_or_depth;   /* 16 */
    uint32_t signature;        /* 0x848EDDC3 */
    uint32_t reserved0;        /* 0 */
    uint32_t segment_offset;   /* 0x1038 */
    uint32_t dword_count;      /* 0x408 */
    uint32_t dwords[0x408];    /* ends exactly at 0x1038 */
};
```

Segment header:

```c
struct Lt2RbSegmentHeader {
    uint32_t type;
    uint32_t next_offset;    /* absolute file offset */
    uint32_t count_or_flags;
};
```

The segment walk starts at `0x1038` and stops when `next_offset == file_size`.
Every observed `next_offset` is forward and in bounds.

## Segment Chain

| Type | Offset | Next | Payload bytes | Count/flags | Decoded status |
| --- | ---: | ---: | ---: | ---: | --- |
| `1` | `0x001038` | `0x007288` | `25,156` | `512` | string table, 704 strings |
| `2` | `0x007288` | `0x1472D22` | `21,412,494` | `233` | bitmap/image records |
| `3` | `0x1472D22` | `0x14F4ABA` | `531,852` | `22` | five PCM sample records |
| `4` | `0x14F4ABA` | `0x14F92C8` | `18,434` | `1` | image-info/mosaic coordinate table |
| `5` | `0x14F92C8` | `0x14FBC82` | `10,670` | `4` | four bitmap-font glyph tables |
| `6` | `0x14FBC82` | `0x14FBC8E` | `0` | `0` | empty |
| `7` | `0x14FBC8E` | `0x14FBC9A` | `0` | `0` | empty |
| `8` | `0x14FBC9A` | `0x1A98452` | `5,883,820` | `15` | length-prefixed binary/music records |
| `13` | `0x1A98452` | `0x1A9845E` | `0` | `0` | empty |
| `11` | `0x1A9845E` | `0x1A9846A` | `0` | `0` | empty |
| `12` | `0x1A9846A` | `0x1A98476` | `0` | `0` | empty |
| `10` | `0x1A98476` | `0x1A9848E` | `12` | `1` | footer-like record `{0, file_size, 0}` |

## Type 1: Strings

Type 1 has a secondary header after the segment header:

```c
struct Lt2RbStringSegment {
    uint32_t string_count;  /* 704 */
    uint32_t data_offset;   /* 0x1050 */
    uint32_t data_size;     /* 0x6238 */
};

struct Lt2RbStringRecord {
    uint32_t byte_length;
    uint8_t cp1252_bytes[byte_length];
};
```

The segment header's `count_or_flags` is `512`; it is not the string count.
Strings decode as Windows CP1252 byte strings and are not NUL-terminated unless
the string data itself includes a NUL.

## Type 2: Bitmaps

Type 2 is a run of `count_or_flags` bitmap records. Record boundaries are fully
deterministic from dimensions and pixel format.

```c
struct Lt2RbBitmapRecord {
    uint16_t width;
    uint16_t height;
    uint32_t format;
    uint32_t flags;
    uint8_t pixels[width * height * bytes_per_pixel(format)];
};
```

Observed formats:

| Format low word | Count | Meaning |
| --- | ---: | --- |
| `0x8760` | 187 | RGB565 plus one alpha byte per pixel |
| `0x0660` | 37 | RGB565 |
| `0x20660` | 4 | RGB565 with high flag bits set; same pixel size as `0x0660` |
| `0xC300` | 5 | 8-bit grayscale |

For RGB565 records, nonzero `flags` is an RGB chroma key encoded as
`0x00RRGGBB`. The current extractor treats matching pixels as transparent.

## Type 3: PCM Samples

Type 3 contains five signed 16-bit little-endian mono PCM sample records. Each
record begins with:

```c
struct Lt2RbPcmHeader {
    uint32_t sample_rate; /* always 11008 in this RB */
    uint32_t tag;         /* always 18 in this RB; semantic name unknown */
};
```

The sample payload continues until the next `{11008, 18}` header or the end of
the type-3 segment. This is corroborated by valid exported WAV files, smooth
PCM-like sample data, even byte-aligned payload lengths, and duplicate
first/last sample hashes. The segment header count/flags value `22` is not the
embedded sample count; it matches the Clickteam-style bank convention of a
maximum sound handle count rather than the number of stored payloads.

| Index | Header offset | PCM bytes | Duration at 11008 Hz |
| --- | ---: | ---: | ---: |
| `0` | `0x1472D2E` | `38,264` | `1.738s` |
| `1` | `0x147C2AE` | `38,916` | `1.768s` |
| `2` | `0x1485ABA` | `22,014` | `1.000s` |
| `3` | `0x148B0C0` | `394,354` | `17.912s` |
| `4` | `0x14EB53A` | `38,264` | `1.738s` |

## Type 4: Image Info / Mosaic Coordinates

Type 4 is an image-info table with implicit image handles. It consumes exactly:

```c
struct Lt2RbImageInfoSegment {
    uint16_t max_image_handle_plus_1; /* 1152 */
    Lt2RbImageInfo image[1152];       /* implicit handle == array index */
};

struct Lt2RbImageInfo {
    int16_t width;      /* always 30 in this table */
    int16_t height;     /* always 40 in this table */
    int16_t x_spot;
    int16_t y_spot;
    int16_t x_action_point;
    int16_t y_action_point;
    int16_t mosaic_x;
    int16_t mosaic_y;
};
```

This matches Clickteam runtime concepts `width`, `height`, `xSpot`, `ySpot`,
`xAP`, `yAP`, `mosaicX`, and `mosaicY`. All 1,152 `(mosaic_x, mosaic_y)`
pairs are unique, and every `mosaic_x + width` / `mosaic_y + height` rectangle
fits inside an 800x600 atlas.

## Type 5: Bitmap Font Glyph Tables

Type 5 is four bitmap-font subrecords over signed little-endian 16-bit values.
It consumes exactly 5,335 shorts:

```c
struct Lt2RbGlyphTable {
    int16_t global_y_offset_or_baseline;
    int16_t glyph_count;
    int16_t atlas_width;              /* 768 */
    int16_t font_or_style_id;
    Lt2RbGlyph glyph[glyph_count];
};

struct Lt2RbGlyph {
    int16_t character_code;           /* CP1252-style code point */
    int16_t source_x;
    int16_t source_y;
    int16_t x_offset;
    int16_t y_offset;
    int16_t width_or_cell_width;
    int16_t height;
    int16_t advance;
    int16_t baseline_or_y_advance;
};
```

| Subrecord | Relative short index | Relative byte offset | First shorts |
| --- | ---: | ---: | --- |
| `0` | `0` | `0x0000` | `-13, 12, 768, 0, 32, 41, 0, 0` |
| `1` | `112` | `0x00E0` | `-14, 193, 768, 2, 32, 42, 10, 0` |
| `2` | `1853` | `0x0E7A` | `-26, 193, 768, 7, 32, 69, 15, 0` |
| `3` | `3594` | `0x1C14` | `-15, 193, 768, 5, 32, 28, 6, 0` |

Subrecord 0 maps space, digits, and colon. Subrecords 1-3 each map 193
CP1252-style character codes from `32..255` with gaps for unused codes.

## Type 8: Binary/Music Records

Type 8 is a sequence of `count_or_flags` length-prefixed binary records:

```c
struct Lt2RbBlobRecord {
    uint32_t byte_length;
    uint8_t data[byte_length];
};
```

All 15 records consume the segment exactly. Records `0` and `2` are FastTracker
II XM modules; record `0` contains sample names such as `Amb_GCS_cart.wav`,
`Amb_GCS_intercom.wav`, and `Amb_Bronx_loop.wav`.

| Index | Data offset | Length | Identification |
| --- | ---: | ---: | --- |
| `0` | `0x14FBCAA` | `4,772,618` | FastTracker II module |
| `1` | `0x1988FB8` | `43,736` | binary data |
| `2` | `0x1993A94` | `1,036,218` | FastTracker II module |
| `3` | `0x1A90A52` | `428` | binary data |
| `4` | `0x1A90C02` | `240` | binary data |
| `5` | `0x1A90CF6` | `3,082` | binary data |
| `6` | `0x1A91904` | `210` | binary data |
| `7` | `0x1A919DA` | `18` | binary data |
| `8` | `0x1A919F0` | `384` | binary data |
| `9` | `0x1A91B74` | `25,200` | binary data |
| `10` | `0x1A97DE8` | `326` | binary data |
| `11` | `0x1A97F32` | `80` | binary data |
| `12` | `0x1A97F86` | `180` | binary data |
| `13` | `0x1A9803E` | `912` | binary data |
| `14` | `0x1A983D2` | `128` | binary data |

### Type 8 Record 1: Nested Zlib/RGB565 Screen

Record 1 is not opaque. It is an 800x600 screen made from zlib-compressed
RGB565 rectangles. Composing the non-empty entries recreates the JAMDAT Mobile
splash screen.

```c
struct Lt2RbZlibScreen {
    uint32_t kind0;              /* 1 */
    uint32_t screen_id;          /* 1 */
    uint32_t width;              /* 800 */
    uint32_t height;             /* 600 */
    uint32_t format_or_bpp;      /* 4 */
    uint32_t max_entry_handle;   /* 60; valid entry ids are 1..59 */
    uint32_t reserved[4];        /* zero */
    Lt2RbZlibScreenEntry entries_until_eof[];
};

struct Lt2RbZlibScreenEntry {
    uint32_t type;               /* 2 */
    uint32_t handle;             /* 1..59 */
    uint32_t x;
    uint32_t y;
    uint32_t width;
    uint32_t height;
    uint32_t total_size;         /* zero for empty entry, else comp_len + 12 */
    uint32_t reserved[5];        /* zero */
    /* present only when total_size != 0 */
    uint32_t comp_len;
    uint32_t raw_len;            /* width * height * 2 */
    uint32_t checksum_or_key;    /* observed 0x19741227 */
    uint8_t zlib_rgb565[comp_len];
    uint8_t zero_padding_to_4_byte_alignment[];
};
```

Record 1 contains 59 entries: 32 zlib/RGB565 rectangles and 27 empty entries.
Every non-empty entry inflates to exactly `width * height * 2` bytes.

## Current Confidence

Container boundaries, segment walking, strings, bitmaps, PCM sample boundaries,
image-info records, glyph table boundaries, empty segments, type-8 blob
boundaries, the nested zlib/RGB565 screen, and the type-10 footer are high
confidence because they consume exact byte ranges with deterministic checks.

The remaining uncertainty is not the RB container format; it is semantic naming
inside small game-data payloads where the original loader symbols would tell us
whether a field should be called "mode", "style", "baseline", or another
runtime-specific name. The current Ghidra MCP session confirms that loader is
still hidden behind the protected `Lemonade2.exe` entry stub.
