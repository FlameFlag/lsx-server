#ifndef WINDOWS_INTELLISENSE_H
#define WINDOWS_INTELLISENSE_H

/*
 * Minimal Windows declarations for macOS/Linux IntelliSense only.
 * Real builds include the platform SDK's <windows.h> instead.
 */

#include <stddef.h>
#include <stdint.h>
#include <string.h>

#define WINAPI
#define FALSE 0
#define TRUE 1
#ifndef NULL
#define NULL ((void *)0)
#endif
#define INVALID_HANDLE_VALUE ((HANDLE)(intptr_t)-1)
#define MAX_PATH 260
#define CP_ACP 0

typedef int BOOL;
typedef int LONG;
typedef unsigned long DWORD;
typedef unsigned long ULONG;
typedef uint16_t WORD;
typedef uint8_t BYTE;
typedef size_t SIZE_T;
typedef void *HANDLE;
typedef void *HMODULE;
typedef void *HINSTANCE;
typedef void *LPVOID;
typedef const void *LPCVOID;
typedef const char *LPCSTR;
typedef char *LPSTR;
typedef const wchar_t *LPCWSTR;
typedef wchar_t *LPWSTR;
typedef void *FARPROC;
typedef DWORD *PDWORD;
typedef ULONG *PULONG;
typedef void *PVOID;
typedef void *LPSECURITY_ATTRIBUTES;
typedef DWORD (*LPTHREAD_START_ROUTINE)(LPVOID);
typedef struct CRITICAL_SECTION { void *opaque; } CRITICAL_SECTION;

typedef struct STARTUPINFOA { DWORD cb; } STARTUPINFOA, *LPSTARTUPINFOA;
typedef struct STARTUPINFOW { DWORD cb; } STARTUPINFOW, *LPSTARTUPINFOW;

typedef struct PROCESS_INFORMATION {
    HANDLE hProcess;
    HANDLE hThread;
    DWORD dwProcessId;
    DWORD dwThreadId;
} PROCESS_INFORMATION, *LPPROCESS_INFORMATION;

typedef struct CONTEXT {
    DWORD ContextFlags;
    DWORD Eax, Ebx, Ecx, Edx, Esi, Edi;
    DWORD Eip, Esp, Ebp, EFlags;
    DWORD Dr0, Dr1, Dr2, Dr3, Dr6, Dr7;
} CONTEXT, *LPCONTEXT;

typedef struct EXCEPTION_RECORD_STUB {
    DWORD ExceptionCode;
    void *ExceptionAddress;
    DWORD NumberParameters;
    uintptr_t ExceptionInformation[15];
} EXCEPTION_RECORD_STUB;

typedef struct EXCEPTION_DEBUG_INFO_STUB {
    EXCEPTION_RECORD_STUB ExceptionRecord;
    DWORD dwFirstChance;
} EXCEPTION_DEBUG_INFO_STUB;

typedef struct CREATE_PROCESS_DEBUG_INFO_STUB {
    HANDLE hProcess;
    HANDLE hThread;
    void *lpBaseOfImage;
    void *lpStartAddress;
} CREATE_PROCESS_DEBUG_INFO_STUB;

typedef struct DEBUG_EVENT {
    DWORD dwDebugEventCode;
    DWORD dwProcessId;
    DWORD dwThreadId;
    union {
        EXCEPTION_DEBUG_INFO_STUB Exception;
        CREATE_PROCESS_DEBUG_INFO_STUB CreateProcessInfo;
    } u;
} DEBUG_EVENT, *LPDEBUG_EVENT;

typedef struct IMAGE_DOS_HEADER { LONG e_lfanew; } IMAGE_DOS_HEADER;
typedef struct IMAGE_DATA_DIRECTORY { DWORD VirtualAddress; DWORD Size; } IMAGE_DATA_DIRECTORY;
typedef struct IMAGE_OPTIONAL_HEADER32 { IMAGE_DATA_DIRECTORY DataDirectory[16]; } IMAGE_OPTIONAL_HEADER32;
typedef struct IMAGE_FILE_HEADER { WORD SizeOfOptionalHeader; } IMAGE_FILE_HEADER;
typedef struct IMAGE_NT_HEADERS32 { DWORD Signature; IMAGE_FILE_HEADER FileHeader; IMAGE_OPTIONAL_HEADER32 OptionalHeader; } IMAGE_NT_HEADERS32;
typedef struct IMAGE_IMPORT_DESCRIPTOR { DWORD OriginalFirstThunk; DWORD TimeDateStamp; DWORD ForwarderChain; DWORD Name; DWORD FirstThunk; } IMAGE_IMPORT_DESCRIPTOR;
typedef struct IMAGE_THUNK_DATA32 { union { DWORD ForwarderString; DWORD Function; DWORD Ordinal; DWORD AddressOfData; } u1; } IMAGE_THUNK_DATA32;
typedef struct IMAGE_IMPORT_BY_NAME { WORD Hint; char Name[1]; } IMAGE_IMPORT_BY_NAME;

#define IMAGE_DIRECTORY_ENTRY_IMPORT 1
#define IMAGE_ORDINAL_FLAG32 0x80000000u

