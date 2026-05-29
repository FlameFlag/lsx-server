/*
 * Lemonade Tycoon 2 installer License Manager / Registration DRM subsystem.
 *
 * This setup EXE target is deliberate: only explicit license.* findings should
 * point at it.
 *
 * This is the installer-side code that reads, validates, and decrypts
 * the user's registration key. The registration check gates the installer
 * from proceeding to extract the game files.
 *
 * The "Your temporary key has expired" message originates from the protected
 * Lemonade2.exe runtime (Armadillo-style protector), but the installer itself
 * performs an independent license check against a hardcoded validation table.
 *
 * Addresses are scoped to the installer image base 0x00400000.
 */

#ifndef LT2_INSTALLER_LICENSE_MANAGER_H
#define LT2_INSTALLER_LICENSE_MANAGER_H

#include "common/win32_recovered_types.h"

/* Max length of a parsed registration key in the #UserRegCode# buffer. */
#define LT2_REGCODE_MAX_LEN  0x29e  /* 670 bytes */

/* reginfo.txt read limit (FUN_00408b00). */
#define LT2_REGINFO_MAX_READ 20000

/* Large file read limit (FUN_004084c0). */
#define LT2_REGINFO_LARGE_MAX 0x1ffb8

/* Minimum registration key length in #S#...#E# format. */
#define LT2_REGCODE_MIN_LEN  6

/* Valid key length range (FUN_004059c0). */
#define LT2_KEY_MIN_LEN      0x10
#define LT2_KEY_MAX_LEN      0x18

typedef enum Lt2RegCodeType {
    LT2_RC_TYPE_A = 0,   /* #S#...#E# delimited, 8-byte XOR key */
    LT2_RC_TYPE_B = 1,
    LT2_RC_TYPE_C = 2,
    LT2_RC_TYPE_D = 3,   /* plain text, hash-based */
    LT2_RC_TYPE_E = 4,   /* plain text, numeric-only */
    LT2_RC_TYPE_COUNT
} Lt2RegCodeType;

typedef enum Lt2LicenseResult {
    LT2_LICENSE_OK            = 0,
    LT2_LICENSE_BAD_FORMAT    = 1,
    LT2_LICENSE_HASH_FAIL     = 2,
    LT2_LICENSE_CHECKSUM_FAIL = 3,
    LT2_LICENSE_LENGTH_FAIL   = 4,
} Lt2LicenseResult;

/*
 * Packed hash table entry (FUN_00405790, FUN_00407850). Table entries start
 * at byte offset 5, immediately after entry_count and tier.
 *
 * Hash formula:
 *   forward: for each byte b: hash = (hash * 2 - (hash >> 31)) + b
 *   reverse: same scanning from end to beginning
 */
typedef struct Lt2HashEntry {
    u32 code_length;
    u32 hash_forward;
    u32 hash_reverse;
    u8  xor_key_start[8];
    u8  xor_key_end[8];
} Lt2HashEntry;

typedef struct Lt2HashTable {
    u32 entry_count;
    u8  tier;
} Lt2HashTable;

#define LT2_HASH_TABLE_HEADER_SIZE 5u

/* Installer file record flags (offset +0x0D of file descriptor). */
#define LT2_FILE_FLAG_ZLIB        0x0001
#define LT2_FILE_FLAG_SKIP_VALID  0x0002
#define LT2_FILE_FLAG_SET_TIMES   0x0008
#define LT2_FILE_FLAG_SFX         0x0020
#define LT2_FILE_FLAG_SKIP_EXIST  0x0040
#define LT2_FILE_FLAG_ALWAYS_SKIP 0x0100
#define LT2_FILE_FLAG_OS_GATED    0x0200
#define LT2_FILE_FLAG_REBOOT      0x0400
#define LT2_FILE_FLAG_POST_ACTION 0x0800
#define LT2_FILE_FLAG_EXECUTE     0x1000
#define LT2_FILE_FLAG_LICENSED    0x2000
#define LT2_FILE_FLAG_DOWNLOAD    0x200000

/* Registration key input sources */
BOOL lt2_license_read_reginfo(void);
void lt2_license_read_reginfo_alt(void);
BOOL lt2_license_read_file(const char *path);
BOOL lt2_license_read_large_file(const char *path);

/* Registration key parsing */
void lt2_license_parse_plain(char *text);
void lt2_license_parse_delimited(char *text, u32 text_len);
void lt2_license_parse_filtered(byte *text, int text_len, int numeric_only);

/* License validation */
BOOL lt2_license_validate_hash(void);
BOOL lt2_license_hash_lookup(byte *code, u32 code_len, int xor_key_len);

/* License decryption engine */
u32 lt2_license_decrypt(byte *output, u32 output_size,
                        const byte *input, int mode);

/* Template variable resolution */
void lt2_license_resolve_template(byte *template_var);
void lt2_license_decrypt_config_field(byte *output, const byte *source);

/* WinINet online check system */
int  lt2_wininet_open_session(void);
void lt2_wininet_shutdown(void);
int  lt2_copy_or_download_file(char *local_path, char *source_url,
                               u32 source_size, u32 progress_start,
                               u32 progress_end);

/* Installer UI registration dialog */
u32 lt2_license_dialog_process(HANDLE hwnd_dialog);

#endif /* LT2_INSTALLER_LICENSE_MANAGER_H */
