#ifdef __INTELLISENSE__
#include "../windows/intellisense.h"
#else
#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#endif
#include <stdio.h>
#include <stdint.h>
#include <stdarg.h>
#include <string.h>

typedef BOOL(WINAPI *CreateProcessAFn)(LPCSTR, LPSTR, LPSECURITY_ATTRIBUTES, LPSECURITY_ATTRIBUTES, BOOL, DWORD, LPVOID, LPCSTR, LPSTARTUPINFOA, LPPROCESS_INFORMATION);
typedef BOOL(WINAPI *CreateProcessWFn)(LPCWSTR, LPWSTR, LPSECURITY_ATTRIBUTES, LPSECURITY_ATTRIBUTES, BOOL, DWORD, LPVOID, LPCWSTR, LPSTARTUPINFOW, LPPROCESS_INFORMATION);
typedef BOOL(WINAPI *DebugActiveProcessFn)(DWORD);
typedef BOOL(WINAPI *WaitForDebugEventFn)(LPDEBUG_EVENT, DWORD);
typedef BOOL(WINAPI *ContinueDebugEventFn)(DWORD, DWORD, DWORD);
typedef BOOL(WINAPI *ReadProcessMemoryFn)(HANDLE, LPCVOID, LPVOID, SIZE_T, SIZE_T *);
typedef BOOL(WINAPI *WriteProcessMemoryFn)(HANDLE, LPVOID, LPCVOID, SIZE_T, SIZE_T *);
typedef BOOL(WINAPI *GetThreadContextFn)(HANDLE, LPCONTEXT);
typedef BOOL(WINAPI *SetThreadContextFn)(HANDLE, const CONTEXT *);
typedef BOOL(WINAPI *VirtualProtectExFn)(HANDLE, LPVOID, SIZE_T, DWORD, PDWORD);
typedef LONG(WINAPI *NtWriteVirtualMemoryFn)(HANDLE, PVOID, PVOID, ULONG, PULONG);
typedef HANDLE(WINAPI *OpenThreadFn)(DWORD, BOOL, DWORD);

static CreateProcessAFn real_CreateProcessA;
static CreateProcessWFn real_CreateProcessW;
static DebugActiveProcessFn real_DebugActiveProcess;
static WaitForDebugEventFn real_WaitForDebugEvent;
static ContinueDebugEventFn real_ContinueDebugEvent;
static ReadProcessMemoryFn real_ReadProcessMemory;
static WriteProcessMemoryFn real_WriteProcessMemory;
static GetThreadContextFn real_GetThreadContext;
static SetThreadContextFn real_SetThreadContext;
static VirtualProtectExFn real_VirtualProtectEx;
static NtWriteVirtualMemoryFn real_NtWriteVirtualMemory;
static OpenThreadFn real_OpenThread;
static unsigned char ntwrite_original[5];
static void *ntwrite_trampoline;

static const char TRACE_DIR[] = "C:\\Users\\Admin\\AppData\\Local\\Temp\\lemonade2_api_trace";
static const char TRACE_LOG[] = "C:\\Users\\Admin\\AppData\\Local\\Temp\\lemonade2_api_trace\\trace.log";

static HANDLE log_file = INVALID_HANDLE_VALUE;
static CRITICAL_SECTION log_lock;
static volatile LONG wpm_seq;
static HANDLE child_process;
static DWORD child_pid;
static DWORD snap_seq;
static DWORD island_seq;
static DWORD island_pages[128];
static DWORD island_hashes[128];
static DWORD island_page_count;
static volatile LONG monitor_started;
static volatile LONG quiet_events = 0;
static DWORD seed_bp_addr = 0x004A6F90u;
static BYTE seed_bp_original;
static volatile LONG seed_bp_armed;
static volatile LONG seed_hook_disabled = -1;
static DWORD seed_bp_step_tid;
static volatile LONG seed_call_seq;
static DWORD seed_ret_bp_addr;
static BYTE seed_ret_bp_original;
static LONG seed_ret_bp_seq;
static DWORD seed_ret_step_tid;
static DWORD generated_base;
static DWORD tea_bp_addr;
static BYTE tea_bp_original;
static volatile LONG tea_bp_armed;
static DWORD tea_bp_step_tid;
static DWORD tea_ret_bp_addr;
static BYTE tea_ret_bp_original;
static DWORD tea_ret_step_tid;
static volatile LONG tea_call_seq;
static LONG tea_ret_bp_seq;
static DWORD tea_ret_ptr;
static DWORD tea_ret_len;
static DWORD mapper_bp_addrs[8];
static BYTE mapper_bp_originals[8];
static int mapper_bp_armed[8];
static DWORD mapper_bp_count;
static DWORD mapper_bp_step_tid;
static volatile LONG mapper_call_seq;
static volatile LONG mapper_global_dump_seq;
static DWORD data_guard_step_tid;
static DWORD data_guard_step_page;
static DWORD data_guard_step_addr;
static DWORD data_guard_step_eip;
static DWORD data_guard_step_access;
static volatile LONG data_guards_armed;
static DWORD data_init_step_tid;
static DWORD data_init_step_addr;
static volatile LONG extended_mapper_breakpoints = -1;

#ifndef THREAD_GET_CONTEXT
#define THREAD_GET_CONTEXT 0x0008
#endif
#ifndef THREAD_QUERY_INFORMATION
#define THREAD_QUERY_INFORMATION 0x0040
#endif
#ifndef THREAD_SET_CONTEXT
#define THREAD_SET_CONTEXT 0x0010
#endif

typedef struct SectionWatch
{
    const char *name;
    DWORD base;
    DWORD size;
    DWORD hash;
    int seen;
} SectionWatch;

typedef struct DataInitBreakpoint
{
    const char *name;
    DWORD addr;
    BYTE expected;
    BYTE original;
    int armed;
    int done;
} DataInitBreakpoint;

static SectionWatch watches[] = {
    {"text", 0x00401000, 0x00091000, 0, 0},
    {"rdata", 0x00492000, 0x0000A000, 0, 0},
    {"data", 0x0049C000, 0x00007000, 0, 0},
    {"text1", 0x004A3000, 0x00030000, 0, 0},
};

static DataInitBreakpoint data_init_bps[] = {
    {"init_crt_runtime_state", 0x004886E3u, 0x55, 0, 0, 0},
    {"init_ctype_case_tables", 0x0048EEE6u, 0x55, 0, 0, 0},
    {"set_thread_locale_ctype_state", 0x0048EC93u, 0x55, 0, 0, 0},
    {"init_object_table_004a02a0", 0x00456BC0u, 0x56, 0, 0, 0},
};

static DWORD data_guard_pages[] = {
    0x0049C000u,
    0x0049D000u,
    0x0049E000u,
    0x0049F000u,
    0x004A0000u,
    0x004A1000u,
    0x004A2000u,
};

static void log_line(const char *fmt, ...);

static int seed_hook_enabled(void)
{
    LONG disabled = seed_hook_disabled;
    if (disabled < 0)
    {
        char value[8] = {0};
        disabled = (GetEnvironmentVariableA("LEMONADE2_DISABLE_SEED_HOOK", value, sizeof(value)) && value[0] != '0') ? 1 : 0;
        InterlockedExchange(&seed_hook_disabled, disabled);
        if (disabled)
            log_line("SEED_HOOK disabled by LEMONADE2_DISABLE_SEED_HOOK\r\n");
    }
    return disabled == 0;
}

static int extended_mapper_breakpoints_enabled(void)
{
    LONG enabled = extended_mapper_breakpoints;
    if (enabled < 0)
    {
        char value[8] = {0};
        enabled = (GetEnvironmentVariableA("LEMONADE2_EXTENDED_MAPPER_BPS", value, sizeof(value)) && value[0] != '0') ? 1 : 0;
        InterlockedExchange(&extended_mapper_breakpoints, enabled);
        if (enabled)
            log_line("MAPPER extended breakpoints enabled\r\n");
    }
    return enabled != 0;
}

static void init_log(void)
{
    CreateDirectoryA(TRACE_DIR, NULL);
    InitializeCriticalSection(&log_lock);
    log_file = CreateFileA(TRACE_LOG, GENERIC_WRITE, FILE_SHARE_READ, NULL,
                           CREATE_ALWAYS, FILE_ATTRIBUTE_NORMAL, NULL);
}

static void log_line(const char *fmt, ...)
{
    char buf[4096];
    DWORD written;
    va_list ap;
    EnterCriticalSection(&log_lock);
    va_start(ap, fmt);
    int n = vsnprintf(buf, sizeof(buf), fmt, ap);
    va_end(ap);
    if (n > 0 && log_file != INVALID_HANDLE_VALUE)
    {
        size_t len = (size_t)n < sizeof(buf) ? (size_t)n : sizeof(buf) - 1;
        WriteFile(log_file, buf, (DWORD)len, &written, NULL);
    }
    LeaveCriticalSection(&log_lock);
}

static DWORD handle_pid(HANDLE h)
{
    DWORD pid = 0;
    if (h)
        pid = GetProcessId(h);
    return pid;
}

static void caller_state(uintptr_t *retaddr, uintptr_t *esp, uintptr_t *ebp)
{
#if defined(__i386__)
    *retaddr = (uintptr_t)__builtin_return_address(0);
    __asm__ __volatile__("movl %%esp,%0" : "=r"(*esp));
    __asm__ __volatile__("movl %%ebp,%0" : "=r"(*ebp));
#else
    *retaddr = (uintptr_t)__builtin_return_address(0);
    *esp = 0;
    *ebp = 0;
#endif
}

static int target_original_sections(uintptr_t addr, SIZE_T size)
{
    if (size > UINTPTR_MAX - addr)
        return 1;
    uintptr_t end = addr + size;
    return (addr < 0x00401000u + 0x91000u && end > 0x00401000u) ||
           (addr < 0x00492000u + 0x0A000u && end > 0x00492000u) ||
           (addr < 0x0049C000u + 0x07000u && end > 0x0049C000u);
}

static DWORD fnv1a(const unsigned char *buf, DWORD size)
{
    DWORD h = 2166136261u;
    for (DWORD i = 0; i < size; i++)
    {
        h ^= buf[i];
        h *= 16777619u;
    }
    return h;
}

static void hex32(const unsigned char *b, SIZE_T size, char out[65]);

static void dump_seed_buffer(LONG seq, DWORD addr, const unsigned char *buf, DWORD size)
{
    char path[MAX_PATH];
    int n = snprintf(path, sizeof(path), "%s\\seedbuf_%05ld_src%08lX_size%08lX.bin",
                     TRACE_DIR, (long)seq, (unsigned long)addr, (unsigned long)size);
    if (n < 0 || (size_t)n >= sizeof(path))
        return;
    HANDLE f = CreateFileA(path, GENERIC_WRITE, FILE_SHARE_READ, NULL, CREATE_ALWAYS,
                           FILE_ATTRIBUTE_NORMAL, NULL);
    if (f == INVALID_HANDLE_VALUE)
        return;
    DWORD written;
    WriteFile(f, buf, size, &written, NULL);
    CloseHandle(f);
}

static void dump_named_buffer(const char *prefix, LONG seq, DWORD addr, const unsigned char *buf, DWORD size)
{
    char path[MAX_PATH];
    int n = snprintf(path, sizeof(path), "%s\\%s_%05ld_src%08lX_size%08lX.bin",
                     TRACE_DIR, prefix, (long)seq, (unsigned long)addr, (unsigned long)size);
    if (n < 0 || (size_t)n >= sizeof(path))
        return;
    HANDLE f = CreateFileA(path, GENERIC_WRITE, FILE_SHARE_READ, NULL, CREATE_ALWAYS,
                           FILE_ATTRIBUTE_NORMAL, NULL);
    if (f == INVALID_HANDLE_VALUE)
        return;
    DWORD written;
    WriteFile(f, buf, size, &written, NULL);
    CloseHandle(f);
}

static void dump_remote_buffer(const char *prefix, LONG seq, DWORD addr, DWORD size)
{
    if (!child_process || !real_ReadProcessMemory || !addr || !size)
        return;
    DWORD capped = size > 0x200000u ? 0x200000u : size;
    unsigned char *buf = (unsigned char *)HeapAlloc(GetProcessHeap(), 0, capped);
    if (!buf)
        return;
    SIZE_T got = 0;
    if (real_ReadProcessMemory(child_process, (LPCVOID)(uintptr_t)addr, buf, capped, &got) && got > 0)
    {
        dump_named_buffer(prefix, seq, addr, buf, (DWORD)got);
        log_line("MAPPER_DUMP prefix=%s seq=%ld addr=%08lX size=%08lX got=%08lX hash=%08lX\r\n",
                 prefix, (long)seq, (unsigned long)addr, (unsigned long)size,
                 (unsigned long)got, (unsigned long)fnv1a(buf, (DWORD)got));
    }
    HeapFree(GetProcessHeap(), 0, buf);
}

