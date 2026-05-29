/* Lemonade Tycoon 2 installer License Manager / Registration DRM subsystem. */

#include "installer/installer_license_manager.h"

#include <stdlib.h>
#include <string.h>

/* Internal globals: installer .data section 0x00424000-0x00429FFF. */

/* DAT_00429328 @ 0x00429328 - #UserRegCode# buffer. */
static byte g_reg_code[LT2_REGCODE_MAX_LEN];

/* DAT_00429650 @ 0x00429650 - Installer configuration blob. */
static byte g_config[256];

/* DAT_004293d0 @ 0x004293d0 - Hash table array (one per license tier). */
static void *g_hash_tables[16];

/* DAT_00429758 @ 0x00429758 - Current hash table index. */
static u32 g_hash_table_index;

/* DAT_00429754 @ 0x00429754 - Max hash table index. */
static s32 g_max_hash_table_index;

/* DAT_004296e0 @ 0x004296e0 - License validation result flag. */
static s32 g_license_valid;

/* DAT_00429690 @ 0x00429690 - Source directory path. */
static char g_source_dir[260];

/* DAT_004292bc @ 0x004292bc - Working string buffer. */
static char g_work_buf[520];

/* DAT_00429740 @ 0x00429740 - Parent window handle. */
static HANDLE g_parent_hwnd;

/* DAT_00424034 @ 0x00424034 - Remaining license check attempts. */
static s32 g_license_attempts;

/* WinINet lazy-loaded function pointers. */
static HMODULE g_wininet_module;
static HANDLE g_hInternet;

typedef HANDLE (__stdcall *LPInternetOpenA)(char *, DWORD, char *, char *, DWORD);
typedef BOOL  (__stdcall *LPInternetCloseHandle)(HANDLE);
typedef HANDLE (__stdcall *LPInternetOpenUrlA)(HANDLE, char *, char *, DWORD, DWORD, DWORD);
typedef BOOL  (__stdcall *LPInternetQueryDataAvailable)(HANDLE, DWORD *, DWORD, DWORD);
typedef BOOL  (__stdcall *LPInternetReadFile)(HANDLE, void *, DWORD, DWORD *);

static LPInternetOpenA              g_pInternetOpenA;
static LPInternetCloseHandle        g_pInternetCloseHandle;
static LPInternetOpenUrlA           g_pInternetOpenUrlA;
static LPInternetQueryDataAvailable g_pInternetQueryDataAvailable;
static LPInternetReadFile           g_pInternetReadFile;

/* Imported Windows API and installer engine helpers. */
extern HANDLE __stdcall CreateFileA(LPCSTR, DWORD, DWORD, void *, DWORD, DWORD, HANDLE);
extern BOOL  __stdcall ReadFile(HANDLE, void *, DWORD, DWORD *, void *);
extern BOOL  __stdcall WriteFile(HANDLE, const void *, DWORD, DWORD *, void *);
extern BOOL  __stdcall CloseHandle(HANDLE handle);
extern BOOL  __stdcall CopyFileA(LPCSTR src, LPCSTR dst, BOOL fail_if_exists);
extern HMODULE __stdcall LoadLibraryA(LPCSTR name);
extern void * __stdcall GetProcAddress(HMODULE module, LPCSTR name);
extern BOOL  __stdcall FreeLibrary(HMODULE module);
extern void  __stdcall GetWindowsDirectoryA(LPSTR buf, UINT size);
extern void  __stdcall GetSystemDirectoryA(LPSTR buf, UINT size);
extern void  __stdcall GetTempPathA(DWORD size, LPSTR buf);
extern int   __stdcall lstrlenA(LPCSTR s);
extern void  __stdcall lstrcatA(LPSTR dst, LPCSTR src);
extern BOOL  __stdcall IsClipboardFormatAvailable(UINT format);
extern BOOL  __stdcall OpenClipboard(HANDLE hwnd);
extern void * __stdcall GetClipboardData(UINT format);
extern void * __stdcall GlobalLock(void *mem);
extern BOOL  __stdcall GlobalUnlock(void *mem);
extern BOOL  __stdcall CloseClipboard(void);
extern char * __stdcall lstrcpyA(LPSTR dst, LPCSTR src);
extern int   __cdecl lstrcmpiA(LPCSTR a, LPCSTR b);
extern LONG  __stdcall RegOpenKeyA(HANDLE key, LPCSTR sub, HANDLE *result);
extern LONG  __stdcall RegQueryValueExA(HANDLE key, LPCSTR name, DWORD *res,
    DWORD *type, BYTE *data, DWORD *data_len);
extern LONG  __stdcall RegCloseKey(HANDLE key);
extern HANDLE __stdcall GetDlgItem(HANDLE dlg, int id);
extern u32   __stdcall GetDlgItemTextA(HANDLE dlg, int id, LPSTR buf, int max);
extern u32   __stdcall SendDlgItemMessageA(HANDLE dlg, int id, UINT msg,
    WPARAM wp, LPARAM lp);
extern HANDLE __stdcall GetFocus(void);

