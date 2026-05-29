/*
 * Lemonade Tycoon 2 focused runtime recovery support types.
 *
 * This header is for readable decompiler output, not a clean-room SDK.
 * Addresses in comments are scoped to the individual extracted file named in
 * the adjacent source comment.
 *
 * Source binaries:
 * - decompiled/local/installers/Lemonade Tycoon 2 - New York City.exe
 * - decompiled/local/unpacked/Lemonade2.unpacked.exe
 * - decompiled/local/lt2_install/Lemonade2.exe (packed/protector evidence)
 * - ./lsx-server/decompiled/local/lt2_install/TeneonIERelease.dll
 * - ./lsx-server/decompiled/local/lt2_install/fmod.dll
 */
#ifndef LT2_WIN32_RECOVERED_TYPES_H
#define LT2_WIN32_RECOVERED_TYPES_H

#include <stddef.h>
#include <stdint.h>

#ifndef __stdcall
#define __stdcall
#endif

#ifndef __thiscall
#define __thiscall
#endif

#ifndef __cdecl
#define __cdecl
#endif

typedef uint8_t byte;
typedef uint8_t u8;
typedef uint16_t u16;
typedef uint32_t u32;
typedef uint64_t u64;
typedef int16_t s16;
typedef int32_t s32;
typedef int BOOL;
typedef void *HANDLE;
typedef HANDLE HWND;
typedef HANDLE HKEY;
typedef void *HMODULE;
typedef uintptr_t UINT_PTR;
typedef uint16_t WORD;
typedef WORD ATOM;
typedef s16 SHORT;
typedef unsigned long DWORD;
typedef long LONG;
typedef unsigned int UINT;
typedef unsigned char BYTE;
typedef const char *LPCSTR;
typedef char *LPSTR;
typedef void *LPVOID;
typedef UINT WPARAM;
typedef LONG LPARAM;
typedef LONG LRESULT;

#ifndef TRUE
#define TRUE 1
#define FALSE 0
#endif /* LT2_WIN32_RECOVERED_TYPES_H */

#ifndef HKEY_LOCAL_MACHINE
#define HKEY_LOCAL_MACHINE ((HKEY)(uintptr_t)0x80000002U)
#endif

#ifndef KEY_QUERY_VALUE
#define KEY_QUERY_VALUE 0x0001
#endif

/*
 * Windows API imports used by the recovered pseudocode. These are declarations
 * only so the decompiled files can preserve call intent without requiring the
 * Windows SDK in this repository.
 */
int __stdcall IsWindowVisible(HANDLE hwnd);
BOOL __stdcall GetComputerNameA(char *buf, DWORD *size);
BOOL __stdcall GetVolumeInformationA(const char *root, char *volume_name,
                                     DWORD volume_name_size, DWORD *serial, DWORD *max_component_len,
                                     DWORD *flags, char *filesystem_name, DWORD filesystem_name_size);
LONG __stdcall RegOpenKeyExA(HKEY key, const char *subkey, DWORD options,
                             DWORD access, HKEY *result);
LONG __stdcall RegQueryValueExA(HKEY key, const char *name, DWORD *reserved,
                                DWORD *type, BYTE *data, DWORD *data_len);
LONG __stdcall RegCloseKey(HKEY key);

/* Source: decompiled/local/lt2_install/TeneonIERelease.dll. */
typedef enum Lt2BrowserResult
{
    LT2_BROWSER_OK = 0,
    LT2_BROWSER_NOT_READY = 1,
    LT2_BROWSER_NAVIGATE_BLOCKED = 2,
    LT2_BROWSER_INVALID_ARGUMENT = 3,
    LT2_BROWSER_FAILED = 4
} Lt2BrowserResult;

typedef struct IEBrowserContainer IEBrowserContainer;
typedef struct Lt2StdString
{
    void *allocator_or_rep;
    char *data;
    u32 size;
    u32 capacity;
} Lt2StdString;

Lt2BrowserResult IEBrowserContainer_LoadURL(IEBrowserContainer *self,
                                            Lt2StdString *url); /* TeneonIERelease.dll @ 0x10001210 */
Lt2BrowserResult IEBrowserContainer_Refresh(IEBrowserContainer *self);
/* TeneonIERelease.dll @ 0x10001310 */

#endif