static DWORD read_remote_dword(DWORD addr)
{
    DWORD value = 0;
    SIZE_T got = 0;
    if (child_process && real_ReadProcessMemory)
        real_ReadProcessMemory(child_process, (LPCVOID)(uintptr_t)addr, &value, sizeof(value), &got);
    return value;
}

static void dump_mapper_globals(DWORD base, DWORD ex_addr)
{
    if (!base)
        return;
    LONG seq = InterlockedIncrement(&mapper_global_dump_seq);
    if (seq > 32)
        return;
    DWORD g_e4a0 = read_remote_dword(base + 0x3E4A0u);
    DWORD g_e6e0 = read_remote_dword(base + 0x3E6E0u);
    DWORD g_e6e4 = read_remote_dword(base + 0x3E6E4u);
    DWORD g_a134 = read_remote_dword(base + 0x3A134u);
    DWORD g_a138 = read_remote_dword(base + 0x3A138u);
    DWORD g_eb90 = read_remote_dword(base + 0x3EB90u);
    DWORD g_eb8c = read_remote_dword(base + 0x3EB8Cu);
    log_line("MAPPER_GLOBALS#%05ld base=%08lX ex=%08lX f2e4a0=%08lX f2e6e0=%08lX f2e6e4=%08lX f2a134=%08lX f2a138=%08lX f2eb90=%08lX f2eb8c=%08lX\r\n",
             (long)seq, (unsigned long)base, (unsigned long)ex_addr,
             (unsigned long)g_e4a0, (unsigned long)g_e6e0, (unsigned long)g_e6e4,
             (unsigned long)g_a134, (unsigned long)g_a138, (unsigned long)g_eb90,
             (unsigned long)g_eb8c);
    dump_remote_buffer("mapper_global_f2e4a0", seq, g_e4a0, 0x30000u);
    dump_remote_buffer("mapper_global_f2e6e0", seq, g_e6e0, 0x30000u);
    dump_remote_buffer("mapper_global_f2a134", seq, g_a134, 0x30000u);
    dump_remote_buffer("mapper_global_f2a138", seq, g_a138, 0x30000u);
}

static int is_data_guard_page(DWORD page)
{
    for (DWORD i = 0; i < (DWORD)(sizeof(data_guard_pages) / sizeof(data_guard_pages[0])); i++)
    {
        if (data_guard_pages[i] == page)
            return 1;
    }
    return 0;
}

static void arm_data_guard_page(DWORD page)
{
    if (!child_process || !real_VirtualProtectEx || !is_data_guard_page(page))
        return;
    DWORD old_protect = 0;
    if (real_VirtualProtectEx(child_process, (LPVOID)(uintptr_t)page, 0x1000, PAGE_READWRITE | PAGE_GUARD, &old_protect))
    {
        log_line("DATAGUARD_ARM page=%08lX old=%08lX\r\n", (unsigned long)page, (unsigned long)old_protect);
    }
    else
    {
        log_line("DATAGUARD_ARM_FAIL page=%08lX err=%lu\r\n", (unsigned long)page, GetLastError());
    }
}

static void arm_data_guards(void)
{
    if (!child_process || !real_VirtualProtectEx)
        return;
    char enabled[8] = {0};
    if (!GetEnvironmentVariableA("LEMONADE2_DATA_GUARD", enabled, sizeof(enabled)) || enabled[0] == '0')
        return;
    if (InterlockedCompareExchange(&data_guards_armed, 1, 0) != 0)
        return;
    for (DWORD i = 0; i < (DWORD)(sizeof(data_guard_pages) / sizeof(data_guard_pages[0])); i++)
        arm_data_guard_page(data_guard_pages[i]);
}

static void dump_preinit_data(const char *name, DWORD addr)
{
    if (!child_process || !real_ReadProcessMemory)
        return;
    DWORD size = 0x7000u;
    unsigned char *buf = (unsigned char *)HeapAlloc(GetProcessHeap(), 0, size);
    if (!buf)
        return;
    SIZE_T got = 0;
    if (real_ReadProcessMemory(child_process, (LPCVOID)(uintptr_t)0x0049C000u, buf, size, &got) && got == size)
    {
        char path[MAX_PATH];
        int n = snprintf(path, sizeof(path), "%s\\preinit_data_%s_%08lX.bin",
                         TRACE_DIR, name, (unsigned long)addr);
        if (n >= 0 && (size_t)n < sizeof(path))
        {
            HANDLE f = CreateFileA(path, GENERIC_WRITE, FILE_SHARE_READ, NULL, CREATE_ALWAYS,
                                   FILE_ATTRIBUTE_NORMAL, NULL);
            if (f != INVALID_HANDLE_VALUE)
            {
                DWORD written;
                WriteFile(f, buf, size, &written, NULL);
                CloseHandle(f);
                log_line("PREINIT_DATA name=%s addr=%08lX hash=%08lX\r\n",
                         name, (unsigned long)addr, (unsigned long)fnv1a(buf, size));
            }
        }
    }
    HeapFree(GetProcessHeap(), 0, buf);
}

static void ensure_data_init_breakpoints(void)
{
    if (!child_process || !real_ReadProcessMemory || !real_WriteProcessMemory || !real_VirtualProtectEx)
        return;
    if (data_init_step_tid)
        return;
    for (DWORD i = 0; i < (DWORD)(sizeof(data_init_bps) / sizeof(data_init_bps[0])); i++)
    {
        DataInitBreakpoint *bp = &data_init_bps[i];
        if (bp->armed || bp->done)
            continue;
        BYTE current = 0;
        SIZE_T got = 0;
        if (!real_ReadProcessMemory(child_process, (LPCVOID)(uintptr_t)bp->addr, &current, sizeof(current), &got) || got != sizeof(current))
            continue;
        if (current != bp->expected)
            continue;
        DWORD old_protect = 0;
        if (!real_VirtualProtectEx(child_process, (LPVOID)(uintptr_t)bp->addr, 1, PAGE_EXECUTE_READWRITE, &old_protect))
            continue;
        bp->original = current;
        BYTE trap = 0xCC;
        SIZE_T written = 0;
        if (real_WriteProcessMemory(child_process, (LPVOID)(uintptr_t)bp->addr, &trap, sizeof(trap), &written) && written == sizeof(trap))
        {
            FlushInstructionCache(child_process, (LPCVOID)(uintptr_t)bp->addr, 1);
            bp->armed = 1;
            log_line("DATAINIT_ARM name=%s addr=%08lX original=%02X\r\n", bp->name, (unsigned long)bp->addr, bp->original);
        }
        real_VirtualProtectEx(child_process, (LPVOID)(uintptr_t)bp->addr, 1, old_protect, &old_protect);
    }
}

static void ensure_seed_breakpoint(void)
{
    if (!seed_hook_enabled())
        return;
    if (!child_process || !real_ReadProcessMemory || !real_WriteProcessMemory || !real_VirtualProtectEx)
        return;
    if (seed_bp_step_tid)
        return;
    BYTE current = 0;
    SIZE_T got = 0;
    if (!real_ReadProcessMemory(child_process, (LPCVOID)(uintptr_t)seed_bp_addr, &current, sizeof(current), &got) || got != sizeof(current))
        return;
    if (current == 0xCC)
    {
        InterlockedExchange(&seed_bp_armed, 1);
        return;
    }
    if (current != 0x55)
        return;
    DWORD old_protect = 0;
    if (!real_VirtualProtectEx(child_process, (LPVOID)(uintptr_t)seed_bp_addr, 1, PAGE_EXECUTE_READWRITE, &old_protect))
        return;
    seed_bp_original = 0x55;
    BYTE trap = 0xCC;
    SIZE_T written = 0;
    if (real_WriteProcessMemory(child_process, (LPVOID)(uintptr_t)seed_bp_addr, &trap, sizeof(trap), &written) && written == sizeof(trap))
    {
        FlushInstructionCache(child_process, (LPCVOID)(uintptr_t)seed_bp_addr, 1);
        InterlockedExchange(&seed_bp_armed, 1);
    }
    real_VirtualProtectEx(child_process, (LPVOID)(uintptr_t)seed_bp_addr, 1, old_protect, &old_protect);
}

static void restore_seed_breakpoint(void)
{
    if (!child_process || !real_WriteProcessMemory || !real_VirtualProtectEx || !seed_bp_armed)
        return;
    DWORD old_protect = 0;
    if (!real_VirtualProtectEx(child_process, (LPVOID)(uintptr_t)seed_bp_addr, 1, PAGE_EXECUTE_READWRITE, &old_protect))
        return;
    SIZE_T written = 0;
    BYTE original = 0x55;
    real_WriteProcessMemory(child_process, (LPVOID)(uintptr_t)seed_bp_addr, &original, sizeof(original), &written);
    FlushInstructionCache(child_process, (LPCVOID)(uintptr_t)seed_bp_addr, 1);
    real_VirtualProtectEx(child_process, (LPVOID)(uintptr_t)seed_bp_addr, 1, old_protect, &old_protect);
    InterlockedExchange(&seed_bp_armed, 0);
}

static void arm_seed_return_breakpoint(DWORD retaddr, LONG seq)
{
    if (!child_process || !real_ReadProcessMemory || !real_WriteProcessMemory || !real_VirtualProtectEx || !retaddr)
        return;
    if (seed_ret_bp_addr)
        return;
    BYTE current = 0;
    SIZE_T got = 0;
    if (!real_ReadProcessMemory(child_process, (LPCVOID)(uintptr_t)retaddr, &current, sizeof(current), &got) || got != sizeof(current))
        return;
    if (current == 0xCC)
        return;
    DWORD old_protect = 0;
    if (!real_VirtualProtectEx(child_process, (LPVOID)(uintptr_t)retaddr, 1, PAGE_EXECUTE_READWRITE, &old_protect))
        return;
    seed_ret_bp_original = current;
    BYTE trap = 0xCC;
    SIZE_T written = 0;
    if (real_WriteProcessMemory(child_process, (LPVOID)(uintptr_t)retaddr, &trap, sizeof(trap), &written) && written == sizeof(trap))
    {
        FlushInstructionCache(child_process, (LPCVOID)(uintptr_t)retaddr, 1);
        seed_ret_bp_addr = retaddr;
        seed_ret_bp_seq = seq;
    }
    real_VirtualProtectEx(child_process, (LPVOID)(uintptr_t)retaddr, 1, old_protect, &old_protect);
}

static void restore_seed_return_breakpoint(void)
{
    if (!child_process || !real_WriteProcessMemory || !real_VirtualProtectEx || !seed_ret_bp_addr)
        return;
    DWORD old_protect = 0;
    if (!real_VirtualProtectEx(child_process, (LPVOID)(uintptr_t)seed_ret_bp_addr, 1, PAGE_EXECUTE_READWRITE, &old_protect))
        return;
    SIZE_T written = 0;
    real_WriteProcessMemory(child_process, (LPVOID)(uintptr_t)seed_ret_bp_addr, &seed_ret_bp_original, sizeof(seed_ret_bp_original), &written);
    FlushInstructionCache(child_process, (LPCVOID)(uintptr_t)seed_ret_bp_addr, 1);
    real_VirtualProtectEx(child_process, (LPVOID)(uintptr_t)seed_ret_bp_addr, 1, old_protect, &old_protect);
}