/* Internal helpers from the installer engine. */
extern void __cdecl operator_delete(void *ptr);
extern int  __cdecl installer_open_existing_file_by_mode(const char *path, int mode);
extern u32  __cdecl installer_get_file_size_preserve_pos(int handle);
extern BOOL __cdecl installer_read_file_chunk_status(int handle, void *buf, int n);
extern void __cdecl installer_close_file(int handle);
extern int  __cdecl installer_strnicmp_locale(const byte *a, const char *b, int n);
extern void __cdecl installer_progress_update(u32 position);
extern void __cdecl installer_buffer_alloc(u32 size, void **out);
extern void __cdecl installer_buffer_free(void **ptr);
extern void __cdecl installer_remove_trailing_slash(char *path);
extern void __cdecl installer_copy_string(char *dst, const char *src);

/* Volatile (runtime) globals. */
extern s32 g_cancel_requested;
extern u32 g_output_checksum;  /* DAT_004292b0 */

/* hash = (hash * 2 - (hash >> 31)) + byte */

static u32 forward_hash(const byte *data, u32 len)
{
    u32 hash = 0;
    u32 i;
    u32 blocks = len >> 2;

    for (i = 0; i < blocks; i++) {
        hash = (hash * 2 - (u32)((s32)hash >> 0x1f)) + data[i * 4];
        hash = (hash * 2 - (u32)((s32)hash >> 0x1f)) + data[i * 4 + 1];
        hash = (hash * 2 - (u32)((s32)hash >> 0x1f)) + data[i * 4 + 2];
        hash = (hash * 2 - (u32)((s32)hash >> 0x1f)) + data[i * 4 + 3];
    }
    for (i = blocks * 4; i < len; i++) {
        hash = (hash * 2 - (u32)((s32)hash >> 0x1f)) + data[i];
    }
    return hash;
}

static u32 reverse_hash(const byte *data, u32 len)
{
    u32 hash = 0;
    u32 i;
    u32 blocks = len >> 2;
    const byte *p;

    p = data + len - 1;
    for (i = 0; i < blocks; i++) {
        hash = (hash * 2 - (u32)((s32)hash >> 0x1f)) + *p;
        hash = (hash * 2 - (u32)((s32)hash >> 0x1f)) + *(p - 1);
        hash = (hash * 2 - (u32)((s32)hash >> 0x1f)) + *(p - 2);
        hash = (hash * 2 - (u32)((s32)hash >> 0x1f)) + *(p - 3);
        p -= 4;
    }
    for (i = blocks * 4; i < len; i++) {
        hash = (hash * 2 - (u32)((s32)hash >> 0x1f)) + *p;
        p--;
    }
    return hash;
}

/*
 * FUN_00408780 @ 0x00408780
 * Main entry: read reginfo.txt file or clipboard, parse a registration key.
 */
BOOL lt2_license_read_reginfo(void)
{
    s32  handle;
    u32  file_size;
    byte *buffer;
    s32  code_type;

    if (g_reg_code[0] != '\0') return TRUE;

    installer_copy_string(g_work_buf, g_source_dir);
    {
        char *last_slash = strrchr(g_work_buf, '\\');
        char *dest = last_slash ? last_slash + 1 : g_work_buf;
        strncpy(dest, "reginfo.txt", 12);
    }

    handle = installer_open_existing_file_by_mode(g_work_buf, 0);
    if (handle != -1) {
        file_size = installer_get_file_size_preserve_pos(handle);
        if (file_size != 0 && file_size < 100000) {
            buffer = (byte *)malloc(file_size + 1);
            if (buffer != NULL) {
                installer_read_file_chunk_status(handle, buffer, file_size);
                buffer[file_size] = 0;
                code_type = *(s32 *)(g_config + 0x08);
                if (code_type == 0) {
                    lt2_license_parse_delimited((char *)buffer, file_size);
                } else {
                    lt2_license_parse_filtered(buffer, (int)file_size,
                        (u32)(code_type == LT2_RC_TYPE_E));
                }
                free(buffer);
            }
        }
        installer_close_file(handle);
    }

    if (g_reg_code[0] == '\0' &&
        IsClipboardFormatAvailable(1) &&
        OpenClipboard(g_parent_hwnd)) {
        void *clip_mem = GetClipboardData(1);
        if (clip_mem != NULL) {
            byte *clip_text = (byte *)GlobalLock(clip_mem);
            if (clip_text != NULL) {
                u32 clip_len = (u32)lstrlenA((const char *)clip_text);
                code_type = *(s32 *)(g_config + 0x08);
                if (code_type == 0) {
                    lt2_license_parse_delimited((char *)clip_text, clip_len);
                } else {
                    lt2_license_parse_filtered(clip_text, (int)clip_len,
                        (u32)(code_type == LT2_RC_TYPE_E));
                }
                GlobalUnlock(clip_mem);
            }
        }
        CloseClipboard();
    }
    return g_reg_code[0] != '\0';
}

/*
 * FUN_00408bb0 @ 0x00408bb0
 * Alternative entry: builds reginfo.txt path, reads file, falls back to clipboard.
 */
void lt2_license_read_reginfo_alt(void)
{
    char *p;
    s32   ok;

    if (g_reg_code[0] != '\0') return;

    installer_copy_string(g_work_buf, g_source_dir);
    {
        char *last_slash = strrchr(g_work_buf, '\\');
        p = last_slash ? last_slash + 1 : g_work_buf;
    }
    {
        const char *src = "reginfo.txt";
        while (*src) *p++ = *src++;
        *p = '\0';
    }

    ok = lt2_license_read_file(g_work_buf);
    if (!ok && g_reg_code[0] == '\0' &&
        IsClipboardFormatAvailable(1) &&
        OpenClipboard(g_parent_hwnd)) {
        void *clip = GetClipboardData(1);
        if (clip != NULL) {
            char *text = (char *)GlobalLock(clip);
            if (text != NULL) {
                lt2_license_parse_plain(text);
                GlobalUnlock(clip);
            }
        }
        CloseClipboard();
    }
}