#define GENERIC_WRITE 0x40000000u
#define FILE_SHARE_READ 0x00000001u
#define CREATE_ALWAYS 2u
#define FILE_ATTRIBUTE_NORMAL 0x00000080u
#define PAGE_READWRITE 0x04u
#define PAGE_GUARD 0x100u
#define PAGE_EXECUTE_READWRITE 0x40u
#define MEM_COMMIT 0x1000u
#define MEM_RESERVE 0x2000u
#define MEM_RELEASE 0x8000u
#define CREATE_SUSPENDED 0x00000004u
#define WAIT_OBJECT_0 0u
#define CONTEXT_FULL 0x00010007u
#define CONTEXT_DEBUG_REGISTERS 0x00010010u
#define EXCEPTION_DEBUG_EVENT 1u
#define EXCEPTION_BREAKPOINT 0x80000003u
#define EXCEPTION_SINGLE_STEP 0x80000004u
#define STATUS_GUARD_PAGE_VIOLATION 0x80000001u
#define CREATE_PROCESS_DEBUG_EVENT 3u
#define EXIT_PROCESS_DEBUG_EVENT 5u
#define DBG_CONTINUE 0x00010002u
#define DLL_PROCESS_ATTACH 1u

void InitializeCriticalSection(CRITICAL_SECTION *cs);
void EnterCriticalSection(CRITICAL_SECTION *cs);
void LeaveCriticalSection(CRITICAL_SECTION *cs);
BOOL CreateDirectoryA(LPCSTR path, void *security);
HANDLE CreateFileA(LPCSTR path, DWORD access, DWORD share, void *security, DWORD creation, DWORD attrs, HANDLE template_file);
BOOL WriteFile(HANDLE file, LPCVOID buffer, DWORD bytes, DWORD *written, void *overlapped);
BOOL CloseHandle(HANDLE h);
DWORD GetProcessId(HANDLE h);
DWORD GetCurrentProcessId(void);
HANDLE GetCurrentProcess(void);
DWORD GetLastError(void);
DWORD GetEnvironmentVariableA(LPCSTR name, char *buffer, DWORD size);
DWORD GetTickCount(void);
void Sleep(DWORD ms);
void *HeapAlloc(HANDLE heap, DWORD flags, SIZE_T size);
BOOL HeapFree(HANDLE heap, DWORD flags, void *mem);
HANDLE GetProcessHeap(void);
LONG InterlockedCompareExchange(volatile LONG *dest, LONG exchange, LONG comparand);
LONG InterlockedExchange(volatile LONG *dest, LONG value);
LONG InterlockedIncrement(volatile LONG *value);
HANDLE CreateThread(void *attrs, SIZE_T stack, LPTHREAD_START_ROUTINE start, void *param, DWORD flags, DWORD *tid);
HMODULE GetModuleHandleA(LPCSTR name);
FARPROC GetProcAddress(HMODULE module, LPCSTR name);
int lstrcmpiA(LPCSTR a, LPCSTR b);
int lstrlenA(LPCSTR s);
BOOL VirtualProtect(void *addr, SIZE_T size, DWORD prot, DWORD *old);
void *VirtualAlloc(void *addr, SIZE_T size, DWORD type, DWORD prot);
void *VirtualAllocEx(HANDLE proc, void *addr, SIZE_T size, DWORD type, DWORD prot);
BOOL VirtualFreeEx(HANDLE proc, void *addr, SIZE_T size, DWORD type);
BOOL FlushInstructionCache(HANDLE proc, LPCVOID addr, SIZE_T size);
BOOL DisableThreadLibraryCalls(HINSTANCE inst);
BOOL CreateProcessA(LPCSTR app, LPSTR cmd, LPSECURITY_ATTRIBUTES pa, LPSECURITY_ATTRIBUTES ta, BOOL inherit, DWORD flags, LPVOID env, LPCSTR cwd, LPSTARTUPINFOA si, LPPROCESS_INFORMATION pi);
HANDLE CreateRemoteThread(HANDLE proc, void *attrs, SIZE_T stack, LPTHREAD_START_ROUTINE start, void *param, DWORD flags, DWORD *tid);
DWORD WaitForSingleObject(HANDLE h, DWORD ms);
BOOL GetExitCodeThread(HANDLE h, DWORD *code);
DWORD ResumeThread(HANDLE h);
BOOL WriteProcessMemory(HANDLE proc, void *base, const void *buffer, SIZE_T size, SIZE_T *written);
BOOL ReadProcessMemory(HANDLE proc, LPCVOID base, LPVOID buffer, SIZE_T size, SIZE_T *read);
BOOL GetThreadContext(HANDLE thread, LPCONTEXT context);
BOOL SetThreadContext(HANDLE thread, const CONTEXT *context);
BOOL VirtualProtectEx(HANDLE proc, LPVOID base, SIZE_T size, DWORD protect, DWORD *old);
HANDLE OpenThread(DWORD access, BOOL inherit, DWORD tid);
BOOL DebugActiveProcess(DWORD pid);
BOOL WaitForDebugEvent(LPDEBUG_EVENT ev, DWORD ms);
BOOL ContinueDebugEvent(DWORD pid, DWORD tid, DWORD status);
int WideCharToMultiByte(unsigned int cp, DWORD flags, LPCWSTR wide, int wide_len, char *out, int out_len, const char *def, BOOL *used_default);

#define ZeroMemory(ptr, size) memset((ptr), 0, (size))

#endif