static void ensure_tea_breakpoint(DWORD base)
{
    if (!child_process || !real_ReadProcessMemory || !real_WriteProcessMemory || !real_VirtualProtectEx || tea_bp_step_tid)
        return;
    if (!generated_base)
        generated_base = base;
    if (!tea_bp_addr)
        tea_bp_addr = generated_base + 0x14ACu;
    BYTE current = 0;
    SIZE_T got = 0;
    if (!real_ReadProcessMemory(child_process, (LPCVOID)(uintptr_t)tea_bp_addr, &current, sizeof(current), &got) || got != sizeof(current))
        return;
    if (current == 0xCC)
    {
        InterlockedExchange(&tea_bp_armed, 1);
        return;
    }
    DWORD old_protect = 0;
    if (!real_VirtualProtectEx(child_process, (LPVOID)(uintptr_t)tea_bp_addr, 1, PAGE_EXECUTE_READWRITE, &old_protect))
        return;
    tea_bp_original = current;
    BYTE trap = 0xCC;
    SIZE_T written = 0;
    if (real_WriteProcessMemory(child_process, (LPVOID)(uintptr_t)tea_bp_addr, &trap, sizeof(trap), &written) && written == sizeof(trap))
    {
        FlushInstructionCache(child_process, (LPCVOID)(uintptr_t)tea_bp_addr, 1);
        InterlockedExchange(&tea_bp_armed, 1);
    }
    real_VirtualProtectEx(child_process, (LPVOID)(uintptr_t)tea_bp_addr, 1, old_protect, &old_protect);
}

static void restore_tea_breakpoint(void)
{
    if (!child_process || !real_WriteProcessMemory || !real_VirtualProtectEx || !tea_bp_armed)
        return;
    DWORD old_protect = 0;
    if (!real_VirtualProtectEx(child_process, (LPVOID)(uintptr_t)tea_bp_addr, 1, PAGE_EXECUTE_READWRITE, &old_protect))
        return;
    SIZE_T written = 0;
    real_WriteProcessMemory(child_process, (LPVOID)(uintptr_t)tea_bp_addr, &tea_bp_original, sizeof(tea_bp_original), &written);
    FlushInstructionCache(child_process, (LPCVOID)(uintptr_t)tea_bp_addr, 1);
    real_VirtualProtectEx(child_process, (LPVOID)(uintptr_t)tea_bp_addr, 1, old_protect, &old_protect);
    InterlockedExchange(&tea_bp_armed, 0);
}

static void arm_tea_return_breakpoint(DWORD retaddr, LONG seq, DWORD ptr, DWORD len)
{
    if (!child_process || !real_ReadProcessMemory || !real_WriteProcessMemory || !real_VirtualProtectEx || !retaddr || tea_ret_bp_addr)
        return;
    BYTE current = 0;
    SIZE_T got = 0;
    if (!real_ReadProcessMemory(child_process, (LPCVOID)(uintptr_t)retaddr, &current, sizeof(current), &got) || got != sizeof(current))
        return;
    if (current == 0xCC)
        return;
    DWORD old_protect = 0;
    if (!real_VirtualProtectEx(child_process, (LPVOID)(uintptr_t)retaddr, 1, PAGE_EXECUTE_READWRITE, &old_protect))
        return;
    tea_ret_bp_original = current;
    BYTE trap = 0xCC;
    SIZE_T written = 0;
    if (real_WriteProcessMemory(child_process, (LPVOID)(uintptr_t)retaddr, &trap, sizeof(trap), &written) && written == sizeof(trap))
    {
        FlushInstructionCache(child_process, (LPCVOID)(uintptr_t)retaddr, 1);
        tea_ret_bp_addr = retaddr;
        tea_ret_bp_seq = seq;
        tea_ret_ptr = ptr;
        tea_ret_len = len;
    }
    real_VirtualProtectEx(child_process, (LPVOID)(uintptr_t)retaddr, 1, old_protect, &old_protect);
}

static void restore_tea_return_breakpoint(void)
{
    if (!child_process || !real_WriteProcessMemory || !real_VirtualProtectEx || !tea_ret_bp_addr)
        return;
    DWORD old_protect = 0;
    if (!real_VirtualProtectEx(child_process, (LPVOID)(uintptr_t)tea_ret_bp_addr, 1, PAGE_EXECUTE_READWRITE, &old_protect))
        return;
    SIZE_T written = 0;
    real_WriteProcessMemory(child_process, (LPVOID)(uintptr_t)tea_ret_bp_addr, &tea_ret_bp_original, sizeof(tea_ret_bp_original), &written);
    FlushInstructionCache(child_process, (LPCVOID)(uintptr_t)tea_ret_bp_addr, 1);
    real_VirtualProtectEx(child_process, (LPVOID)(uintptr_t)tea_ret_bp_addr, 1, old_protect, &old_protect);
}

static void ensure_mapper_breakpoints(DWORD base)
{
    if (!child_process || !real_ReadProcessMemory || !real_WriteProcessMemory || !real_VirtualProtectEx || mapper_bp_step_tid)
        return;
    if (!generated_base)
        generated_base = base;
    ZeroMemory(mapper_bp_addrs, sizeof(mapper_bp_addrs));
    mapper_bp_addrs[0] = generated_base + 0x1F498u;
    mapper_bp_addrs[1] = generated_base + 0x28237u;
    mapper_bp_count = 2;
    if (extended_mapper_breakpoints_enabled())
    {
        mapper_bp_addrs[2] = generated_base + 0x14E46u;
        mapper_bp_addrs[3] = generated_base + 0x19D2Bu;
        mapper_bp_addrs[4] = generated_base + 0x29E13u;
        mapper_bp_addrs[5] = generated_base + 0x09CBDu;
        mapper_bp_addrs[6] = generated_base + 0x270BAu;
        mapper_bp_addrs[7] = generated_base + 0x270E8u;
        mapper_bp_count = 8;
    }
    for (DWORD i = 0; i < mapper_bp_count; i++)
    {
        if (mapper_bp_armed[i])
            continue;
        BYTE current = 0;
        SIZE_T got = 0;
        if (!real_ReadProcessMemory(child_process, (LPCVOID)(uintptr_t)mapper_bp_addrs[i], &current, sizeof(current), &got) || got != sizeof(current))
        {
            log_line("MAPPER_ARM_READ_FAIL index=%lu addr=%08lX got=%08lX err=%lu\r\n",
                     (unsigned long)i, (unsigned long)mapper_bp_addrs[i], (unsigned long)got, GetLastError());
            continue;
        }
        if (current == 0xCC)
        {
            mapper_bp_armed[i] = 1;
            continue;
        }
        DWORD old_protect = 0;
        if (!real_VirtualProtectEx(child_process, (LPVOID)(uintptr_t)mapper_bp_addrs[i], 1, PAGE_EXECUTE_READWRITE, &old_protect))
        {
            log_line("MAPPER_ARM_PROTECT_FAIL index=%lu addr=%08lX err=%lu\r\n",
                     (unsigned long)i, (unsigned long)mapper_bp_addrs[i], GetLastError());
            continue;
        }
        mapper_bp_originals[i] = current;
        BYTE trap = 0xCC;
        SIZE_T written = 0;
        if (real_WriteProcessMemory(child_process, (LPVOID)(uintptr_t)mapper_bp_addrs[i], &trap, sizeof(trap), &written) && written == sizeof(trap))
        {
            FlushInstructionCache(child_process, (LPCVOID)(uintptr_t)mapper_bp_addrs[i], 1);
            mapper_bp_armed[i] = 1;
            log_line("MAPPER_ARM index=%lu addr=%08lX original=%02X\r\n",
                     (unsigned long)i, (unsigned long)mapper_bp_addrs[i], (unsigned long)current);
        }
        else
        {
            log_line("MAPPER_ARM_WRITE_FAIL index=%lu addr=%08lX written=%08lX err=%lu\r\n",
                     (unsigned long)i, (unsigned long)mapper_bp_addrs[i], (unsigned long)written, GetLastError());
        }
        real_VirtualProtectEx(child_process, (LPVOID)(uintptr_t)mapper_bp_addrs[i], 1, old_protect, &old_protect);
    }
}

static void restore_mapper_breakpoint(DWORD index)
{
    if (!child_process || !real_WriteProcessMemory || !real_VirtualProtectEx || index >= mapper_bp_count || !mapper_bp_armed[index])
        return;
    DWORD old_protect = 0;
    if (!real_VirtualProtectEx(child_process, (LPVOID)(uintptr_t)mapper_bp_addrs[index], 1, PAGE_EXECUTE_READWRITE, &old_protect))
        return;
    SIZE_T written = 0;
    real_WriteProcessMemory(child_process, (LPVOID)(uintptr_t)mapper_bp_addrs[index], &mapper_bp_originals[index], sizeof(mapper_bp_originals[index]), &written);
    FlushInstructionCache(child_process, (LPCVOID)(uintptr_t)mapper_bp_addrs[index], 1);
    real_VirtualProtectEx(child_process, (LPVOID)(uintptr_t)mapper_bp_addrs[index], 1, old_protect, &old_protect);
    mapper_bp_armed[index] = 0;
}

static int handle_seed_callback_event(DEBUG_EVENT *ev)
{
    if (!seed_hook_enabled())
        return 0;
    if (!ev || ev->dwDebugEventCode != EXCEPTION_DEBUG_EVENT || !real_GetThreadContext || !real_SetThreadContext || !real_OpenThread)
        return 0;
    DWORD ex_code = ev->u.Exception.ExceptionRecord.ExceptionCode;
    DWORD ex_addr = (DWORD)(uintptr_t)ev->u.Exception.ExceptionRecord.ExceptionAddress;
    if (ex_code == EXCEPTION_SINGLE_STEP && seed_ret_step_tid == ev->dwThreadId)
    {
        seed_ret_step_tid = 0;
        real_ContinueDebugEvent(ev->dwProcessId, ev->dwThreadId, DBG_CONTINUE);
        return 1;
    }
    if (ex_code == EXCEPTION_SINGLE_STEP && seed_bp_step_tid == ev->dwThreadId)
    {
        seed_bp_step_tid = 0;
        ensure_seed_breakpoint();
        real_ContinueDebugEvent(ev->dwProcessId, ev->dwThreadId, DBG_CONTINUE);
        return 1;
    }
    if (ex_code == EXCEPTION_BREAKPOINT && seed_ret_bp_addr && ex_addr == seed_ret_bp_addr)
    {
        HANDLE th = real_OpenThread(THREAD_GET_CONTEXT | THREAD_SET_CONTEXT | THREAD_QUERY_INFORMATION, FALSE, ev->dwThreadId);
        if (!th)
            return 0;
        CONTEXT ctx;
        ZeroMemory(&ctx, sizeof(ctx));
        ctx.ContextFlags = CONTEXT_FULL;
        if (!real_GetThreadContext(th, &ctx))
        {
            CloseHandle(th);
            return 0;
        }
        log_line("SEEDRET#%05ld tid=%lu ret=%08lX eax=%08lX\r\n",
                 (long)seed_ret_bp_seq, (unsigned long)ev->dwThreadId,
                 (unsigned long)seed_ret_bp_addr, (unsigned long)ctx.Eax);
        restore_seed_return_breakpoint();
        ctx.Eip = seed_ret_bp_addr;
        ctx.EFlags |= 0x100u;
        seed_ret_bp_addr = 0;
        seed_ret_bp_seq = 0;
        seed_ret_step_tid = ev->dwThreadId;
        real_SetThreadContext(th, &ctx);
        CloseHandle(th);
        real_ContinueDebugEvent(ev->dwProcessId, ev->dwThreadId, DBG_CONTINUE);
        return 1;
    }
    if (ex_code != EXCEPTION_BREAKPOINT || ex_addr != seed_bp_addr)
        return 0;

    HANDLE th = real_OpenThread(THREAD_GET_CONTEXT | THREAD_SET_CONTEXT | THREAD_QUERY_INFORMATION, FALSE, ev->dwThreadId);
    if (!th)
        return 0;
    CONTEXT ctx;
    ZeroMemory(&ctx, sizeof(ctx));
    ctx.ContextFlags = CONTEXT_FULL;
    if (!real_GetThreadContext(th, &ctx))
    {
        CloseHandle(th);
        return 0;
    }

    DWORD retaddr = 0;
    DWORD args[3] = {0, 0, 0};
    SIZE_T got = 0;
    real_ReadProcessMemory(child_process, (LPCVOID)(uintptr_t)ctx.Esp, &retaddr, sizeof(retaddr), &got);
    real_ReadProcessMemory(child_process, (LPCVOID)(uintptr_t)(ctx.Esp + 4), args, sizeof(args), &got);
    LONG seq = InterlockedIncrement(&seed_call_seq);
    DWORD sample_size = args[1] > 0x10000u ? 0x10000u : args[1];
    unsigned char *sample = NULL;
    char hex[65] = {0};
    DWORD hash = 0;
    if (sample_size)
        sample = (unsigned char *)HeapAlloc(GetProcessHeap(), 0, sample_size);
    if (sample)
    {
        SIZE_T read = 0;
        if (real_ReadProcessMemory(child_process, (LPCVOID)(uintptr_t)args[0], sample, sample_size, &read) && read > 0)
        {
            sample_size = (DWORD)read;
            hash = fnv1a(sample, sample_size);
            hex32(sample, sample_size, hex);
            dump_seed_buffer(seq, args[0], sample, sample_size);
        }
        HeapFree(GetProcessHeap(), 0, sample);
    }
    log_line("SEEDCALL#%05ld tid=%lu eip=%08lX esp=%08lX ret=%08lX buf=%08lX len=%08lX seed=%08lX sample=%08lX hash=%08lX bytes=%s\r\n",
             (long)seq, (unsigned long)ev->dwThreadId, (unsigned long)ctx.Eip, (unsigned long)ctx.Esp,
             (unsigned long)retaddr,
             (unsigned long)args[0], (unsigned long)args[1], (unsigned long)args[2],
             (unsigned long)sample_size, (unsigned long)hash, hex);

    restore_seed_breakpoint();
    arm_seed_return_breakpoint(retaddr, seq);
    ctx.Eip = seed_bp_addr;
    ctx.EFlags |= 0x100u;
    real_SetThreadContext(th, &ctx);
    CloseHandle(th);
    seed_bp_step_tid = ev->dwThreadId;
    real_ContinueDebugEvent(ev->dwProcessId, ev->dwThreadId, DBG_CONTINUE);
    return 1;
}