/*
 * FUN_00408b00 @ 0x00408b00
 * Opens a file, reads up to LT2_REGINFO_MAX_READ bytes, parses key.
 */
BOOL lt2_license_read_file(const char *path)
{
    char  *buffer = NULL;
    BOOL   ok;
    s32    handle;
    u32    file_size;
    s32    to_read;

    g_reg_code[0] = '\0';
    installer_buffer_alloc(LT2_REGINFO_MAX_READ + 1, (void **)&buffer);
    if (buffer == NULL) return FALSE;

    handle = installer_open_existing_file_by_mode(path, 0);
    if (handle != -1) {
        file_size = installer_get_file_size_preserve_pos(handle);
        to_read = (s32)(file_size < LT2_REGINFO_MAX_READ
                        ? file_size : LT2_REGINFO_MAX_READ);
        ok = installer_read_file_chunk_status(handle, buffer, to_read);
        if (ok) {
            buffer[to_read] = '\0';
            if (*(s32 *)(g_config + 0x08) == 0) {
                lt2_license_parse_delimited(buffer, to_read);
            } else {
                lt2_license_parse_filtered((byte *)buffer, to_read,
                    (u32)(*(s32 *)(g_config + 0x08) == LT2_RC_TYPE_E));
            }
        }
        installer_close_file(handle);
    }
    installer_buffer_free((void **)&buffer);
    return g_reg_code[0] != '\0';
}

/*
 * FUN_004084c0 @ 0x004084c0
 * Reads a larger file (~131KB) for #S#...#E# license data parsing.
 */
BOOL lt2_license_read_large_file(const char *path)
{
    char  *buffer = NULL;
    BOOL   ok;
    s32    handle;
    u32    file_size;
    s32    to_read;

    installer_buffer_alloc(LT2_REGINFO_LARGE_MAX + 1, (void **)&buffer);
    if (buffer == NULL) return FALSE;

    g_reg_code[0] = '\0';
    handle = installer_open_existing_file_by_mode(path, 0);
    if (handle != -1) {
        file_size = installer_get_file_size_preserve_pos(handle);
        to_read = (s32)(file_size < LT2_REGINFO_LARGE_MAX
                        ? file_size : LT2_REGINFO_LARGE_MAX);
        ok = installer_read_file_chunk_status(handle, buffer, to_read);
        if (ok) {
            lt2_license_parse_delimited(buffer, to_read);
        }
        installer_close_file(handle);
    }
    installer_buffer_free((void **)&buffer);
    return g_reg_code[0] != '\0';
}

/*
 * FUN_00408a70 @ 0x00408a70
 * Parses plain registration text from a buffer.
 * If text contains #...# delimiters: extract content between them.
 * Else if length 19-25: treat as serial number.
 * Filters to alphanumeric + # / @. Stores in g_reg_code.
 */
void lt2_license_parse_plain(char *text)
{
    char *start, *end;
    char *out;
    s32   out_idx, i;
    char  c;

    start = text;
    end   = NULL;

    {
        char *hash1 = strchr(text, '#');
        if (hash1 != NULL) {
            char *hash2 = strrchr(text, '#');
            if (hash2 != NULL) {
                start = hash1;
                end   = hash2;
                goto have_span;
            }
        }
    }

    {
        u32 n = 0xffffffff;
        const char *p = text;
        do { if (n == 0) break; n--; c = *p++; } while (c != '\0');
        u32 text_len = ~n - 1;
        if (0x13 < text_len && text_len < 0x19) {
            end = text + text_len;
        }
    }

have_span:
    if (end == NULL || end <= start) return;

    out     = (char *)g_reg_code;
    out_idx = 0;

    for (i = 0; i < (s32)(end - start) && out_idx < 0x293; i++) {
        c = start[i];
        if (c == '#' || (c > '/' && c < ':') || (c > '@' && c < '[')) {
            out[out_idx++] = c;
        }
    }
    out[out_idx] = '\0';
}

/*
 * FUN_00408570 @ 0x00408570
 * Parses #S#...#E# delimited registration data with checksum validation.
 *
 * Extracts alphanumeric content between #S# and #E markers.
 * Checksum: sum bytes[3..len-7]; trailing two chars are:
 *   'A' + (sum & 0xFF) % 26  and  'A' + ((sum >> 8) & 0xFF) % 26
 * Passes to hash table validation.
 */
