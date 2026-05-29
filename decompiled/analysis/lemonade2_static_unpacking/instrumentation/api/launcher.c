#ifdef __INTELLISENSE__
#include "../windows/intellisense.h"
#else
#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#endif
#include <stdio.h>

static void close_process_info(PROCESS_INFORMATION *pi) {
    if (pi->hThread) CloseHandle(pi->hThread);
    if (pi->hProcess) CloseHandle(pi->hProcess);
}

int main(int argc, char **argv) {
    const char *exe = argc > 1 ? argv[1] : "C:\\Program Files (x86)\\Lemonade Tycoon 2 - New York City\\Lemonade2.exe";
    const char *dll = argc > 2 ? argv[2] : "C:\\Users\\Admin\\AppData\\Local\\Temp\\hook.dll";
    const char *extra_args = argc > 3 ? argv[3] : "";

    STARTUPINFOA si;
    PROCESS_INFORMATION pi;
    ZeroMemory(&si, sizeof(si));
    ZeroMemory(&pi, sizeof(pi));
    si.cb = sizeof(si);

    char cmd[4096];
    int n = extra_args[0] ? snprintf(cmd, sizeof(cmd), "\"%s\" %s", exe, extra_args) : snprintf(cmd, sizeof(cmd), "\"%s\"", exe);
    if (n < 0 || (size_t)n >= sizeof(cmd)) {
        fprintf(stderr, "command line is too long\n");
        return 1;
    }
    if (!CreateProcessA(exe, cmd, NULL, NULL, FALSE, CREATE_SUSPENDED, NULL, NULL, &si, &pi)) {
        fprintf(stderr, "CreateProcess failed %lu\n", GetLastError());
        return 1;
    }
    fprintf(stdout, "created pid=%lu tid=%lu\n", pi.dwProcessId, pi.dwThreadId);

    SIZE_T len = lstrlenA(dll) + 1;
    void *remote = VirtualAllocEx(pi.hProcess, NULL, len, MEM_COMMIT | MEM_RESERVE, PAGE_READWRITE);
    if (!remote) {
        fprintf(stderr, "VirtualAllocEx failed %lu\n", GetLastError());
        close_process_info(&pi);
        return 1;
    }
    SIZE_T written;
    if (!WriteProcessMemory(pi.hProcess, remote, dll, len, &written)) {
        fprintf(stderr, "WriteProcessMemory path failed %lu\n", GetLastError());
        VirtualFreeEx(pi.hProcess, remote, 0, MEM_RELEASE);
        close_process_info(&pi);
        return 1;
    }
    HMODULE k32 = GetModuleHandleA("kernel32.dll");
    FARPROC load_library = GetProcAddress(k32, "LoadLibraryA");
    if (!load_library) {
        fprintf(stderr, "GetProcAddress LoadLibraryA failed %lu\n", GetLastError());
        VirtualFreeEx(pi.hProcess, remote, 0, MEM_RELEASE);
        close_process_info(&pi);
        return 1;
    }
    HANDLE thread = CreateRemoteThread(pi.hProcess, NULL, 0, (LPTHREAD_START_ROUTINE)load_library, remote, 0, NULL);
    if (!thread) {
        fprintf(stderr, "CreateRemoteThread failed %lu\n", GetLastError());
        VirtualFreeEx(pi.hProcess, remote, 0, MEM_RELEASE);
        close_process_info(&pi);
        return 1;
    }
    DWORD wait = WaitForSingleObject(thread, 10000);
    if (wait != WAIT_OBJECT_0) {
        fprintf(stderr, "LoadLibrary thread wait failed %lu\n", wait);
        CloseHandle(thread);
        VirtualFreeEx(pi.hProcess, remote, 0, MEM_RELEASE);
        close_process_info(&pi);
        return 1;
    }
    DWORD dll_base = 0;
    GetExitCodeThread(thread, &dll_base);
    fprintf(stdout, "injected dll_base=0x%08lX\n", dll_base);
    CloseHandle(thread);
    VirtualFreeEx(pi.hProcess, remote, 0, MEM_RELEASE);

    if (ResumeThread(pi.hThread) == (DWORD)-1) {
        fprintf(stderr, "ResumeThread failed %lu\n", GetLastError());
        close_process_info(&pi);
        return 1;
    }
    fprintf(stdout, "resumed\n");
    close_process_info(&pi);
    return 0;
}