static int handle_tea_event(DEBUG_EVENT *ev)
{
    if (!ev || ev->dwDebugEventCode != EXCEPTION_DEBUG_EVENT || !real_GetThreadContext || !real_SetThreadContext || !real_OpenThread)
        return 0;
    DWORD ex_code = ev->u.Exception.ExceptionRecord.ExceptionCode;
    DWORD ex_addr = (DWORD)(uintptr_t)ev->u.Exception.ExceptionRecord.ExceptionAddress;
    if (ex_code == EXCEPTION_SINGLE_STEP && tea_ret_step_tid == ev->dwThreadId)
    {
        tea_ret_step_tid = 0;
        real_ContinueDebugEvent(ev->dwProcessId, ev->dwThreadId, DBG_CONTINUE);
        return 1;
    }
    if (ex_code == EXCEPTION_SINGLE_STEP && tea_bp_step_tid == ev->dwThreadId)
    {
        tea_bp_step_tid = 0;
        if (generated_base)
        {
            ensure_tea_breakpoint(generated_base);
            ensure_mapper_breakpoints(generated_base);
        }
        real_ContinueDebugEvent(ev->dwProcessId, ev->dwThreadId, DBG_CONTINUE);
        return 1;
    }
    if (ex_code == EXCEPTION_BREAKPOINT && tea_ret_bp_addr && ex_addr == tea_ret_bp_addr)
    {
        HANDLE th = real_OpenThread(THREAD_GET_CONTEXT | THREAD_SET_CONTEXT | THREAD_QUERY_INFORMATION, FALSE, ev->dwThreadId);
        if (!th)
            return 0;
        CONTEXT ctx;
        ZeroMemory(&ctx, sizeof(ctx));
        ctx.ContextFlags = CONTEXT_FULL;
        if (!real_GetThreadContext(th, &ctx))
        {
            CloseHandle(th);
            return 0;
        }
        DWORD sample_size = tea_ret_len > 0x10000u ? 0x10000u : tea_ret_len;
        unsigned char *sample = sample_size ? (unsigned char *)HeapAlloc(GetProcessHeap(), 0, sample_size) : NULL;
        char hex[65] = {0};
        DWORD hash = 0;
        if (sample)
        {
            SIZE_T read = 0;
            if (real_ReadProcessMemory(child_process, (LPCVOID)(uintptr_t)tea_ret_ptr, sample, sample_size, &read) && read > 0)
            {
                sample_size = (DWORD)read;
                hash = fnv1a(sample, sample_size);
                hex32(sample, sample_size, hex);
                dump_named_buffer("teapost", tea_ret_bp_seq, tea_ret_ptr, sample, sample_size);
            }
            HeapFree(GetProcessHeap(), 0, sample);
        }
        log_line("TEARET#%05ld tid=%lu ret=%08lX ptr=%08lX len=%08lX sample=%08lX hash=%08lX bytes=%s\r\n",
                 (long)tea_ret_bp_seq, (unsigned long)ev->dwThreadId,
                 (unsigned long)tea_ret_bp_addr, (unsigned long)tea_ret_ptr,
                 (unsigned long)tea_ret_len, (unsigned long)sample_size,
                 (unsigned long)hash, hex);
        restore_tea_return_breakpoint();
        ctx.Eip = tea_ret_bp_addr;
        ctx.EFlags |= 0x100u;
        tea_ret_bp_addr = 0;
        tea_ret_bp_seq = 0;
        tea_ret_ptr = 0;
        tea_ret_len = 0;
        tea_ret_step_tid = ev->dwThreadId;
        real_SetThreadContext(th, &ctx);
        CloseHandle(th);
        real_ContinueDebugEvent(ev->dwProcessId, ev->dwThreadId, DBG_CONTINUE);
        return 1;
    }
    if (ex_code != EXCEPTION_BREAKPOINT || !tea_bp_addr || ex_addr != tea_bp_addr)
        return 0;

    HANDLE th = real_OpenThread(THREAD_GET_CONTEXT | THREAD_SET_CONTEXT | THREAD_QUERY_INFORMATION, FALSE, ev->dwThreadId);
    if (!th)
        return 0;
    CONTEXT ctx;
    ZeroMemory(&ctx, sizeof(ctx));
    ctx.ContextFlags = CONTEXT_FULL;
    if (!real_GetThreadContext(th, &ctx))
    {
        CloseHandle(th);
        return 0;
    }
    DWORD retaddr = 0;
    DWORD args[4] = {0, 0, 0, 0};
    SIZE_T got = 0;
    real_ReadProcessMemory(child_process, (LPCVOID)(uintptr_t)ctx.Esp, &retaddr, sizeof(retaddr), &got);
    real_ReadProcessMemory(child_process, (LPCVOID)(uintptr_t)(ctx.Esp + 4), args, sizeof(args), &got);
    LONG seq = InterlockedIncrement(&tea_call_seq);
    int is_read_chunk_tea = generated_base && retaddr == generated_base + 0x77BCu;
    if (generated_base && retaddr == generated_base + 0x77BCu)
    {
        DWORD read_chunk_ret = 0;
        DWORD read_chunk_args[5] = {0, 0, 0, 0, 0};
        DWORD stream_global = read_remote_dword(generated_base + 0x3E6E0u);
        DWORD length_key = read_remote_dword(generated_base + 0x3E6E4u);
        DWORD source_header = 0;
        DWORD source_prefix0 = 0;
        DWORD source_prefix4 = 0;
        DWORD dest_prefix_before = args[1] ? read_remote_dword(args[1]) : 0;
        if (ctx.Ebp)
        {
            real_ReadProcessMemory(child_process, (LPCVOID)(uintptr_t)(ctx.Ebp + 4u), &read_chunk_ret, sizeof(read_chunk_ret), &got);
            real_ReadProcessMemory(child_process, (LPCVOID)(uintptr_t)(ctx.Ebp + 8u), read_chunk_args, sizeof(read_chunk_args), &got);
            if (read_chunk_args[0])
            {
                source_header = read_remote_dword(read_chunk_args[0]);
                source_prefix0 = read_remote_dword(read_chunk_args[0] + 4u);
                source_prefix4 = read_remote_dword(read_chunk_args[0] + 8u);
                dump_remote_buffer("readchunk_source", seq, read_chunk_args[0], args[2] + 4u);
            }
            if (read_chunk_args[4])
                dump_remote_buffer("readchunk_aux", seq, read_chunk_args[4], 0x1004u);
        }
        log_line("TEA_READCHUNK_CTX#%05ld tid=%lu parent_ret=%08lX frame_stream=%08lX frame_dst=%08lX frame_lenptr=%08lX frame_seed=%08lX frame_aux=%08lX global_stream=%08lX length_key=%08lX source_header=%08lX source_prefix=%08lX%08lX dest_prefix_before=%08lX\r\n",
                 (long)seq, (unsigned long)ev->dwThreadId, (unsigned long)read_chunk_ret,
                 (unsigned long)read_chunk_args[0], (unsigned long)read_chunk_args[1],
                 (unsigned long)read_chunk_args[2], (unsigned long)read_chunk_args[3],
                 (unsigned long)read_chunk_args[4], (unsigned long)stream_global,
                 (unsigned long)length_key, (unsigned long)source_header,
                 (unsigned long)source_prefix0, (unsigned long)source_prefix4,
                 (unsigned long)dest_prefix_before);
    }
    DWORD sample_limit = is_read_chunk_tea ? 0x80000u : 0x10000u;
    DWORD sample_size = args[2] > sample_limit ? sample_limit : args[2];
    unsigned char *sample = sample_size ? (unsigned char *)HeapAlloc(GetProcessHeap(), 0, sample_size) : NULL;
    char hex[65] = {0};
    DWORD hash = 0;
    if (sample)
    {
        SIZE_T read = 0;
        if (real_ReadProcessMemory(child_process, (LPCVOID)(uintptr_t)args[1], sample, sample_size, &read) && read > 0)
        {
            sample_size = (DWORD)read;
            hash = fnv1a(sample, sample_size);
            hex32(sample, sample_size, hex);
            dump_named_buffer("teapre", seq, args[1], sample, sample_size);
        }
        HeapFree(GetProcessHeap(), 0, sample);
    }
    log_line("TEACALL#%05ld tid=%lu eip=%08lX esp=%08lX ret=%08lX seed=%08lX ptr=%08lX len=%08lX mode=%08lX sample=%08lX hash=%08lX bytes=%s\r\n",
             (long)seq, (unsigned long)ev->dwThreadId, (unsigned long)ctx.Eip,
             (unsigned long)ctx.Esp, (unsigned long)retaddr, (unsigned long)args[0],
             (unsigned long)args[1], (unsigned long)args[2], (unsigned long)args[3],
             (unsigned long)sample_size, (unsigned long)hash, hex);
    restore_tea_breakpoint();
    if (generated_base)
        ensure_mapper_breakpoints(generated_base);
    arm_tea_return_breakpoint(retaddr, seq, args[1], args[2]);
    ctx.Eip = tea_bp_addr;
    ctx.EFlags |= 0x100u;
    real_SetThreadContext(th, &ctx);
    CloseHandle(th);
    tea_bp_step_tid = ev->dwThreadId;
    real_ContinueDebugEvent(ev->dwProcessId, ev->dwThreadId, DBG_CONTINUE);
    return 1;
}