void lt2_license_parse_delimited(char *text, u32 text_len)
{
    char  *delim_start = NULL;
    char  *cursor;
    u32    idx;
    BOOL   first_found = TRUE;

    g_reg_code[0] = '\0';
    if (text == NULL || text_len <= 5) return;

    for (idx = 0; idx < text_len - 2; idx++) {
        cursor = text + idx;

        if (delim_start == NULL) {
            if (cursor[0] == '#' && cursor[1] == 'S' && cursor[2] == '#') {
                delim_start = cursor;
            }
        } else if (cursor >= delim_start + 3 &&
                   cursor[0] == '#' && cursor[1] == 'E') {

            if (g_license_attempts < 1) return;
            if (first_found) g_license_attempts--;

            u32 work_len = (u32)(cursor + (cursor[2] != '#' ? 2 : 3) - delim_start);
            char *work_buf = (char *)malloc(work_len + 1);
            if (work_buf == NULL) return;

            /* Filter to alphanumeric + #. */
            char *out = work_buf;
            char *p = delim_start;
            s32 remaining = (s32)work_len;
            for (; p < delim_start + work_len; p++) {
                char ch = *p;
                if (ch == '#' || (ch > '/' && ch < '7') ||
                    (ch > '@' && ch < '[')) {
                    *out++ = ch;
                }
            }
            if (cursor[2] != '#') *out++ = '#';
            *out = '\0';
            work_len = (u32)(out - work_buf);

            if (work_len < LT2_REGCODE_MAX_LEN) {
                lstrcpyA((char *)g_reg_code, work_buf);
            }

            u32 code_len = (u32)lstrlenA((const char *)g_reg_code);
            if (code_len > 8) {
                u32 checksum = 0;
                s32 i;
                for (i = 3; i < (s32)(code_len - 6); i++) {
                    checksum += g_reg_code[i];
                }

                s32 code_type = *(s32 *)(g_config + 0x08);
                s32 xkey = (code_type == 0) ? 8 : 5;
                s32 hash_ok = lt2_license_hash_lookup(g_reg_code + 3,
                    code_len - 7, xkey);

                if (hash_ok != 0) {
                    char cs1 = (char)((checksum & 0xFF) % 26) + 'A';
                    char cs2 = (char)(((checksum >> 8) & 0xFF) % 26) + 'A';
                    if (g_reg_code[code_len - 6] != cs1 ||
                        g_reg_code[code_len - 5] != cs2) {
                        g_reg_code[0] = '\0';
                    }
                } else {
                    g_reg_code[0] = '\0';
                }
            }

            free(work_buf);
            if (g_reg_code[0] != '\0') return;
            first_found = FALSE;
        }
    }
}

/*
 * FUN_00409340 @ 0x00409340
 * Filters input to alphanumeric, validates length, calls hash lookup.
 * Excludes 'I' and 'O' (confusable with 1 and 0) when not numeric_only.
 */
void lt2_license_parse_filtered(byte *text, int text_len, int numeric_only)
{
    s32 out_idx = 0;
    s32 i;
    byte c;

    g_reg_code[0] = '\0';

    for (i = 0; i < text_len && out_idx < (LT2_REGCODE_MAX_LEN - 1); i++) {
        c = text[i];

        if (c == 0x20 || c == 0x0A || c == 0x0D ||
            c == 0x2D || c == 0x2F || c == 0x5C ||
            c == 0x2A || c == 0x23 || c == 0x2B || c == 0x5F) {
            continue;
        }

        if (numeric_only) {
            if (c > '/' && c < ':') {
                g_reg_code[out_idx++] = c;
            } else {
                out_idx = 0;
                g_reg_code[0] = '\0';
            }
        } else {
            if ((c > '/' && c < '9') ||
                (c > '@' && c < '[' && c != 'I' && c != 'O')) {
                g_reg_code[out_idx++] = c;
            } else {
                out_idx = 0;
                g_reg_code[0] = '\0';
            }
        }
    }
    g_reg_code[out_idx] = '\0';

    u32 code_len = (u32)lstrlenA((const char *)g_reg_code);
    if (code_len < 0x12) {
        g_reg_code[0] = '\0';
        return;
    }

    s32 code_type = *(s32 *)(g_config + 0x08);
    if (g_license_attempts > 0) g_license_attempts--;

    if (code_type == 3) {
        u32 seed = *(u16 *)(g_config + 0x02);
        u32 chk  = seed;
        s32 j;
        for (j = 0; j < (s32)(code_len - 2); j++) {
            chk = g_reg_code[j] + (chk >> 0xF & 1) + chk * 2;
        }
    }

    s32 hash_ok = lt2_license_hash_lookup(g_reg_code, code_len - 1, 5);
    if (hash_ok == 0) {
        g_reg_code[0] = '\0';
        return;
    }

    if (code_type == 3) {
        byte chk = (byte)(*(u16 *)(g_config + 0x02) & 0x1F);
        if (chk < 6) chk += 0x31;
        else { chk += 0x3B; if (chk == 'I') chk = '7'; else if (chk == 'O') chk = '8'; }
        if (g_reg_code[code_len - 2] != chk) {
            g_reg_code[0] = '\0';
        }
    }
}

/*
 * FUN_00407850 @ 0x00407850
 * Validates g_reg_code against the hardcoded hash table when the installer
 * configuration enables the hash gate.
 *
 * If code_type != 0: skip first 3 chars, xor_key_len = 5
 * Else:              use full code,    xor_key_len = 8
 *
 * Checks: code_length, forward_hash, reverse_hash match table entry,
 * then XOR-validates start and end of code against table keys.
 * Returns non-zero only when the hash gate is active and the code/table state
 * passes the gate.
 */