static int handle_mapper_event(DEBUG_EVENT *ev)
{
    if (!ev || ev->dwDebugEventCode != EXCEPTION_DEBUG_EVENT || !real_GetThreadContext || !real_SetThreadContext || !real_OpenThread)
        return 0;
    DWORD ex_code = ev->u.Exception.ExceptionRecord.ExceptionCode;
    DWORD ex_addr = (DWORD)(uintptr_t)ev->u.Exception.ExceptionRecord.ExceptionAddress;
    if (ex_code == EXCEPTION_SINGLE_STEP && mapper_bp_step_tid == ev->dwThreadId)
    {
        mapper_bp_step_tid = 0;
        if (generated_base)
            ensure_mapper_breakpoints(generated_base);
        real_ContinueDebugEvent(ev->dwProcessId, ev->dwThreadId, DBG_CONTINUE);
        return 1;
    }
    if (ex_code != EXCEPTION_BREAKPOINT)
        return 0;
    DWORD index = mapper_bp_count;
    for (DWORD i = 0; i < mapper_bp_count; i++)
    {
        if (mapper_bp_armed[i] && ex_addr == mapper_bp_addrs[i])
        {
            index = i;
            break;
        }
    }
    if (index >= mapper_bp_count)
        return 0;
    HANDLE th = real_OpenThread(THREAD_GET_CONTEXT | THREAD_SET_CONTEXT | THREAD_QUERY_INFORMATION, FALSE, ev->dwThreadId);
    if (!th)
        return 0;
    CONTEXT ctx;
    ZeroMemory(&ctx, sizeof(ctx));
    ctx.ContextFlags = CONTEXT_FULL;
    if (!real_GetThreadContext(th, &ctx))
    {
        CloseHandle(th);
        return 0;
    }
    DWORD retaddr = 0;
    DWORD args[3] = {0, 0, 0};
    SIZE_T got = 0;
    real_ReadProcessMemory(child_process, (LPCVOID)(uintptr_t)ctx.Esp, &retaddr, sizeof(retaddr), &got);
    real_ReadProcessMemory(child_process, (LPCVOID)(uintptr_t)(ctx.Esp + 4), args, sizeof(args), &got);
    LONG seq = InterlockedIncrement(&mapper_call_seq);
    if (index == 0)
    {
        DWORD stream_ptr_addr = ctx.Ebp - 0x34u;
        DWORD stream = 0;
        real_ReadProcessMemory(child_process, (LPCVOID)(uintptr_t)stream_ptr_addr, &stream, sizeof(stream), &got);
        log_line("MAPPER28237_CALLSITE#%05ld tid=%lu eip=%08lX stream_ptr=%08lX initial=%08lX got=%08lX\r\n",
                 (long)seq, (unsigned long)ev->dwThreadId, (unsigned long)ctx.Eip,
                 (unsigned long)stream_ptr_addr, (unsigned long)stream, (unsigned long)got);
        dump_remote_buffer("mapper28237_callsite_initial", seq, stream, 0x30000u);
    }
    else if (index == 1)
    {
        DWORD stream_ptr_addr = ctx.Ebp - 0x34u;
        DWORD stream = 0;
        real_ReadProcessMemory(child_process, (LPCVOID)(uintptr_t)stream_ptr_addr, &stream, sizeof(stream), &got);
        log_line("MAPPER28237_RETURNSITE#%05ld tid=%lu eip=%08lX stream_ptr=%08lX selected=%08lX got=%08lX\r\n",
                 (long)seq, (unsigned long)ev->dwThreadId, (unsigned long)ctx.Eip,
                 (unsigned long)stream_ptr_addr, (unsigned long)stream, (unsigned long)got);
        dump_remote_buffer("mapper28237_returnsite_selected", seq, stream, 0x30000u);
    }
    else if (index == 2)
    {
        DWORD seed_field = 0;
        if (ctx.Ecx)
            real_ReadProcessMemory(child_process, (LPCVOID)(uintptr_t)(ctx.Ecx + 0x1D34u), &seed_field, sizeof(seed_field), &got);
        log_line("MAPPER14E46_CALL#%05ld tid=%lu eip=%08lX ret=%08lX this=%08lX mode=%08lX seed_field=%08lX got=%08lX\r\n",
                 (long)seq, (unsigned long)ev->dwThreadId, (unsigned long)ctx.Eip, (unsigned long)retaddr,
                 (unsigned long)ctx.Ecx, (unsigned long)args[0], (unsigned long)seed_field, (unsigned long)got);
    }
    else
    if (index == 3)
    {
        DWORD seed_field = 0;
        DWORD table_base = 0;
        DWORD table_offset = 0;
        DWORD table_end = 0;
        DWORD config_ptr = 0;
        DWORD cfg38 = 0;
        DWORD cfg3c = 0;
        BYTE cfg_e875 = 0;
        if (ctx.Ecx)
        {
            real_ReadProcessMemory(child_process, (LPCVOID)(uintptr_t)(ctx.Ecx + 0x1D34u), &seed_field, sizeof(seed_field), &got);
            real_ReadProcessMemory(child_process, (LPCVOID)(uintptr_t)(ctx.Ecx + 0x3D40u), &table_base, sizeof(table_base), &got);
            real_ReadProcessMemory(child_process, (LPCVOID)(uintptr_t)(ctx.Ecx + 0x3BFCu), &table_offset, sizeof(table_offset), &got);
            real_ReadProcessMemory(child_process, (LPCVOID)(uintptr_t)(ctx.Ecx + 0x3C00u), &table_end, sizeof(table_end), &got);
        }
        if (generated_base)
        {
            real_ReadProcessMemory(child_process, (LPCVOID)(uintptr_t)(generated_base + 0x3E6D8u), &config_ptr, sizeof(config_ptr), &got);
            real_ReadProcessMemory(child_process, (LPCVOID)(uintptr_t)(generated_base + 0x3E875u), &cfg_e875, sizeof(cfg_e875), &got);
        }
        if (config_ptr)
        {
            real_ReadProcessMemory(child_process, (LPCVOID)(uintptr_t)(config_ptr + 0x38u), &cfg38, sizeof(cfg38), &got);
            real_ReadProcessMemory(child_process, (LPCVOID)(uintptr_t)(config_ptr + 0x3Cu), &cfg3c, sizeof(cfg3c), &got);
        }
        log_line("MAPPER19D2B_CALL#%05ld tid=%lu eip=%08lX ret=%08lX this=%08lX seed_arg=%08lX check_only=%08lX seed_field=%08lX table=%08lX off=%08lX end=%08lX cfg=%08lX cfg38=%08lX cfg3c=%08lX e875=%02X got=%08lX\r\n",
                 (long)seq, (unsigned long)ev->dwThreadId, (unsigned long)ctx.Eip, (unsigned long)retaddr,
                 (unsigned long)ctx.Ecx, (unsigned long)args[0], (unsigned long)args[1], (unsigned long)seed_field,
                 (unsigned long)table_base, (unsigned long)table_offset, (unsigned long)table_end,
                 (unsigned long)config_ptr, (unsigned long)cfg38, (unsigned long)cfg3c, (unsigned long)cfg_e875, (unsigned long)got);
        if (table_base && table_offset)
            dump_remote_buffer("mapper19d2b_validation_table", seq, table_base + table_offset, 0x1000u);
    }
    else
    {
        DWORD g_e4a0 = generated_base ? read_remote_dword(generated_base + 0x3E4A0u) : 0;
        DWORD g_e6e0 = generated_base ? read_remote_dword(generated_base + 0x3E6E0u) : 0;
        DWORD g_e6e4 = generated_base ? read_remote_dword(generated_base + 0x3E6E4u) : 0;
        DWORD g_a134 = generated_base ? read_remote_dword(generated_base + 0x3A134u) : 0;
        DWORD g_a138 = generated_base ? read_remote_dword(generated_base + 0x3A138u) : 0;
        DWORD stream0 = g_e6e0 ? read_remote_dword(g_e6e0) : 0;
        DWORD stream4 = g_e6e0 ? read_remote_dword(g_e6e0 + 4u) : 0;
        log_line("MAPPER_KEYSITE#%05ld index=%lu tid=%lu eip=%08lX ret=%08lX eax=%08lX ebx=%08lX ecx=%08lX edx=%08lX esi=%08lX edi=%08lX f2e4a0=%08lX f2e6e0=%08lX f2e6e4=%08lX f2a134=%08lX f2a138=%08lX stream0=%08lX stream4=%08lX\r\n",
                 (long)seq, (unsigned long)index, (unsigned long)ev->dwThreadId,
                 (unsigned long)ctx.Eip, (unsigned long)retaddr,
                 (unsigned long)ctx.Eax, (unsigned long)ctx.Ebx, (unsigned long)ctx.Ecx,
                 (unsigned long)ctx.Edx, (unsigned long)ctx.Esi, (unsigned long)ctx.Edi,
                 (unsigned long)g_e4a0, (unsigned long)g_e6e0, (unsigned long)g_e6e4,
                 (unsigned long)g_a134, (unsigned long)g_a138,
                 (unsigned long)stream0, (unsigned long)stream4);
        if (index == 7)
        {
            dump_remote_buffer("mapper09cbd_first_chain", seq, g_a138, 0x30000u);
            dump_remote_buffer("mapper09cbd_second_chain", seq, g_a134, 0x30000u);
        }
    }
    restore_mapper_breakpoint(index);
    ctx.Eip = mapper_bp_addrs[index];
    ctx.EFlags |= 0x100u;
    real_SetThreadContext(th, &ctx);
    CloseHandle(th);
    mapper_bp_step_tid = ev->dwThreadId;
    real_ContinueDebugEvent(ev->dwProcessId, ev->dwThreadId, DBG_CONTINUE);
    return 1;
}

static int handle_data_guard_event(DEBUG_EVENT *ev)
{
    if (!ev || ev->dwDebugEventCode != EXCEPTION_DEBUG_EVENT || !real_GetThreadContext || !real_SetThreadContext)
        return 0;
    DWORD code = ev->u.Exception.ExceptionRecord.ExceptionCode;
    if (code == EXCEPTION_SINGLE_STEP && data_guard_step_tid == ev->dwThreadId)
    {
        DWORD post = 0;
        SIZE_T got = 0;
        if (child_process && real_ReadProcessMemory && data_guard_step_addr)
            real_ReadProcessMemory(child_process, (LPCVOID)(uintptr_t)data_guard_step_addr, &post, sizeof(post), &got);
        log_line("DATAGUARD_POST tid=%lu eip=%08lX access=%lu addr=%08lX page=%08lX got=%08lX value=%08lX\r\n",
                 (unsigned long)ev->dwThreadId, (unsigned long)data_guard_step_eip,
                 (unsigned long)data_guard_step_access, (unsigned long)data_guard_step_addr,
                 (unsigned long)data_guard_step_page, (unsigned long)got, (unsigned long)post);
        arm_data_guard_page(data_guard_step_page);
        data_guard_step_tid = 0;
        data_guard_step_page = 0;
        data_guard_step_addr = 0;
        data_guard_step_eip = 0;
        data_guard_step_access = 0;
        real_ContinueDebugEvent(ev->dwProcessId, ev->dwThreadId, DBG_CONTINUE);
        return 1;
    }
    if (code != STATUS_GUARD_PAGE_VIOLATION)
        return 0;
    DWORD access_addr = 0;
    DWORD access_kind = 0;
    if (ev->u.Exception.ExceptionRecord.NumberParameters >= 2)
    {
        access_kind = (DWORD)ev->u.Exception.ExceptionRecord.ExceptionInformation[0];
        access_addr = (DWORD)ev->u.Exception.ExceptionRecord.ExceptionInformation[1];
    }
    else
    {
        access_addr = (DWORD)(uintptr_t)ev->u.Exception.ExceptionRecord.ExceptionAddress;
    }
    DWORD page = access_addr & 0xFFFFF000u;
    if (!is_data_guard_page(page))
        return 0;
    if (!real_OpenThread)
    {
        HMODULE k32 = GetModuleHandleA("kernel32.dll");
        real_OpenThread = (OpenThreadFn)GetProcAddress(k32, "OpenThread");
    }
    HANDLE th = real_OpenThread ? real_OpenThread(THREAD_GET_CONTEXT | THREAD_QUERY_INFORMATION | THREAD_SET_CONTEXT, FALSE, ev->dwThreadId) : NULL;
    if (!th)
        return 0;
    CONTEXT ctx;
    ZeroMemory(&ctx, sizeof(ctx));
    ctx.ContextFlags = CONTEXT_FULL;
    if (!real_GetThreadContext(th, &ctx))
    {
        CloseHandle(th);
        return 0;
    }
    DWORD pre = 0;
    SIZE_T pre_got = 0;
    if (child_process && real_ReadProcessMemory)
        real_ReadProcessMemory(child_process, (LPCVOID)(uintptr_t)access_addr, &pre, sizeof(pre), &pre_got);
    log_line("DATAGUARD_HIT tid=%lu eip=%08lX access=%lu addr=%08lX page=%08lX got=%08lX pre=%08lX eax=%08lX ebx=%08lX ecx=%08lX edx=%08lX esi=%08lX edi=%08lX\r\n",
             (unsigned long)ev->dwThreadId, (unsigned long)ctx.Eip,
             (unsigned long)access_kind, (unsigned long)access_addr, (unsigned long)page,
             (unsigned long)pre_got, (unsigned long)pre,
             (unsigned long)ctx.Eax, (unsigned long)ctx.Ebx, (unsigned long)ctx.Ecx,
             (unsigned long)ctx.Edx, (unsigned long)ctx.Esi, (unsigned long)ctx.Edi);
    ctx.EFlags |= 0x100u;
    real_SetThreadContext(th, &ctx);
    CloseHandle(th);
    data_guard_step_tid = ev->dwThreadId;
    data_guard_step_page = page;
    data_guard_step_addr = access_addr;
    data_guard_step_eip = ctx.Eip;
    data_guard_step_access = access_kind;
    real_ContinueDebugEvent(ev->dwProcessId, ev->dwThreadId, DBG_CONTINUE);
    return 1;
}

static int handle_data_init_event(DEBUG_EVENT *ev)
{
    if (!ev || ev->dwDebugEventCode != EXCEPTION_DEBUG_EVENT || !real_GetThreadContext || !real_SetThreadContext || !real_OpenThread)
        return 0;
    DWORD ex_code = ev->u.Exception.ExceptionRecord.ExceptionCode;
    DWORD ex_addr = (DWORD)(uintptr_t)ev->u.Exception.ExceptionRecord.ExceptionAddress;
    if (ex_code == EXCEPTION_SINGLE_STEP && data_init_step_tid == ev->dwThreadId)
    {
        data_init_step_tid = 0;
        data_init_step_addr = 0;
        ensure_data_init_breakpoints();
        real_ContinueDebugEvent(ev->dwProcessId, ev->dwThreadId, DBG_CONTINUE);
        return 1;
    }
    if (ex_code != EXCEPTION_BREAKPOINT)
        return 0;
    for (DWORD i = 0; i < (DWORD)(sizeof(data_init_bps) / sizeof(data_init_bps[0])); i++)
    {
        DataInitBreakpoint *bp = &data_init_bps[i];
        if (!bp->armed || bp->done || ex_addr != bp->addr)
            continue;
        HANDLE th = real_OpenThread(THREAD_GET_CONTEXT | THREAD_SET_CONTEXT | THREAD_QUERY_INFORMATION, FALSE, ev->dwThreadId);
        if (!th)
            return 0;
        CONTEXT ctx;
        ZeroMemory(&ctx, sizeof(ctx));
        ctx.ContextFlags = CONTEXT_FULL;
        if (!real_GetThreadContext(th, &ctx))
        {
            CloseHandle(th);
            return 0;
        }
        dump_preinit_data(bp->name, bp->addr);
        DWORD old_protect = 0;
        if (real_VirtualProtectEx(child_process, (LPVOID)(uintptr_t)bp->addr, 1, PAGE_EXECUTE_READWRITE, &old_protect))
        {
            SIZE_T written = 0;
            real_WriteProcessMemory(child_process, (LPVOID)(uintptr_t)bp->addr, &bp->original, sizeof(bp->original), &written);
            FlushInstructionCache(child_process, (LPCVOID)(uintptr_t)bp->addr, 1);
            real_VirtualProtectEx(child_process, (LPVOID)(uintptr_t)bp->addr, 1, old_protect, &old_protect);
        }
        bp->armed = 0;
        bp->done = 1;
        ctx.Eip = bp->addr;
        ctx.EFlags |= 0x100u;
        real_SetThreadContext(th, &ctx);
        CloseHandle(th);
        data_init_step_tid = ev->dwThreadId;
        data_init_step_addr = bp->addr;
        log_line("DATAINIT_HIT name=%s addr=%08lX tid=%lu\r\n", bp->name, (unsigned long)bp->addr, (unsigned long)ev->dwThreadId);
        real_ContinueDebugEvent(ev->dwProcessId, ev->dwThreadId, DBG_CONTINUE);
        return 1;
    }
    return 0;
}

static void dump_snapshot(const SectionWatch *w, const unsigned char *buf, DWORD size, DWORD seq, DWORD code, DWORD tid, DWORD ex_code, DWORD ex_addr)
{
    char path[MAX_PATH];
    int n = snprintf(path, sizeof(path), "%s\\snap_%04lu_%s_code%lu_tid%lu_ex%08lX_at%08lX.bin",
                     TRACE_DIR, (unsigned long)seq, w->name,
                     (unsigned long)code, (unsigned long)tid,
                     (unsigned long)ex_code, (unsigned long)ex_addr);
    if (n < 0 || (size_t)n >= sizeof(path))
    {
        return;
    }
    HANDLE f = CreateFileA(path, GENERIC_WRITE, FILE_SHARE_READ, NULL, CREATE_ALWAYS,
                           FILE_ATTRIBUTE_NORMAL, NULL);
    if (f == INVALID_HANDLE_VALUE)
        return;
    DWORD written;
    WriteFile(f, buf, size, &written, NULL);
    CloseHandle(f);
}

static int island_page_changed(DWORD page, DWORD hash)
{
    for (DWORD i = 0; i < island_page_count; i++)
    {
        if (island_pages[i] == page)
        {
            if (island_hashes[i] == hash)
                return 0;
            island_hashes[i] = hash;
            return 1;
        }
    }
    if (island_page_count < (DWORD)(sizeof(island_pages) / sizeof(island_pages[0])))
    {
        island_pages[island_page_count++] = page;
        island_hashes[island_page_count - 1] = hash;
    }
    return 1;
}

static void dump_island_region(DWORD pid, DWORD tid, DWORD ex_code, DWORD ex_addr)
{
    if (!child_process || !real_ReadProcessMemory)
        return;
    if (ex_addr < 0x00800000u || ex_addr >= 0x02000000u)
        return;

    DWORD base = ex_addr & 0xFFFF0000u;
    DWORD size = 0x00020000u;
    unsigned char *buf = (unsigned char *)HeapAlloc(GetProcessHeap(), 0, size);
    if (!buf)
        return;

    SIZE_T got = 0;
    BOOL ok = real_ReadProcessMemory(child_process, (LPCVOID)(uintptr_t)base, buf, size, &got);
    if (ok && got > 0)
    {
        DWORD hash = fnv1a(buf, (DWORD)got);
        if (!island_page_changed(base, hash))
        {
            HeapFree(GetProcessHeap(), 0, buf);
            return;
        }
        char path[MAX_PATH];
        DWORD seq = ++island_seq;
        int n = snprintf(path, sizeof(path), "%s\\island_%04lu_pid%lu_tid%lu_ex%08lX_at%08lX_base%08lX_size%08lX.bin",
                         TRACE_DIR, (unsigned long)seq, (unsigned long)pid, (unsigned long)tid,
                         (unsigned long)ex_code, (unsigned long)ex_addr,
                         (unsigned long)base, (unsigned long)got);
        if (n >= 0 && (size_t)n < sizeof(path))
        {
            HANDLE f = CreateFileA(path, GENERIC_WRITE, FILE_SHARE_READ, NULL, CREATE_ALWAYS,
                                   FILE_ATTRIBUTE_NORMAL, NULL);
            if (f != INVALID_HANDLE_VALUE)
            {
                DWORD written;
                WriteFile(f, buf, (DWORD)got, &written, NULL);
                CloseHandle(f);
                log_line("ISLAND#%04lu pid=%lu tid=%lu ex=%08lX addr=%08lX base=%08lX size=%08lX hash=%08lX\r\n",
                         (unsigned long)seq, (unsigned long)pid, (unsigned long)tid,
                          (unsigned long)ex_code, (unsigned long)ex_addr,
                         (unsigned long)base, (unsigned long)got, (unsigned long)hash);
            }
        }
    }
    HeapFree(GetProcessHeap(), 0, buf);
}

static void snapshot_child(const DEBUG_EVENT *ev)
{
    if (!child_process)
        return;
    DWORD ex_code = 0, ex_addr = 0;
    if (ev && ev->dwDebugEventCode == EXCEPTION_DEBUG_EVENT)
    {
        ex_code = ev->u.Exception.ExceptionRecord.ExceptionCode;
        ex_addr = (DWORD)(uintptr_t)ev->u.Exception.ExceptionRecord.ExceptionAddress;
    }
    for (int i = 0; i < (int)(sizeof(watches) / sizeof(watches[0])); i++)
    {
        SectionWatch *w = &watches[i];
        unsigned char *buf = (unsigned char *)HeapAlloc(GetProcessHeap(), 0, w->size);
        if (!buf)
            continue;
        SIZE_T got = 0;
        BOOL ok = real_ReadProcessMemory(child_process, (LPCVOID)(uintptr_t)w->base, buf, w->size, &got);
        if (ok && got == w->size)
        {
            DWORD h = fnv1a(buf, w->size);
            if (!w->seen || h != w->hash)
            {
                DWORD seq = ++snap_seq;
                w->seen = 1;
                w->hash = h;
                log_line("SNAP#%04lu section=%s hash=%08lX event=%lu pid=%lu tid=%lu ex=%08lX addr=%08lX first=%02X%02X%02X%02X\r\n",
                         (unsigned long)seq, w->name, (unsigned long)h,
                         ev ? (unsigned long)ev->dwDebugEventCode : 0,
                         ev ? (unsigned long)ev->dwProcessId : 0,
                         ev ? (unsigned long)ev->dwThreadId : 0,
                         (unsigned long)ex_code, (unsigned long)ex_addr,
                         buf[0], buf[1], buf[2], buf[3]);
                if (seq <= 250)
                    dump_snapshot(w, buf, w->size, seq, ev ? ev->dwDebugEventCode : 0, ev ? ev->dwThreadId : 0, ex_code, ex_addr);
            }
        }
        else if (!w->seen)
        {
            log_line("SNAP_READ_FAIL section=%s base=%08lX size=%08lX ok=%d got=%08lX err=%lu\r\n",
                     w->name, (unsigned long)w->base, (unsigned long)w->size,
                     ok, (unsigned long)got, GetLastError());
        }
        HeapFree(GetProcessHeap(), 0, buf);
    }
    ensure_seed_breakpoint();
    ensure_data_init_breakpoints();
    arm_data_guards();
}

static void log_event_context(const DEBUG_EVENT *ev)
{
    if (!ev || ev->dwDebugEventCode != EXCEPTION_DEBUG_EVENT || !real_GetThreadContext)
        return;
    if (!real_OpenThread)
    {
        HMODULE k32 = GetModuleHandleA("kernel32.dll");
        real_OpenThread = (OpenThreadFn)GetProcAddress(k32, "OpenThread");
    }
    if (!real_OpenThread)
        return;
    HANDLE th = real_OpenThread(THREAD_GET_CONTEXT | THREAD_QUERY_INFORMATION, FALSE, ev->dwThreadId);
    if (!th)
    {
        log_line("EVCTX_OPEN_FAIL pid=%lu tid=%lu err=%lu\r\n",
                 (unsigned long)ev->dwProcessId, (unsigned long)ev->dwThreadId, GetLastError());
        return;
    }
    CONTEXT ctx;
    ZeroMemory(&ctx, sizeof(ctx));
    ctx.ContextFlags = CONTEXT_FULL | CONTEXT_DEBUG_REGISTERS;
    if (real_GetThreadContext(th, &ctx))
    {
        DWORD ex_code = ev->u.Exception.ExceptionRecord.ExceptionCode;
        DWORD ex_addr = (DWORD)(uintptr_t)ev->u.Exception.ExceptionRecord.ExceptionAddress;
        log_line("EVCTX pid=%lu tid=%lu ex=%08lX addr=%08lX eip=%08lX esp=%08lX ebp=%08lX eax=%08lX ebx=%08lX ecx=%08lX edx=%08lX esi=%08lX edi=%08lX eflags=%08lX dr0=%08lX dr1=%08lX dr2=%08lX dr3=%08lX dr6=%08lX dr7=%08lX\r\n",
                 (unsigned long)ev->dwProcessId, (unsigned long)ev->dwThreadId,
                 (unsigned long)ex_code,
                 (unsigned long)ex_addr,
                 (unsigned long)ctx.Eip, (unsigned long)ctx.Esp, (unsigned long)ctx.Ebp,
                 (unsigned long)ctx.Eax, (unsigned long)ctx.Ebx, (unsigned long)ctx.Ecx,
                 (unsigned long)ctx.Edx, (unsigned long)ctx.Esi, (unsigned long)ctx.Edi,
                 (unsigned long)ctx.EFlags,
                 (unsigned long)ctx.Dr0, (unsigned long)ctx.Dr1, (unsigned long)ctx.Dr2,
                 (unsigned long)ctx.Dr3, (unsigned long)ctx.Dr6, (unsigned long)ctx.Dr7);
        DWORD base = 0;
        if (ex_addr >= 0x00800000u && ex_addr < 0x02000000u)
        {
            DWORD low16 = ex_addr & 0x0000FFFFu;
            if (low16 == 0x7D34u)
                base = ex_addr - 0x7D34u;
            else if (low16 == 0x95DBu)
                base = ex_addr - 0x295DBu;
            else if (low16 == 0x983Fu)
                base = ex_addr - 0x2983Fu;
            else if (low16 == 0x9ECFu)
                base = ex_addr - 0x29ECFu;
            if (base)
            {
                ensure_tea_breakpoint(base);
                ensure_mapper_breakpoints(base);
                if (low16 == 0x95DBu || low16 == 0x983Fu || low16 == 0x9ECFu)
                    dump_mapper_globals(base, ex_addr);
            }
        }
        dump_island_region(ev->dwProcessId, ev->dwThreadId, ex_code, ex_addr);
    }
    else
    {
        log_line("EVCTX_GTC_FAIL pid=%lu tid=%lu err=%lu\r\n",
                 (unsigned long)ev->dwProcessId, (unsigned long)ev->dwThreadId, GetLastError());
    }
    CloseHandle(th);
}