BOOL lt2_license_validate_hash(void)
{
    u32           code_len;
    u32           fwd_hash, rev_hash;
    byte         *code;
    u32           prefix_skip;
    s32           xor_key_len;
    Lt2HashTable *table;
    Lt2HashEntry *entry;
    s32           entry_count, j, i, code_type;

    if ((g_config[1] & 4) == 0) return FALSE;

    table = (Lt2HashTable *)g_hash_tables[g_hash_table_index];
    if (table == NULL) return TRUE;

    code_len    = (u32)lstrlenA((const char *)g_reg_code);
    code_type   = *(s32 *)(g_config + 0x08);
    code        = g_reg_code;
    xor_key_len = 5;

    if (code_type == 0) {
        code       += 3;
        code_len   -= 6;
        xor_key_len = 8;
    }

    if (code_len == 0) return TRUE;

    fwd_hash = forward_hash(code, code_len);
    rev_hash = reverse_hash(code, code_len);

    entry       = (Lt2HashEntry *)((byte *)table + LT2_HASH_TABLE_HEADER_SIZE);
    entry_count = (s32)table->entry_count;

    for (j = 0; j < entry_count; j++) {
        if (entry->code_length == code_len &&
            entry->hash_forward == fwd_hash &&
            entry->hash_reverse == rev_hash) {
            s32 matched = 0;
            for (i = 0; i < xor_key_len; i++) {
                byte exp_s = entry->xor_key_start[i] ^ (byte)(i + 0x55);
                byte exp_e = entry->xor_key_end[i] ^ (byte)(i + 0x77);
                if (code[i] != exp_s ||
                    code[code_len - xor_key_len + i] != exp_e)
                    break;
                matched++;
            }
            if (matched == xor_key_len) return TRUE;
        }
        entry++;
    }
    return FALSE;
}

/*
 * FUN_00405790 @ 0x00405790
 * Core hash table lookup with XOR key validation.
 * Sets g_license_valid = 1 on success.
 * The table tier byte is biased by 0x0E before it is compared with the active
 * hash-table index.
 */
BOOL lt2_license_hash_lookup(byte *code, u32 code_len, int xor_key_len)
{
    Lt2HashTable *table;
    Lt2HashEntry *entry;
    u32           fwd_hash, rev_hash;
    u32           entry_count, j;
    s32           i;
    byte          tier;

    g_license_valid = 0;

    table = (Lt2HashTable *)g_hash_tables[g_hash_table_index];
    if (table == NULL) return TRUE;

    fwd_hash = forward_hash(code, code_len);
    rev_hash = reverse_hash(code, code_len);

    entry       = (Lt2HashEntry *)((byte *)table + LT2_HASH_TABLE_HEADER_SIZE);
    entry_count = table->entry_count;

    for (j = 0; j < entry_count; j++) {
        if (entry->code_length == code_len &&
            entry->hash_forward == fwd_hash &&
            entry->hash_reverse == rev_hash) {
            s32 matched = 0;
            for (i = 0; i < xor_key_len; i++) {
                byte exp_s = entry->xor_key_start[i] ^ (byte)(i + 0x55);
                byte exp_e = entry->xor_key_end[i] ^ (byte)(i + 0x77);
                if (code[i] != exp_s ||
                    code[code_len - xor_key_len + i] != exp_e)
                    break;
                matched++;
            }
            if (matched == xor_key_len) {
                g_license_valid = 1;
                tier = (byte)(table->tier - 0x0E);
                if (tier == 0) return FALSE;
                if (tier - 1 == g_hash_table_index) return FALSE;
                if (g_max_hash_table_index < (s32)tier) return FALSE;
            }
        }
        entry++;
    }
    return TRUE;
}

/* RC4-like key scheduling used by the license decryptor. */

static void rc4_ksa(u16 *sbox, u32 sbox_size,
                    u16 seed_a, byte seed_b, BOOL legacy_mode)
{
    u32 i;
    u32 state = seed_b;

    if (sbox_size == 0) return;
    sbox[0] = 0;

    if (!legacy_mode) {
        for (i = 1; i < sbox_size; i++) {
            u32 pos = (i + seed_a ^ state) % i;
            state   = state + (pos ^ i);
            sbox[i] = sbox[pos];
            sbox[pos] = (u16)i;
        }
    } else {
        for (i = 1; i < sbox_size; i++) {
            u32 pos = (i + (seed_a & 0xFF) ^ (state & 0xFF)) % i;
            state   = (byte)((byte)state + ((byte)pos ^ (byte)i));
            sbox[i] = sbox[pos];
            sbox[pos] = (u16)i;
        }
    }
}

/*
 * FUN_004059c0 @ 0x004059c0
 * Registration key decryptor: charset/length checks, 5-bit decode,
 * RC4-like permutation, XOR feedback, and g_config[0x1D] checksum.
 */