static DWORD WINAPI monitor_thread(LPVOID unused)
{
    (void)unused;
    DWORD start = GetTickCount();
    while (GetTickCount() - start < 120000)
    {
        snapshot_child(NULL);
        Sleep(10);
    }
    log_line("MONITOR_DONE child_pid=%lu snaps=%lu\r\n", (unsigned long)child_pid, (unsigned long)snap_seq);
    return 0;
}

static void maybe_start_monitor(void)
{
    if (child_process && InterlockedCompareExchange(&monitor_started, 1, 0) == 0)
    {
        CreateThread(NULL, 0, monitor_thread, NULL, 0, NULL);
        log_line("MONITOR_START child_pid=%lu hProcess=%08lX\r\n",
                 (unsigned long)child_pid, (unsigned long)(uintptr_t)child_process);
    }
}

static void hex32(const unsigned char *b, SIZE_T size, char out[65])
{
    SIZE_T n = size < 32 ? size : 32;
    for (SIZE_T i = 0; i < n; i++)
    {
        snprintf(out + i * 2, 65 - i * 2, "%02X", b[i]);
    }
    out[n * 2] = 0;
}

static void dump_wpm(LONG seq, DWORD target_pid, uintptr_t dst, const void *src, SIZE_T size)
{
    char path[MAX_PATH];
    if (!src || size == 0)
        return;
    int n = snprintf(path, sizeof(path), "%s\\wpm_%05ld_pid%lu_dst%08lX_size%08lX.bin",
                     TRACE_DIR, (long)seq, (unsigned long)target_pid,
                     (unsigned long)dst, (unsigned long)size);
    if (n < 0 || (size_t)n >= sizeof(path))
    {
        return;
    }
    HANDLE f = CreateFileA(path, GENERIC_WRITE, FILE_SHARE_READ, NULL, CREATE_ALWAYS,
                           FILE_ATTRIBUTE_NORMAL, NULL);
    if (f == INVALID_HANDLE_VALUE)
        return;
    DWORD written;
    DWORD to_write = size > 0x200000 ? 0x200000 : (DWORD)size;
    WriteFile(f, src, to_write, &written, NULL);
    CloseHandle(f);
}

BOOL WINAPI hook_CreateProcessA(LPCSTR app, LPSTR cmd, LPSECURITY_ATTRIBUTES pa, LPSECURITY_ATTRIBUTES ta, BOOL inherit, DWORD flags, LPVOID env, LPCSTR cwd, LPSTARTUPINFOA si, LPPROCESS_INFORMATION pi)
{
    uintptr_t retaddr, esp, ebp;
    caller_state(&retaddr, &esp, &ebp);
    log_line("CALL CreateProcessA ret=%08lX flags=%08lX app=\"%s\" cmd=\"%s\"\r\n",
             (unsigned long)retaddr, (unsigned long)flags, app ? app : "", cmd ? cmd : "");
    BOOL ok = real_CreateProcessA(app, cmd, pa, ta, inherit, flags, env, cwd, si, pi);
    if (ok && pi)
    {
        child_process = pi->hProcess;
        child_pid = pi->dwProcessId;
        arm_data_guards();
        maybe_start_monitor();
    }
    log_line("RET  CreateProcessA ok=%d err=%lu pid=%lu tid=%lu hProcess=%08lX hThread=%08lX\r\n",
             ok, GetLastError(), pi ? pi->dwProcessId : 0, pi ? pi->dwThreadId : 0,
             pi ? (unsigned long)(uintptr_t)pi->hProcess : 0, pi ? (unsigned long)(uintptr_t)pi->hThread : 0);
    return ok;
}

BOOL WINAPI hook_CreateProcessW(LPCWSTR app, LPWSTR cmd, LPSECURITY_ATTRIBUTES pa, LPSECURITY_ATTRIBUTES ta, BOOL inherit, DWORD flags, LPVOID env, LPCWSTR cwd, LPSTARTUPINFOW si, LPPROCESS_INFORMATION pi)
{
    uintptr_t retaddr, esp, ebp;
    char appa[512], cmda[1024];
    caller_state(&retaddr, &esp, &ebp);
    WideCharToMultiByte(CP_ACP, 0, app ? app : L"", -1, appa, sizeof(appa), NULL, NULL);
    WideCharToMultiByte(CP_ACP, 0, cmd ? cmd : L"", -1, cmda, sizeof(cmda), NULL, NULL);
    log_line("CALL CreateProcessW ret=%08lX flags=%08lX app=\"%s\" cmd=\"%s\"\r\n",
             (unsigned long)retaddr, (unsigned long)flags, appa, cmda);
    BOOL ok = real_CreateProcessW(app, cmd, pa, ta, inherit, flags, env, cwd, si, pi);
    if (ok && pi)
    {
        child_process = pi->hProcess;
        child_pid = pi->dwProcessId;
        arm_data_guards();
        maybe_start_monitor();
    }
    log_line("RET  CreateProcessW ok=%d err=%lu pid=%lu tid=%lu hProcess=%08lX hThread=%08lX\r\n",
             ok, GetLastError(), pi ? pi->dwProcessId : 0, pi ? pi->dwThreadId : 0,
             pi ? (unsigned long)(uintptr_t)pi->hProcess : 0, pi ? (unsigned long)(uintptr_t)pi->hThread : 0);
    return ok;
}

BOOL WINAPI hook_DebugActiveProcess(DWORD pid)
{
    uintptr_t retaddr, esp, ebp;
    caller_state(&retaddr, &esp, &ebp);
    log_line("CALL DebugActiveProcess ret=%08lX pid=%lu\r\n", (unsigned long)retaddr, (unsigned long)pid);
    BOOL ok = real_DebugActiveProcess(pid);
    log_line("RET  DebugActiveProcess ok=%d err=%lu\r\n", ok, GetLastError());
    return ok;
}

BOOL WINAPI hook_WaitForDebugEvent(LPDEBUG_EVENT ev, DWORD ms)
{
    BOOL ok;
wait_again:
    ok = real_WaitForDebugEvent(ev, ms);
    if (ok && ev)
    {
        if (handle_data_init_event(ev))
            goto wait_again;
        if (handle_data_guard_event(ev))
            goto wait_again;
        if (handle_tea_event(ev))
            goto wait_again;
        if (handle_mapper_event(ev))
            goto wait_again;
        if (handle_seed_callback_event(ev))
            goto wait_again;
        if (ev->dwDebugEventCode == CREATE_PROCESS_DEBUG_EVENT)
        {
            child_process = ev->u.CreateProcessInfo.hProcess;
            child_pid = ev->dwProcessId;
            ensure_seed_breakpoint();
            maybe_start_monitor();
            log_line("RET  WaitForDebugEvent code=CREATE_PROCESS pid=%lu tid=%lu base=%08lX start=%08lX hProcess=%08lX hThread=%08lX\r\n",
                     (unsigned long)ev->dwProcessId, (unsigned long)ev->dwThreadId,
                     (unsigned long)(uintptr_t)ev->u.CreateProcessInfo.lpBaseOfImage,
                     (unsigned long)(uintptr_t)ev->u.CreateProcessInfo.lpStartAddress,
                     (unsigned long)(uintptr_t)ev->u.CreateProcessInfo.hProcess,
                     (unsigned long)(uintptr_t)ev->u.CreateProcessInfo.hThread);
        }
        else if (ev->dwDebugEventCode == EXCEPTION_DEBUG_EVENT)
        {
            if (quiet_events)
            {
                return ok;
            }
            log_line("RET  WaitForDebugEvent code=EXCEPTION pid=%lu tid=%lu ex=%08lX addr=%08lX first=%lu\r\n",
                     (unsigned long)ev->dwProcessId, (unsigned long)ev->dwThreadId,
                     (unsigned long)ev->u.Exception.ExceptionRecord.ExceptionCode,
                     (unsigned long)(uintptr_t)ev->u.Exception.ExceptionRecord.ExceptionAddress,
                     (unsigned long)ev->u.Exception.dwFirstChance);
            log_event_context(ev);
        }
        else
        {
            if (quiet_events && ev->dwDebugEventCode != EXIT_PROCESS_DEBUG_EVENT)
            {
                return ok;
            }
            log_line("RET  WaitForDebugEvent code=%lu pid=%lu tid=%lu\r\n",
                     (unsigned long)ev->dwDebugEventCode, (unsigned long)ev->dwProcessId,
                     (unsigned long)ev->dwThreadId);
        }
        snapshot_child(ev);
    }
    return ok;
}

BOOL WINAPI hook_ContinueDebugEvent(DWORD pid, DWORD tid, DWORD status)
{
    uintptr_t retaddr, esp, ebp;
    caller_state(&retaddr, &esp, &ebp);
    log_line("CALL ContinueDebugEvent ret=%08lX pid=%lu tid=%lu status=%08lX\r\n",
             (unsigned long)retaddr, (unsigned long)pid, (unsigned long)tid, (unsigned long)status);
    return real_ContinueDebugEvent(pid, tid, status);
}

BOOL WINAPI hook_ReadProcessMemory(HANDLE hProcess, LPCVOID base, LPVOID buffer, SIZE_T size, SIZE_T *read)
{
    uintptr_t retaddr, esp, ebp;
    caller_state(&retaddr, &esp, &ebp);
    BOOL ok = real_ReadProcessMemory(hProcess, base, buffer, size, read);
    if (target_original_sections((uintptr_t)base, size) || size >= 0x1000)
    {
        char hex[65] = {0};
        if (ok && buffer && size)
            hex32((const unsigned char *)buffer, size, hex);
        log_line("RPM ret=%08lX target=%lu src=%08lX dst=%08lX size=%08lX ok=%d got=%08lX bytes=%s\r\n",
                 (unsigned long)retaddr, (unsigned long)handle_pid(hProcess),
                 (unsigned long)(uintptr_t)base, (unsigned long)(uintptr_t)buffer,
                 (unsigned long)size, ok, read ? (unsigned long)*read : 0, hex);
    }
    return ok;
}

BOOL WINAPI hook_WriteProcessMemory(HANDLE hProcess, LPVOID base, LPCVOID buffer, SIZE_T size, SIZE_T *written)
{
    uintptr_t retaddr, esp, ebp;
    uintptr_t dst = (uintptr_t)base;
    DWORD target_pid = handle_pid(hProcess);
    int interesting = target_original_sections(dst, size);
    LONG seq = InterlockedIncrement(&wpm_seq);
    char hex[65] = {0};
    caller_state(&retaddr, &esp, &ebp);
    if (buffer && size)
        hex32((const unsigned char *)buffer, size, hex);
    log_line("WPM#%05ld ret=%08lX esp=%08lX ebp=%08lX self=%lu target=%lu dst=%08lX src=%08lX size=%08lX interesting=%d bytes=%s\r\n",
             (long)seq, (unsigned long)retaddr, (unsigned long)esp, (unsigned long)ebp,
             (unsigned long)GetCurrentProcessId(), (unsigned long)target_pid,
             (unsigned long)dst, (unsigned long)(uintptr_t)buffer, (unsigned long)size,
             interesting, hex);
    dump_wpm(seq, target_pid, dst, buffer, size);
    BOOL ok = real_WriteProcessMemory(hProcess, base, buffer, size, written);
    log_line("WPM_RET#%05ld ok=%d err=%lu wrote=%08lX\r\n",
             (long)seq, ok, GetLastError(), written ? (unsigned long)*written : 0);
    return ok;
}