u32 lt2_license_decrypt(byte *output, u32 output_size,
                        const byte *input, int mode)
{
    u32   code_type;
    u32   input_len;
    BOOL  legacy_mode;
    byte  rc4_byte;
    u16   seed_a;
    u16  *sbox     = NULL;
    byte *scratch  = NULL;
    u32   decoded_len;
    u32   fwd_hash, rev_hash;
    u32   i, j;
    u32   state;
    s32   bit_buf;
    s32   bit_count;
    u32   out_idx;
    s32   checksum;
    byte  final_checksum;
    Lt2HashTable *table;
    Lt2HashEntry *entry;
    u32   entry_count;
    s32   xorkey_len;
    s32   tier;
    u32   table_pos;
    s32   code_type_cfg;

    memset(output, 0, output_size);

    input_len = (u32)lstrlenA((const char *)input);
    if (input_len < 8) return 0;

    code_type = *(u32 *)(g_config + 0x04);

    /* Direct key validation path (code_type 3 or 4). */
    if (code_type == 3 || code_type == 4) {
        if (input_len < LT2_KEY_MIN_LEN || input_len > LT2_KEY_MAX_LEN) return 0;

        if (mode == 0) {
            legacy_mode = (code_type == 4);
            rc4_byte    = g_config[0x2A];
            seed_a      = *(u16 *)(g_config + 0x1A);
        } else {
            legacy_mode = (code_type == 4);
            rc4_byte    = g_config[0x53];
            seed_a      = *(u16 *)(g_config + 0x0F);
        }

        for (i = 0; i < input_len; i++) {
            byte c = input[i];
            if (c < '0' || c > 'Z') goto fail_zero;
            if (legacy_mode) {
                if (c > '9') goto fail_zero;
            } else {
                if (c > '8' && c < 'A') goto fail_zero;
            }
        }

        if (!legacy_mode) {
            decoded_len = 0;
            bit_buf     = 0;
            bit_count   = 1;
            for (i = 0; i < input_len; i++) {
                j = 5;
                while (j-- > 0) {
                    bit_count *= 2;
                    if (bit_count > 0xFF) {
                        decoded_len++;
                        bit_count = 1;
                    }
                    if (decoded_len >= output_size) break;
                }
                if (decoded_len >= output_size) break;
            }
            if (bit_count != 1 && decoded_len < output_size) decoded_len++;
            if (decoded_len < 12) decoded_len = 12;
        } else {
            decoded_len = 12;
        }
        if (decoded_len > LT2_REGCODE_MAX_LEN) decoded_len = LT2_REGCODE_MAX_LEN;

        fwd_hash = forward_hash(input, input_len);
        rev_hash = reverse_hash(input, input_len);

        sbox    = (u16 *)malloc(0x53C * sizeof(u16));
        scratch = (byte *)malloc(LT2_REGCODE_MAX_LEN);
        if (sbox == NULL || scratch == NULL) { free(sbox); return 0; }

        rc4_ksa(sbox, decoded_len, seed_a, rc4_byte, legacy_mode);

        table       = (Lt2HashTable *)g_hash_tables[g_hash_table_index];
        entry       = NULL;
        entry_count = 0;
        xorkey_len  = 5;
        if (table != NULL) {
            tier = table->tier;
            if (tier == 0 || tier - 1 == g_hash_table_index ||
                g_max_hash_table_index < tier) {
                entry       = (Lt2HashEntry *)((byte *)table + LT2_HASH_TABLE_HEADER_SIZE);
                entry_count = table->entry_count;
            }
            code_type_cfg = *(s32 *)(g_config + 0x08);
            if (code_type_cfg == 0) xorkey_len = 8;
        }

        out_idx    = 0;
        bit_buf    = 1;
        table_pos  = 0;

        for (i = 0; i < decoded_len; i++) {
            byte raw_byte;
            if (i >= input_len) break;
            raw_byte = input[i];
            if (raw_byte == 0) break;

            if (legacy_mode) {
                state = raw_byte;
                bit_count = 1;
            } else if (raw_byte == '7') {
                state = 0x0E; bit_count = 5;
            } else if (raw_byte == '8') {
                state = 0x14; bit_count = 5;
            } else if (raw_byte < 'A') {
                state = raw_byte - 0x31; bit_count = 5;
            } else {
                state = raw_byte - 0x3B; bit_count = 5;
            }

            for (j = 0; j < (u32)bit_count; j++) {
                if (!legacy_mode) {
                    bit_buf = bit_buf * 2;
                    if (state & 1) bit_buf |= 1;
                    state >>= 1;
                }
                if (bit_buf > 0xFF) {
                    byte val = (byte)bit_buf;
                    scratch[sbox[out_idx & 0xFFFF]] = val;

                    if (entry != NULL && table_pos < entry_count) {
                        if (entry[table_pos].code_length == input_len &&
                            entry[table_pos].hash_forward == fwd_hash &&
                            entry[table_pos].hash_reverse == rev_hash) {
                            s32 xk;
                            for (xk = 0; xk < xorkey_len; xk++) {
                                byte es = entry[table_pos].xor_key_start[xk]
                                        ^ (byte)(xk + 0x55);
                                byte ee = entry[table_pos].xor_key_end[xk]
                                        ^ (byte)(xk + 0x77);
                                if (input[xk] != es ||
                                    input[input_len - xorkey_len + xk] != ee)
                                    break;
                            }
                            if (xk == xorkey_len) out_idx += 0x10000;
                        }
                        table_pos++;
                    }
                    bit_buf = 1;
                    out_idx++;
                }
            }
        }

        if (bit_buf != 1 && (out_idx & 0xFFFF) < decoded_len) {
            while (bit_buf < 0x100) bit_buf *= 2;
            scratch[sbox[out_idx & 0xFFFF]] = (byte)bit_buf;
        }

        checksum = 0;
        for (i = 0; i < decoded_len; i++) {
            byte b;
            if (legacy_mode) {
                b = (byte)((s32)((scratch[i] - rc4_byte % 10) - 38) % 10) + 0x30;
            } else {
                b = scratch[i] ^ rc4_byte;
            }
            scratch[i] = b;
            rc4_byte += b;

            for (j = 0; j < i; j++) {
                u32 k;
                for (k = 1; k < i; k++) {
                    checksum += (u32)((byte)(scratch[k] ^ scratch[j]) >> 1);
                }
            }
            scratch[i] ^= 0x6A;
        }

        for (i = 0; i < decoded_len; i++) {
            output[i] = scratch[sbox[i]] ^ (byte)i;
            checksum += output[i];
        }

        if (g_output_checksum != (u32)checksum) {
            g_output_checksum = (u32)checksum;
        }

        free(scratch);
        free(sbox);

        /* Final checksum validation. */
        if (code_type == 3) {
            byte chk = 0;
            for (i = 0; i < (s32)(decoded_len - 1); i++) {
                chk = chk * 2 + ((output[i] ^ (byte)i ^ 0x6A) - (s32)(chk >> 7));
            }
            if ((output[decoded_len - 1] ^ (byte)i ^ 0x6A) != chk) return 0;
        }

        {
            byte chain = 0;
            for (i = 0; i < 5; i++) {
                chain = chain * 2 + ((output[i] ^ (byte)i ^ 0x6A) - (s32)(chain >> 7));
            }
            if (chain != g_config[0x1D]) return 0;
        }

        return decoded_len;
    }

    /* Non-keyed mode path (code_type 1 or 2): strip prefix, decode rest. */
    {
        byte *stripped;
        if (input_len < 8) return 0;
        stripped = (byte *)malloc(input_len - 8);
        if (stripped == NULL) return 0;
        memcpy(stripped, input + 3, input_len - 9);
        stripped[input_len - 9] = '\0';
        free(stripped);
    }

fail_zero:
    return 0;
}