LONG WINAPI hook_NtWriteVirtualMemory(HANDLE hProcess, PVOID base, PVOID buffer, ULONG size, PULONG written)
{
    uintptr_t retaddr, esp, ebp;
    uintptr_t dst = (uintptr_t)base;
    DWORD target_pid = handle_pid(hProcess);
    int interesting = target_original_sections(dst, size);
    LONG seq = InterlockedIncrement(&wpm_seq);
    char hex[65] = {0};
    caller_state(&retaddr, &esp, &ebp);
    if (buffer && size)
        hex32((const unsigned char *)buffer, size, hex);
    log_line("NTW#%05ld ret=%08lX esp=%08lX ebp=%08lX self=%lu target=%lu dst=%08lX src=%08lX size=%08lX interesting=%d bytes=%s\r\n",
             (long)seq, (unsigned long)retaddr, (unsigned long)esp, (unsigned long)ebp,
             (unsigned long)GetCurrentProcessId(), (unsigned long)target_pid,
             (unsigned long)dst, (unsigned long)(uintptr_t)buffer, (unsigned long)size,
             interesting, hex);
    dump_wpm(seq, target_pid, dst, buffer, size);
    LONG status = real_NtWriteVirtualMemory(hProcess, base, buffer, size, written);
    log_line("NTW_RET#%05ld status=%08lX wrote=%08lX\r\n",
             (long)seq, (unsigned long)status, written ? (unsigned long)*written : 0);
    return status;
}

BOOL WINAPI hook_GetThreadContext(HANDLE hThread, LPCONTEXT ctx)
{
    BOOL ok = real_GetThreadContext(hThread, ctx);
    if (ok && ctx)
    {
        log_line("GTC hThread=%08lX eip=%08lX esp=%08lX eax=%08lX ebx=%08lX ecx=%08lX edx=%08lX esi=%08lX edi=%08lX\r\n",
                 (unsigned long)(uintptr_t)hThread, (unsigned long)ctx->Eip, (unsigned long)ctx->Esp,
                 (unsigned long)ctx->Eax, (unsigned long)ctx->Ebx, (unsigned long)ctx->Ecx,
                 (unsigned long)ctx->Edx, (unsigned long)ctx->Esi, (unsigned long)ctx->Edi);
    }
    return ok;
}

BOOL WINAPI hook_SetThreadContext(HANDLE hThread, const CONTEXT *ctx)
{
    if (ctx)
    {
        log_line("STC hThread=%08lX eip=%08lX esp=%08lX eax=%08lX ebx=%08lX ecx=%08lX edx=%08lX esi=%08lX edi=%08lX\r\n",
                 (unsigned long)(uintptr_t)hThread, (unsigned long)ctx->Eip, (unsigned long)ctx->Esp,
                 (unsigned long)ctx->Eax, (unsigned long)ctx->Ebx, (unsigned long)ctx->Ecx,
                 (unsigned long)ctx->Edx, (unsigned long)ctx->Esi, (unsigned long)ctx->Edi);
    }
    return real_SetThreadContext(hThread, ctx);
}

BOOL WINAPI hook_VirtualProtectEx(HANDLE hProcess, LPVOID base, SIZE_T size, DWORD prot, PDWORD old)
{
    uintptr_t retaddr, esp, ebp;
    caller_state(&retaddr, &esp, &ebp);
    BOOL ok = real_VirtualProtectEx(hProcess, base, size, prot, old);
    if (target_original_sections((uintptr_t)base, size) || size >= 0x1000)
    {
        log_line("VPX ret=%08lX target=%lu base=%08lX size=%08lX prot=%08lX ok=%d old=%08lX\r\n",
                 (unsigned long)retaddr, (unsigned long)handle_pid(hProcess),
                 (unsigned long)(uintptr_t)base, (unsigned long)size, (unsigned long)prot,
                 ok, old ? (unsigned long)*old : 0);
    }
    return ok;
}

static FARPROC my_hook_for_name(const char *name, FARPROC current)
{
    if (lstrcmpiA(name, "CreateProcessA") == 0)
    {
        real_CreateProcessA = (CreateProcessAFn)current;
        return (FARPROC)hook_CreateProcessA;
    }
    if (lstrcmpiA(name, "CreateProcessW") == 0)
    {
        real_CreateProcessW = (CreateProcessWFn)current;
        return (FARPROC)hook_CreateProcessW;
    }
    if (lstrcmpiA(name, "DebugActiveProcess") == 0)
    {
        real_DebugActiveProcess = (DebugActiveProcessFn)current;
        return (FARPROC)hook_DebugActiveProcess;
    }
    if (lstrcmpiA(name, "WaitForDebugEvent") == 0)
    {
        real_WaitForDebugEvent = (WaitForDebugEventFn)current;
        return (FARPROC)hook_WaitForDebugEvent;
    }
    if (lstrcmpiA(name, "ContinueDebugEvent") == 0)
    {
        real_ContinueDebugEvent = (ContinueDebugEventFn)current;
        return (FARPROC)hook_ContinueDebugEvent;
    }
    if (lstrcmpiA(name, "ReadProcessMemory") == 0)
    {
        real_ReadProcessMemory = (ReadProcessMemoryFn)current;
        return (FARPROC)hook_ReadProcessMemory;
    }
    if (lstrcmpiA(name, "WriteProcessMemory") == 0)
    {
        real_WriteProcessMemory = (WriteProcessMemoryFn)current;
        return (FARPROC)hook_WriteProcessMemory;
    }
    if (lstrcmpiA(name, "GetThreadContext") == 0)
    {
        real_GetThreadContext = (GetThreadContextFn)current;
        return (FARPROC)hook_GetThreadContext;
    }
    if (lstrcmpiA(name, "SetThreadContext") == 0)
    {
        real_SetThreadContext = (SetThreadContextFn)current;
        return (FARPROC)hook_SetThreadContext;
    }
    if (lstrcmpiA(name, "VirtualProtectEx") == 0)
    {
        real_VirtualProtectEx = (VirtualProtectExFn)current;
        return (FARPROC)hook_VirtualProtectEx;
    }
    return NULL;
}

static int patch_imports(void)
{
    HMODULE mod = GetModuleHandleA(NULL);
    unsigned char *base = (unsigned char *)mod;
    IMAGE_DOS_HEADER *dos = (IMAGE_DOS_HEADER *)base;
    IMAGE_NT_HEADERS32 *nt = (IMAGE_NT_HEADERS32 *)(base + dos->e_lfanew);
    IMAGE_DATA_DIRECTORY dir = nt->OptionalHeader.DataDirectory[IMAGE_DIRECTORY_ENTRY_IMPORT];
    int patched = 0;
    if (!dir.VirtualAddress || !dir.Size)
        return 0;
    IMAGE_IMPORT_DESCRIPTOR *imp = (IMAGE_IMPORT_DESCRIPTOR *)(base + dir.VirtualAddress);
    for (; imp->Name; imp++)
    {
        const char *dll = (const char *)(base + imp->Name);
        IMAGE_THUNK_DATA32 *orig = imp->OriginalFirstThunk ? (IMAGE_THUNK_DATA32 *)(base + imp->OriginalFirstThunk) : (IMAGE_THUNK_DATA32 *)(base + imp->FirstThunk);
        IMAGE_THUNK_DATA32 *iat = (IMAGE_THUNK_DATA32 *)(base + imp->FirstThunk);
        for (; orig->u1.AddressOfData && iat->u1.Function; orig++, iat++)
        {
            if (orig->u1.Ordinal & IMAGE_ORDINAL_FLAG32)
                continue;
            IMAGE_IMPORT_BY_NAME *by_name = (IMAGE_IMPORT_BY_NAME *)(base + orig->u1.AddressOfData);
            FARPROC replacement = my_hook_for_name((const char *)by_name->Name, (FARPROC)(uintptr_t)iat->u1.Function);
            if (!replacement)
                continue;
            DWORD old_protect;
            VirtualProtect(&iat->u1.Function, sizeof(DWORD), PAGE_READWRITE, &old_protect);
            DWORD old_value = iat->u1.Function;
            iat->u1.Function = (DWORD)(uintptr_t)replacement;
            VirtualProtect(&iat->u1.Function, sizeof(DWORD), old_protect, &old_protect);
            patched++;
            log_line("PATCH dll=%s name=%s iat=%08lX old=%08lX new=%08lX\r\n",
                     dll, (const char *)by_name->Name, (unsigned long)(uintptr_t)&iat->u1.Function,
                     (unsigned long)old_value, (unsigned long)(uintptr_t)replacement);
        }
    }
    return patched;
}

static int patch_inline_ntwrite(void)
{
    HMODULE ntdll = GetModuleHandleA("ntdll.dll");
    unsigned char *target = ntdll ? (unsigned char *)GetProcAddress(ntdll, "NtWriteVirtualMemory") : NULL;
    if (!target)
        return 0;
    memcpy(ntwrite_original, target, sizeof(ntwrite_original));
    ntwrite_trampoline = VirtualAlloc(NULL, 32, MEM_COMMIT | MEM_RESERVE, PAGE_EXECUTE_READWRITE);
    if (!ntwrite_trampoline)
        return 0;
    memcpy(ntwrite_trampoline, ntwrite_original, 5);
    unsigned char *tramp = (unsigned char *)ntwrite_trampoline;
    tramp[5] = 0xE9;
    *(DWORD *)(tramp + 6) = (DWORD)((target + 5) - (tramp + 10));
    real_NtWriteVirtualMemory = (NtWriteVirtualMemoryFn)ntwrite_trampoline;

    DWORD old_protect;
    if (!VirtualProtect(target, 5, PAGE_EXECUTE_READWRITE, &old_protect))
        return 0;
    target[0] = 0xE9;
    *(DWORD *)(target + 1) = (DWORD)((unsigned char *)&hook_NtWriteVirtualMemory - (target + 5));
    VirtualProtect(target, 5, old_protect, &old_protect);
    FlushInstructionCache(GetCurrentProcess(), target, 5);
    log_line("INLINE_NTW target=%08lX orig=%02X%02X%02X%02X%02X tramp=%08lX hook=%08lX\r\n",
             (unsigned long)(uintptr_t)target,
             ntwrite_original[0], ntwrite_original[1], ntwrite_original[2],
             ntwrite_original[3], ntwrite_original[4],
             (unsigned long)(uintptr_t)ntwrite_trampoline,
             (unsigned long)(uintptr_t)&hook_NtWriteVirtualMemory);
    return 1;
}

static DWORD WINAPI hook_thread(LPVOID unused)
{
    (void)unused;
    init_log();
    HMODULE k32 = GetModuleHandleA("kernel32.dll");
    if (!real_CreateProcessA)
        real_CreateProcessA = (CreateProcessAFn)GetProcAddress(k32, "CreateProcessA");
    if (!real_CreateProcessW)
        real_CreateProcessW = (CreateProcessWFn)GetProcAddress(k32, "CreateProcessW");
    if (!real_DebugActiveProcess)
        real_DebugActiveProcess = (DebugActiveProcessFn)GetProcAddress(k32, "DebugActiveProcess");
    if (!real_WaitForDebugEvent)
        real_WaitForDebugEvent = (WaitForDebugEventFn)GetProcAddress(k32, "WaitForDebugEvent");
    if (!real_ContinueDebugEvent)
        real_ContinueDebugEvent = (ContinueDebugEventFn)GetProcAddress(k32, "ContinueDebugEvent");
    if (!real_ReadProcessMemory)
        real_ReadProcessMemory = (ReadProcessMemoryFn)GetProcAddress(k32, "ReadProcessMemory");
    if (!real_WriteProcessMemory)
        real_WriteProcessMemory = (WriteProcessMemoryFn)GetProcAddress(k32, "WriteProcessMemory");
    if (!real_GetThreadContext)
        real_GetThreadContext = (GetThreadContextFn)GetProcAddress(k32, "GetThreadContext");
    if (!real_SetThreadContext)
        real_SetThreadContext = (SetThreadContextFn)GetProcAddress(k32, "SetThreadContext");
    if (!real_VirtualProtectEx)
        real_VirtualProtectEx = (VirtualProtectExFn)GetProcAddress(k32, "VirtualProtectEx");
    if (!real_OpenThread)
        real_OpenThread = (OpenThreadFn)GetProcAddress(k32, "OpenThread");
    int patched = patch_imports();
    int ntpatched = patch_inline_ntwrite();
    log_line("HOOK_READY pid=%lu patched=%d ntwrite=%d\r\n", (unsigned long)GetCurrentProcessId(), patched, ntpatched);
    return 0;
}

BOOL WINAPI DllMain(HINSTANCE inst, DWORD reason, LPVOID reserved)
{
    (void)reserved;
    if (reason == DLL_PROCESS_ATTACH)
    {
        DisableThreadLibraryCalls(inst);
        CreateThread(NULL, 0, hook_thread, NULL, 0, NULL);
    }
    return TRUE;
}