/*
 * FUN_0040bca0 @ 0x0040bca0
 * Resolves installer template variables like #UserRegCode#,
 * #UserName#, #UserSerialNumber#, #InstallDir#, #Windows#, etc.
 */
void lt2_license_resolve_template(byte *template_var)
{
    if (lstrcmpiA((const char *)template_var, "#Windows#") == 0) {
        GetWindowsDirectoryA((char *)template_var, 260);
        installer_remove_trailing_slash((char *)template_var);
        return;
    }
    if (lstrcmpiA((const char *)template_var, "#System#") == 0) {
        GetSystemDirectoryA((char *)template_var, 260);
        installer_remove_trailing_slash((char *)template_var);
        return;
    }
    if (lstrcmpiA((const char *)template_var, "#TempDir#") == 0) {
        GetTempPathA(260, (char *)template_var);
        installer_remove_trailing_slash((char *)template_var);
        return;
    }
    if (lstrcmpiA((const char *)template_var, "#UserRegCode#") == 0) {
        installer_copy_string((char *)template_var, (const char *)g_reg_code);
        return;
    }
    if (lstrcmpiA((const char *)template_var, "#UserName#") == 0) {
        lt2_license_decrypt_config_field(template_var, NULL);
        return;
    }
    if (lstrcmpiA((const char *)template_var, "#UserCompany#") == 0) {
        lt2_license_decrypt_config_field(template_var, NULL);
        return;
    }
    if (lstrcmpiA((const char *)template_var, "#UserSerialNumber#") == 0) {
        lt2_license_decrypt_config_field(template_var, NULL);
        return;
    }
    if (lstrcmpiA((const char *)template_var, "#SourceDir#") == 0) {
        installer_copy_string((char *)template_var, g_source_dir);
        return;
    }
    template_var[0] = '\0';
}

/*
 * FUN_0040c3b0 @ 0x0040c3b0
 * Decrypts an installer config field via XOR cipher.
 * If g_config[0x08] != 0: plain copy (length-delimited by first byte).
 * Else: XOR decrypt with position-dependent key.
 */
void lt2_license_decrypt_config_field(byte *output, const byte *source)
{
    u32 len, i;
    output[0] = '\0';
    if (source == NULL) return;

    if (*(s32 *)(g_config + 0x08) != 0) {
        len = (u32)source[0];
        memcpy(output, source + 1, len);
        output[len] = '\0';
    } else {
        s32 offset = (s32)(source - g_reg_code);
        byte first_byte = source[0];
        len = (u32)((byte)(offset ^ first_byte ^ 0x6A));
        for (i = 0; i < len; i++) {
            output[i] = source[i + 1] ^ (byte)(offset + i + 1) ^ 0x6A;
        }
        output[len] = '\0';
    }
}

int lt2_wininet_open_session(void)
{
    if (g_wininet_module == NULL) {
        g_wininet_module = LoadLibraryA("wininet.dll");
        g_pInternetOpenA = (LPInternetOpenA)
            GetProcAddress(g_wininet_module, "InternetOpenA");
        g_pInternetCloseHandle = (LPInternetCloseHandle)
            GetProcAddress(g_wininet_module, "InternetCloseHandle");
        g_pInternetOpenUrlA = (LPInternetOpenUrlA)
            GetProcAddress(g_wininet_module, "InternetOpenUrlA");
        g_pInternetQueryDataAvailable = (LPInternetQueryDataAvailable)
            GetProcAddress(g_wininet_module, "InternetQueryDataAvailable");
        g_pInternetReadFile = (LPInternetReadFile)
            GetProcAddress(g_wininet_module, "InternetReadFile");

        if (!g_pInternetOpenA || !g_pInternetCloseHandle ||
            !g_pInternetOpenUrlA || !g_pInternetQueryDataAvailable ||
            !g_pInternetReadFile) {
            lt2_wininet_shutdown();
        }
        if (g_wininet_module == NULL) return 0;
    }

    if (g_hInternet != NULL) return (int)(intptr_t)g_hInternet;
    g_hInternet = g_pInternetOpenA("InstallProgram", 0, NULL, NULL, 0);
    return (int)(intptr_t)g_hInternet;
}

void lt2_wininet_shutdown(void)
{
    if (g_hInternet != NULL) {
        g_pInternetCloseHandle(g_hInternet);
        g_hInternet = NULL;
    }
    if (g_wininet_module != NULL) {
        FreeLibrary(g_wininet_module);
        g_wininet_module = NULL;
    }
    g_pInternetOpenA              = NULL;
    g_pInternetCloseHandle        = NULL;
    g_pInternetOpenUrlA           = NULL;
    g_pInternetQueryDataAvailable = NULL;
    g_pInternetReadFile           = NULL;
}

int lt2_copy_or_download_file(char *local_path, char *source_url,
                              u32 source_size, u32 progress_start,
                              u32 progress_end)
{
    s32 result = 0;

    if (installer_strnicmp_locale((byte *)source_url, "file://", 7) == 0) {
        char converted[260];
        char *p = source_url + 7;
        char *d = converted;
        while (*p) *d++ = *p++;
        *d = '\0';
        for (p = converted; *p; p++) {
            if (*p == '/') *p = '\\';
        }
        return CopyFileA(converted, local_path, FALSE) != 0;
    }

    if (!lt2_wininet_open_session()) return 0;

    {
        void *chunk_buf = malloc(0x1000);
        if (chunk_buf == NULL) return 0;

        HANDLE hUrl = g_pInternetOpenUrlA(
            g_hInternet, source_url, NULL, 0, 0, 0);

        if (hUrl != NULL) {
            HANDLE hFile = CreateFileA(
                local_path, 0x40000000, 2, NULL, 2, 0x80, NULL);

            if (hFile != (HANDLE)-1) {
                u32 total_read = 0;
                BOOL error = FALSE;
                u32 prev_progress = 0;

                while (!error && !g_cancel_requested) {
                    DWORD available = 0;
                    BOOL ok = g_pInternetQueryDataAvailable(
                        hUrl, &available, 0, 0);
                    if (!ok || available == 0) break;

                    DWORD to_read = (available < 0x1000)
                                    ? available : 0x1000;
                    DWORD bytes_read = 0;
                    ok = g_pInternetReadFile(
                        hUrl, chunk_buf, to_read, &bytes_read);

                    if (ok) {
                        DWORD written = 0;
                        ok = WriteFile(hFile, chunk_buf,
                                       bytes_read, &written, NULL);
                        if (ok) {
                            total_read += bytes_read;
                            if (progress_start != progress_end &&
                                source_size > 0) {
                                u32 p = (progress_end - progress_start) *
                                        total_read / source_size +
                                        progress_start;
                                if (p != prev_progress) {
                                    installer_progress_update(p);
                                    prev_progress = p;
                                }
                            }
                        } else {
                            error = TRUE;
                        }
                    } else {
                        error = TRUE;
                    }
                }

                if (!error) {
                    installer_progress_update(progress_end);
                    result = 1;
                }
                CloseHandle(hFile);
            }
            g_pInternetCloseHandle(hUrl);
        }
        free(chunk_buf);
    }
    return result;
}

u32 lt2_license_dialog_process(HANDLE hwnd_dialog)
{
    s32 len1 = (s32)SendDlgItemMessageA(hwnd_dialog, 0x406, 0x0E, 0, 0);
    s32 len2 = (s32)SendDlgItemMessageA(hwnd_dialog, 0x407, 0x0E, 0, 0);
    s32 len3 = (s32)SendDlgItemMessageA(hwnd_dialog, 0x408, 0x0E, 0, 0);
    u32 next_focus = 0;

    /* Auto-tab when the focused field reaches its expected length. */
    if (len1 == 6) {
        if (len2 == 0) {
            HANDLE ctrl1 = GetDlgItem(hwnd_dialog, 0x406);
            HANDLE focus = GetFocus();
            if (focus == ctrl1) next_focus = 0x407;
        } else if (len2 == 8 && len3 == 0) {
            HANDLE ctrl2 = GetDlgItem(hwnd_dialog, 0x407);
            HANDLE focus = GetFocus();
            if (focus == ctrl2) next_focus = 0x408;
        }
    }

    if (len1 > 0 && len2 > 0 && len3 > 0) {
        s32 total = len1 + len2 + len3;
        byte *buf = (byte *)malloc(total + 1);
        if (buf != NULL) {
            GetDlgItemTextA(hwnd_dialog, 0x406, (char *)buf, total + 1);
            GetDlgItemTextA(hwnd_dialog, 0x407,
                           (char *)(buf + len1), (total - len1) + 1);
            GetDlgItemTextA(hwnd_dialog, 0x408,
                           (char *)(buf + len1 + len2),
                           (total - len1 - len2) + 1);
            buf[total] = '\0';

            lt2_license_parse_filtered(buf, total,
                (u32)(*(s32 *)(g_config + 0x08) == LT2_RC_TYPE_E));
            free(buf);
        }
    }
    return next_focus;
}
