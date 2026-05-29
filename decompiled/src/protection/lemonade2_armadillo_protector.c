/*
 * Lemonade2.exe Armadillo runtime.
 * Source: decompiled/local/lt2_install/Lemonade2.exe
 * SHA-256: 784BD579465E3971EA6FE600592B9C13B67A1DBF8EB7243162ACDA45E1CCD3C7
 *
 * Status: historical pre-static-unpacking sketch. Use
 * decompiled/analysis/lemonade2_static_unpacking/ and go/tools/lt2normalize for
 * authoritative decrypt/decompress/rebuild logic. This file is retained for
 * packed-loader strings, imports, and broad protector context only; do not treat
 * guessed runtime globals or loader pseudocode below as final game-code recovery.
 *
 * .pdata streams:
 * - 37 KB BMP splash image
 * - 258 KB PE32 protection DLL, image base 0x10000000
 * - 14 KB UTF-16-LE DRM string table
 *
 * Packed EXE image base: 0x00400000.
 *
 * Current map:
 * - 0x10001363 / 0x100014AC: XTEA and key schedule used by code/license data.
 * - 0x10003A73..0x100052F9: Digital River SOAP activation/check/request flows.
 * - 0x10008184..0x100091B4: hardware fingerprint hash sources.
 * - 0x10013B1B..0x100148C6: license storage, Base64, and signed license block I/O.
 * - 0x100145E5: license signature/fingerprint reconciliation.
 * - 0x10019D2B: mapper validation-entry decryptor reached by valid ShortV3 seed.
 * - 0x10006C67: SetFunctionAddresses hook registration for the packed loader.
 */
#include "protection/lemonade2_armadillo_protector.h"
#include "common/win32_recovered_types.h"

#include <stdlib.h>
#include <string.h>

/*
 * DAT_0049c000 area (.data section) - protector runtime state:
 *
 * +0x00  g_original_entry_point    Original OEP (0x00401000 typically)
 * +0x04  g_packed_data_ptr         Pointer to .pdata compressed block
 * +0x08  g_packed_data_size        Size of compressed data
 * +0x0C  g_unpacked_base           VirtualAlloc base for unpacked program
 * +0x10  g_unpacked_size           Total size of unpacked program
 * +0x14  g_crc_table[256]          CRC32 lookup table
 * +0x114 g_license_state           License validation result
 * +0x118 g_hardware_id[16]         Hardware fingerprint hash
 * +0x128 g_install_path[260]       Install directory from registry
 * +0x22C g_user_name[128]          Registered user name
 * +0x2AC g_user_key[32]            License key string
 * +0x2CC g_armadillo_debug_pid           ARMDEBUG target process ID
 * +0x2D0 g_splash_enabled          Non-zero unless ARMSPLASHOFF set
 * +0x2D4 g_crc_fail_count          Number of failed CRC checks
 * +0x2D8 g_debugger_detected       IsDebuggerPresent result
 * +0x2DC g_clock_tamper_count      Back-dating attempts
 */

static byte g_armadillo_config[0x300];

/*
 * ShortV3 keygen provenance.
 *
 * These values come from the decoded Armadillo mapper metadata, not from the
 * restored game code. The public record is embedded in the protected metadata;
 * the private exponent is included here as the reproducible discrete-log
 * recovery from that public record. See go/tools/shortv3derive.
 */
static const u32 armadillo_mapper_window_seed = 0x031e1692;
static const u32 armadillo_mapper_metadata_checksum = 0x7a192edc;
static const u32 armadillo_shortv3_record_id = 0xd074508d;
static const byte armadillo_shortv3_public_record[9] = {
    0x9c, 0xc5, 0x0e, 0x4d, 0x25, 0x41, 0x64, 0x64, 0xb9
};
static const byte armadillo_shortv3_prime_base[9] = {
    0xf3, 0xc7, 0xe0, 0x0a, 0x4b, 0x58, 0x15, 0x52, 0x99
};
static const byte armadillo_shortv3_private_exponent[9] = {
    0x70, 0x30, 0x11, 0x69, 0xde, 0x7c, 0x75, 0xd6, 0x6f
};
static const u32 armadillo_shortv3_valid_seed = 0xccf0580a;

typedef enum ArmadilloSoapOperation {
    ARMADILLO_SOAP_GENERATE_KEY = 0,
    ARMADILLO_SOAP_REISSUE_KEY = 1,
    ARMADILLO_SOAP_GENERATE_KEY_NO_TRIAL = 2,
    ARMADILLO_SOAP_VALIDATE_LICENSE = 3,
    ARMADILLO_SOAP_ACTIVATE_LICENSE = 4,
} ArmadilloSoapOperation;

typedef struct ArmadilloOnlineFunctionMap {
    u32 address;
    const char *name;
    const char *role;
} ArmadilloOnlineFunctionMap;

static const ArmadilloOnlineFunctionMap armadillo_online_function_map[] = {
    {0x10003a73, "Armadillo_BuildActivationUrl",
     "loads BuyURL/KeyURL/CsrURL/DrVersion/RequisitionID/ProductID and appends hardwareSignature"},
    {0x10003b93, "Armadillo_HasActivationConfig",
     "requires BuyURL, KeyURL, CsrURL, unnamed URL/config field, and DrVersion before online flow"},
    {0x10003bbe, "Armadillo_HasRequisitionId",
     "checks whether RequisitionID is configured for no-trial online key generation"},
    {0x100041b0, "Armadillo_ParseActivationUrl",
     "parses http://host[:port]/path into resolved IPv4:port plus request path"},
    {0x100042a4, "Armadillo_BuildSoapRequest",
     "builds HTTP/1.0 SOAP request for generate/reissue/no-trial/validate/activate operations"},
    {0x10004843, "Armadillo_ParseSoapResponse",
     "extracts result, entitlementID, and optional user/key material from SOAP XML"},
    {0x10003ed2, "Armadillo_OnlineSoapTransaction",
     "common socket transaction helper with 50 second timeout and UI message pump"},
    {0x10003bc9, "Armadillo_GenerateOrReissueKeyFlow",
     "REGISTER/key retrieval flow; can show browser/manual fallback and uses operations 0 or 1"},
    {0x10004b74, "Armadillo_GenerateNoTrialKeyFlow",
     "startup serial-to-key flow; uses operation 2 when online flags allow it"},
    {0x10004d95, "Armadillo_PeriodicValidateLicenseFlow",
     "periodic Internet license verification; uses operation 3 and tracks retry/shutdown day state"},
    {0x1000511a, "Armadillo_ActivateLicenseFlow",
     "activation flow; optional prompt then operation 4, returning split registered name/key"},
    {0x1001c417, "Armadillo_RegisterCommandHandler",
     "REGISTER command/UI handler that invokes generate/reissue and stores returned key material"},
    {0x10018876, "Armadillo_StartupNoTrialOnlineCheck",
     "startup license-state handler that invokes generateKeyForNoTrial when configured"},
    {0x1001b720, "Armadillo_ActivateLicenseApi",
     "API/helper wrapper that calls activateLicense and copies returned name/key to caller buffers"},
    {0x10019d2b, "Armadillo_DecryptValidationEntry",
     "ShortV3 seed-driven mapper validation decryptor and complement-trailer check"},
    {0x10014e46, "Armadillo_GetValidationSeed",
     "returns license seed dword xor operation-specific masks 0x19283746/0x91827364"},
};

static const u32 armadillo_validation_entry_seed = 0xccf0580a;
static const u32 armadillo_validation_entry_address = 0x30b;
static const u32 armadillo_validation_entry_size = 0x230;
static const u32 armadillo_validation_entry_repeat_count = 1161;
static const u32 armadillo_validation_entry_md5[4] = {
    0xf37a12b4, 0x98cd3a79, 0xeed7e789, 0x368a604b
};
static const u32 armadillo_validation_entry_trailer_a = 0x5f8779e1;
static const u32 armadillo_validation_entry_trailer_b = 0xa078861e;

extern int  armadillo_inflate_init(void *stream);
extern int  armadillo_inflate(void *stream, int flush);
extern int  armadillo_inflate_end(void *stream);
extern int  armadillo_inflate_reset(void *stream);

extern BOOL  __stdcall IsDebuggerPresent(void);
extern void  __stdcall OutputDebugStringA(LPCSTR str);
extern void  __stdcall OutputDebugStringW(const void *str);
extern HANDLE __stdcall CreateMutexA(void *attrs, BOOL owned, LPCSTR name);
extern HANDLE __stdcall OpenMutexA(DWORD access, BOOL inherit, LPCSTR name);
extern DWORD __stdcall WaitForSingleObject(HANDLE obj, DWORD ms);
extern BOOL  __stdcall ReleaseMutex(HANDLE mutex);
extern HANDLE __stdcall CreateThread(void *attrs, DWORD stack,
    void *(*start)(void *), void *arg, DWORD flags, DWORD *tid);
extern DWORD __stdcall ResumeThread(HANDLE thread);
extern DWORD __stdcall SuspendThread(HANDLE thread);
extern BOOL  __stdcall DebugActiveProcess(DWORD pid);
extern BOOL  __stdcall WaitForDebugEvent(void *event, DWORD ms);
extern BOOL  __stdcall ContinueDebugEvent(DWORD pid, DWORD tid, DWORD status);
extern BOOL  __stdcall ReadProcessMemory(HANDLE proc, void *addr,
    void *buf, DWORD size, DWORD *read);
extern BOOL  __stdcall WriteProcessMemory(HANDLE proc, void *addr,
    const void *buf, DWORD size, DWORD *written);
extern BOOL  __stdcall GetThreadContext(HANDLE thread, void *ctx);
extern BOOL  __stdcall SetThreadContext(HANDLE thread, const void *ctx);
extern BOOL  __stdcall VirtualProtect(void *addr, DWORD size,
    DWORD new_prot, DWORD *old_prot);
extern BOOL  __stdcall VirtualProtectEx(HANDLE proc, void *addr,
    DWORD size, DWORD new_prot, DWORD *old_prot);
extern void * __stdcall VirtualAlloc(void *addr, DWORD size,
    DWORD type, DWORD prot);
extern BOOL  __stdcall VirtualFree(void *addr, DWORD size, DWORD type);
extern HANDLE __stdcall GetCurrentProcess(void);
extern DWORD __stdcall GetCurrentProcessId(void);
extern DWORD __stdcall GetCurrentThreadId(void);
extern HANDLE __stdcall GetCurrentThread(void);
extern BOOL  __stdcall GetVersionExA(void *info);
extern DWORD __stdcall GetTickCount(void);
extern void  __stdcall GetLocalTime(void *systime);
extern void  __stdcall GetSystemTime(void *systime);
extern DWORD __stdcall GetTimeZoneInformation(void *tzinfo);
extern DWORD __stdcall GetEnvironmentVariableA(LPCSTR name,
    LPSTR buf, DWORD size);
extern DWORD __stdcall GetModuleFileNameA(HMODULE mod,
    LPSTR buf, DWORD size);
extern DWORD __stdcall GetModuleFileNameW(HMODULE mod,
    void *buf, DWORD size);
extern DWORD __stdcall GetShortPathNameA(LPCSTR long_path,
    LPSTR short_path, DWORD size);
extern char * __stdcall GetCommandLineA(void);
extern void * __stdcall GetCommandLineW(void);
extern void  __stdcall GetStartupInfoA(void *info);
extern void  __stdcall GetStartupInfoW(void *info);
extern BOOL  __stdcall CreateProcessA(LPCSTR app, LPSTR cmdline,
    void *proc_attrs, void *thread_attrs, BOOL inherit,
    DWORD flags, void *env, LPCSTR dir, void *si, void *pi);
extern BOOL  __stdcall CreateProcessW(const void *app, void *cmdline,
    void *proc_attrs, void *thread_attrs, BOOL inherit,
    DWORD flags, void *env, const void *dir, void *si, void *pi);
extern void  __stdcall ExitProcess(UINT code);
extern BOOL  __stdcall TerminateProcess(HANDLE proc, UINT code);
extern DWORD __stdcall GetExitCodeProcess(HANDLE proc, DWORD *code);
extern BOOL  __stdcall SetEnvironmentVariableA(LPCSTR name, LPCSTR value);
extern void  __stdcall Sleep(DWORD ms);
extern void  __stdcall SetThreadPriority(HANDLE thread, int priority);
extern DWORD __stdcall GetFileSize(HANDLE file, DWORD *size_hi);
extern HANDLE __stdcall CreateFileA(LPCSTR name, DWORD access,
    DWORD share, void *attrs, DWORD creation, DWORD flags, HANDLE tmpl);
extern BOOL  __stdcall ReadFile(HANDLE file, void *buf,
    DWORD size, DWORD *read, void *overlapped);
extern BOOL  __stdcall CloseHandle(HANDLE obj);
extern BOOL  __stdcall DuplicateHandle(HANDLE src_proc, HANDLE src,
    HANDLE dst_proc, HANDLE *dst, DWORD access, BOOL inherit, DWORD opts);
extern HANDLE __stdcall CreateFileMappingA(HANDLE file, void *attrs,
    DWORD prot, DWORD size_hi, DWORD size_lo, LPCSTR name);
extern void * __stdcall MapViewOfFile(HANDLE mapping, DWORD access,
    DWORD offset_hi, DWORD offset_lo, DWORD size);
extern DWORD __stdcall GetLastError(void);
extern void  __stdcall SetLastError(DWORD err);

extern HANDLE __stdcall CreateWindowExA(DWORD ex_style, LPCSTR class,
    LPCSTR title, DWORD style, int x, int y, int w, int h,
    HANDLE parent, HANDLE menu, HANDLE instance, void *param);
extern BOOL  __stdcall DestroyWindow(HANDLE hwnd);
extern BOOL  __stdcall ShowWindow(HANDLE hwnd, int cmd);
extern BOOL  __stdcall UpdateWindow(HANDLE hwnd);
extern BOOL  __stdcall PostMessageA(HANDLE hwnd, UINT msg,
    WPARAM wp, LPARAM lp);
extern HANDLE __stdcall FindWindowA(LPCSTR class, LPCSTR title);
extern ATOM  __stdcall RegisterClassA(const void *wc);
extern int   __stdcall MessageBoxA(HANDLE hwnd, LPCSTR text,
    LPCSTR caption, UINT type);
extern HANDLE __stdcall LoadCursorA(HANDLE inst, LPCSTR name);
extern LRESULT __stdcall DefWindowProcA(HANDLE hwnd, UINT msg,
    WPARAM wp, LPARAM lp);
extern BOOL  __stdcall PeekMessageA(void *msg, HANDLE hwnd,
    UINT filter_min, UINT filter_max, UINT remove);
extern BOOL  __stdcall TranslateMessage(const void *msg);
extern LRESULT __stdcall DispatchMessageA(const void *msg);
extern BOOL  __stdcall GetMessageA(void *msg, HANDLE hwnd,
    UINT filter_min, UINT filter_max);
extern BOOL  __stdcall SetTimer(HANDLE hwnd, UINT_PTR id,
    UINT ms, void *callback);
extern BOOL  __stdcall KillTimer(HANDLE hwnd, UINT_PTR id);
extern int   __stdcall GetSystemMetrics(int index);
extern SHORT __stdcall GetAsyncKeyState(int vk);
extern HANDLE __stdcall GetDlgItem(HANDLE dlg, int id);

extern LONG __stdcall RegOpenKeyExA(HANDLE key, LPCSTR sub,
    DWORD opts, DWORD access, HANDLE *result);
extern LONG __stdcall RegQueryValueExA(HANDLE key, LPCSTR name,
    DWORD *reserved, DWORD *type, BYTE *data, DWORD *size);
extern LONG __stdcall RegSetValueExA(HANDLE key, LPCSTR name,
    DWORD reserved, DWORD type, const BYTE *data, DWORD size);
extern LONG __stdcall RegCloseKey(HANDLE key);

extern HANDLE __stdcall FindFirstFileA(LPCSTR pattern, void *data);
extern HANDLE __stdcall FindFirstFileW(const void *pattern, void *data);
extern BOOL  __stdcall FindClose(HANDLE find);

extern void * __cdecl malloc(size_t size);
extern void   __cdecl free(void *ptr);
extern int   __cdecl strcmp(const char *a, const char *b);
extern int   __cdecl strncmp(const char *a, const char *b, size_t n);
extern char * __cdecl strstr(const char *haystack, const char *needle);

/* Unpacker error messages from .data1 string table at 0x004e3000. */

static const char *const armadillo_error_failed_crc =
    "Failed CRC check";
static const char *const armadillo_error_relocations =
    "Relocations error";
static const char *const armadillo_error_execute_target =
    "Failed to execute target process";
static const char *const armadillo_error_dll_init =
    "DLL initialization failed";
static const char *const armadillo_error_allocations =
    "Cannot set allocations";
static const char *const armadillo_error_find_import =
    "Cannot find import; DLL may be missing, corrupt, or wrong version";
static const char *const armadillo_error_alloc_dll =
    "Cannot allocate memory for DLL";
static const char *const armadillo_error_locate_data =
    "Cannot locate protected program data";
static const char *const armadillo_error_general =
    "General extraction error";
static const char *const armadillo_error_memory =
    "Insufficient memory!";

static const char *const armadillo_loading_text = "Loading...";
static const char *const armadillo_loading_class = "ArBase Bitmap Window";
static const char *const armadillo_test_class = "ArBase Test Bitmap Window";
static const char *const armadillo_main_class = "MainClass";
static const char *const armadillo_font_face = "MS Sans Serif";

/* DRM dialog strings from .pdata stream 3. */

static const char *const armadillo_drm_clock_backdating =
    "Your system clock appears to have been set back, possibly in an "
    "attempt to defeat the security system on this program. Please "
    "correct your system clock before trying to run this program again. "
    "If your clock is correct, please contact the author of this program "
    "for instructions on correcting this error (report code %1).";

static const char *const armadillo_drm_clock_changed =
    "Your system clock appears to have been changed, possibly in an "
    "attempt to defeat the security system on this program. Please "
    "correct your system clock before trying to run this program again. "
    "If your clock is already correct, rebooting the system may fix "
    "this problem, otherwise contact the author of this program for "
    "instructions (report code CCB-A).";

static const char *const armadillo_drm_debugger_active =
    "For security purposes, this program will not run while system "
    "debuggers are active. Please remove or disable the system "
    "debugger before trying to run this program again.";

static const char *const armadillo_drm_temporary_days =
    "You have %1 day(s) left on your temporary key.";

static const char *const armadillo_drm_temporary_uses =
    "You have %1 use(s) left (after this one) on your temporary key.";

static const char *const armadillo_drm_temporary_both =
    "You have %1 day(s) or %2 use(s) left on your temporary key.";

static const char *const armadillo_drm_key_expired =
    "Key Expired";

static const char *const armadillo_drm_key_expired_full =
    "This key is expired. Please contact the author of this program "
    "for a new one.";

static const char *const armadillo_drm_invalid_fixclock =
    "That is not a valid FixClock key. Please try again.";

static const char *const armadillo_drm_enter_fixclock =
    "Make sure your computer's date and time are correct, then enter "
    "the FixClock key below, exactly as given to you.";

static const char *const armadillo_drm_fixclock_error =
    "There was an error correcting your system. Please reboot and try "
    "running FixClock again. If you have already done this, please "
    "contact the author or the Silicon Realms Toolworks for assistance.";

static const char *const armadillo_drm_system_corrected =
    "Your system has been corrected.";

static const char *const armadillo_drm_no_fix_needed =
    "Your system does not require correcting.";

static const char *const armadillo_drm_no_key =
    "This program requires a security key. If you have one, select OK "
    "to enter it. After entering a valid key, you will not be prompted "
    "again.";

static const char *const armadillo_drm_key_not_valid_for_version =
    "The security key for this program currently stored on your system "
    "does not appear to be valid for this version of the program. "
    "Select Yes to enter a new key, or No to revert to the default "
    "setting (if any).";

static const char *const armadillo_drm_no_registration_key =
    "This program does not have a registration key installed, or the "
    "registration information is restricted.";

static const char *const armadillo_drm_enter_name_key =
    "Enter the registration name and key below, exactly as given to you.";

static const char *const armadillo_drm_name_key_invalid =
    "The name/key you entered does not appear to be valid. Please try "
    "again.";

static const char *const armadillo_drm_key_is_expired =
    "The key you entered is already expired. Please enter a new key.";

static const char *const armadillo_drm_key_valid_stored =
    "Key is valid, and has been stored.";

static const char *const armadillo_drm_key_valid =
    "Key Valid";

static const char *const armadillo_drm_must_enter_name_key =
    "You must enter a name and a key.";

static const char *const armadillo_drm_upgrade_key =
    "The key you entered is an upgrade key. You must have a valid key "
    "already on the system before installing it. Please enter your "
    "original key, then try this one again. If you don't have your "
    "original key, or it does not work, please contact the program's "
    "author for assistance.";

static const char *const armadillo_drm_registered_to =
    "This program is registered to:\n\n%1\n%2";

static const char *const armadillo_drm_hardware_fingerprint =
    "Hardware fingerprint: %1";

static const char *const armadillo_drm_transfer_warning =
    "WARNING: If you complete this operation, this program will STOP "
    "WORKING on this computer. If you make a mistake, you will have to "
    "contact the author of this program for assistance.\n\n"
    "Select the target computer and call up the code-entry window. You "
    "will need the hardware-locking code from it.\n\n"
    "Are you SURE you wish to do this?";

static const char *const armadillo_drm_transfer_enter_hwcode =
    "You must enter the hardware code from the machine you plan to "
    "transfer the program to.";

static const char *const armadillo_drm_transfer_confirm =
    "Please confirm: %1 is correct?";

static const char *const armadillo_drm_transfer_info =
    "Name: %1\nCode: %2\n\nThis code is good ONLY for the machine "
    "specified by the hardware code %3. This program will cease to run "
    "on this machine.\n\nYou will be shown this message %4 more "
    "time(s), as a precaution.";

static const char *const armadillo_drm_lock_transferred =
    "Lock Transferred!";

static const char *const armadillo_drm_hardware_locking_code =
    "Hardware-Locking code for target computer:";

static const char *const armadillo_drm_too_many_copies =
    "You have too many copies of this program already running on your "
    "system or network. You are licensed for only %1 copy/copies at "
    "a time.";

static const char *const armadillo_drm_buy_now =
    "Buy Now!";

static const char *const armadillo_drm_enter_password =
    "This program requires a password. Enter it now:";

static const char *const armadillo_drm_password_incorrect =
    "Password is incorrect. Try again.";

static const char *const armadillo_drm_key_required =
    "Key Required";

static const char *const armadillo_drm_unregister_warning =
    "If you continue, the program key will be permanently removed from "
    "your system, and you will not be able to reinstall it.";

static const char *const armadillo_drm_unregister_confirm =
    "Program key removed. Your confirmation code is shown below. Please "
    "COPY THIS DOWN -- you will need it to confirm that you've removed "
    "the program. Press all three of the numbered buttons to confirm "
    "that you've copied it down, then press OK.";

static const char *const armadillo_drm_unregister_program =
    "Unregister Program";

static const char *const armadillo_drm_restart_system =
    "To complete the installation of this program, you must restart the "
    "system. Would you like to restart it now?";

static const char *const armadillo_drm_server_running =
    "Server is running a different version. Please upgrade to server's "
    "version.";

static const char *const armadillo_drm_server_already_running =
    "Server already running on system %1, startup cancelled";

static const char *const armadillo_drm_server_started =
    "Server started";

static const char *const armadillo_drm_copies_found =
    "%1 copies found already running on network";

static const char *const armadillo_drm_copies_running =
    "Copies running/allowed";

static const char *const armadillo_drm_shutdown =
    "Shutdown";

static const char *const armadillo_drm_access_denied_version =
    "Access request from client %1 denied: using different version";

static const char *const armadillo_drm_access_granted =
    "Access request from client %1 granted";

static const char *const armadillo_drm_shutdown_notification =
    "Shutdown notification received from client %1";

static const char *const armadillo_drm_discrepancy =
    "Discrepancy: expected %1 copies, found %2 instead";

static const char *const armadillo_drm_server_license_expired =
    "Server license expired. All further client access denied.";

static const char *const armadillo_drm_cannot_write_key =
    "Cannot write key information.";

static const char *const armadillo_drm_damaged =
    "This program has been damaged, possibly by a bad sector of the "
    "hard drive or a virus. Please reinstall it.";

static const char *const armadillo_drm_admin_required =
    "Please run this program from the Administrator account so it can "
    "set up your license. Once the license is set up, you can run it "
    "from any account.";

static const char *const armadillo_drm_unpack_error =
    "Error while unpacking program, code %1. Please report to author.";

static const char *const armadillo_drm_server_log_error =
    "Cannot open ServerLog, logfile will not be written";

static const char *const armadillo_drm_web_browser_error =
    "There was an error starting your web viewer to display the web "
    "site \"%1\". Your system may not be properly configured for web "
    "access.";

static const char *const armadillo_drm_internet_verify_ask =
    "This program must periodically contact an Internet server to "
    "verify your license. You can continue to use the program for a "
    "short time even if you don't allow the program to contact the "
    "Internet, but it will eventually stop working if it cannot contact "
    "the server. May it contact the server now?";

static const char *const armadillo_drm_internet_verified =
    "Your license has been verified.";

static const char *const armadillo_drm_internet_error =
    "There was an error contacting the license server (%1). The program "
    "will try again the next time you start it. You can continue to "
    "use it for now, but it must contact the license server soon or it "
    "will shut down.";

static const char *const armadillo_drm_violation =
    "You have violated the licensing terms for this program, or there "
    "was an error that made it appear that you have. Please contact the "
    "author of this program for instructions. Error code %1.";

static const char *const armadillo_drm_internet_serial =
    "This program must contact an Internet server to verify your serial "
    "number and obtain a key. May it contact the server now?";

static const char *const armadillo_drm_enter_serial =
    "Enter your serial number below, exactly as given to you, and press "
    "the OK button.";

static const char *const armadillo_drm_serial_invalid =
    "The serial number you entered was not recognized, or there was an "
    "error contacting the server. Try again later, or enter a key "
    "(which you can receive from Customer Service) manually.";

static const char *const armadillo_drm_connections_denied =
    "Connection request from %1 denied: no more uses remaining";

static const char *const armadillo_drm_cannot_open_port =
    "Cannot open a communications port. Shutting down.";

static const char *const armadillo_drm_cannot_open_locator =
    "Cannot open a locator port. Shutting down.";

static const char *const armadillo_drm_cannot_find_server =
    "Cannot find the server. Please check with your systems "
    "administrator to ensure that it is running, then try again.";

static const char *const armadillo_drm_server_transfer_blocked =
    "A server version of this program is already running. Please shut "
    "it down before attempting to transfer the license.";

static const char *const armadillo_drm_window_close_timer =
    "This window will close in %1 seconds.";

static const char *const armadillo_drm_tok_enable_timer =
    "OK button will be enabled in %1 seconds...";

/*
 * XTEA cipher, DLL 0x10001363. 32 rounds, delta 0x9E3779B9.
 * SetCipherKey at 0x100014ac rotates 32-bit master key 0xF1C62847 into
 * four subkeys. The length argument is bytes; the DLL processes complete
 * 64-bit blocks only.
 */
static void armadillo_xtea_cipher(u32 key[4], u32 *data, u32 byte_count, int encrypt)
{
    u32 k0 = key[0];
    u32 k1 = key[1];
    u32 k2 = key[2];
    u32 k3 = key[3];
    u32 *end = data + ((byte_count >> 2) & ~1u);

    if (encrypt < 1) {
        while (data < end) {
            u32 v0 = data[0];
            u32 v1 = data[1];
            u32 sum = 0xC6EF3720;  /* delta * 32 */
            u32 i;

            for (i = 0; i < 32; i++) {
                v1 -= ((v0 >> 5) + k3) ^ (v0 * 16 + k2) ^ (sum + v0);
                sum += 0x61C88647;  /* -delta */
                v0 -= ((v1 >> 5) + k1) ^ (v1 * 16 + k0) ^ (sum + v1);
            }
            data[0] = v0;
            data[1] = v1;
            data += 2;
            if (encrypt < 0) {
                k0 = v0;
                k1 = v1;  /* CBC-like chaining in negative mode */
            }
        }
    } else {
        while (data < end) {
            u32 v0 = data[0];
            u32 v1 = data[1];
            u32 plain0 = v0;
            u32 plain1 = v1;
            u32 sum = 0;
            u32 i;

            for (i = 0; i < 32; i++) {
                sum -= 0x61C88647;  /* += delta */
                v0 += ((v1 >> 5) + k1) ^ (v1 * 16 + k0) ^ (sum + v1);
                v1 += ((v0 >> 5) + k3) ^ (v0 * 16 + k2) ^ (sum + v0);
            }
            data[0] = v0;
            data[1] = v1;
            data += 2;
            if (encrypt > 1) {
                k0 = plain0;
                k1 = plain1;  /* Chaining uses the original plaintext block. */
            }
        }
    }
}

/* DLL 0x100014ac: Armadillo_SetCipherKey. */
static void armadillo_set_xtea_key(u32 master_key, u32 *data, u32 byte_count, int mode)
{
    u32 key[4];

    key[0] = master_key;
    key[1] = (master_key << 24) | (master_key >> 8);
    key[2] = (master_key >> 8) << 24 | (key[1] >> 8);
    key[3] = (key[1] >> 8) << 24 | (key[2] >> 8);

    armadillo_xtea_cipher(key, data, byte_count, mode);
}

/* DLL 0x10001dcc. */
static u32 armadillo_ror(u32 value, int shift)
{
    u32 count;

    if (shift < 0)
        shift += 32;
    count = (u32)shift & 0x1F;
    return (value << ((32 - count) & 0x1F)) | (value >> count);
}

static u32 armadillo_crc32_table[256];

static void armadillo_crc32_init(void)
{
    u32 i, j;
    for (i = 0; i < 256; i++) {
        u32 crc = i;
        for (j = 0; j < 8; j++) {
            if (crc & 1)
                crc = (crc >> 1) ^ 0xEDB88320;
            else
                crc >>= 1;
        }
        armadillo_crc32_table[i] = crc;
    }
}

static u32 armadillo_crc32_compute(const byte *data, u32 len, u32 seed)
{
    u32 crc = seed ^ 0xFFFFFFFF;
    u32 i;

    for (i = 0; i < len; i++) {
        crc = armadillo_crc32_table[(crc ^ data[i]) & 0xFF] ^ (crc >> 8);
    }
    return crc ^ 0xFFFFFFFF;
}

static u32 armadillo_hash_combine(const byte *data, u32 len, u32 seed);

/*
 * Armadillo_GenerateHardwareFingerprint (DLL at 0x100083ef)
 *
 * Reads BIOS version and date from:
 *   HKLM\Hardware\Description\System\SystemBiosVersion  (REG_SZ)
 *   HKLM\Hardware\Description\System\SystemBiosDate     (REG_SZ)
 */

static u32 armadillo_gen_bios_fingerprint(u32 seed, BOOL alt_mode)
{
    HKEY  key;
    u32   hash = seed;
    byte  buf[256];
    DWORD buf_size;

    if (!alt_mode) {
        if (RegOpenKeyExA(HKEY_LOCAL_MACHINE,
                "Hardware\\Description\\System", 0, KEY_QUERY_VALUE, &key) == 0) {

            buf_size = sizeof(buf);
            if (RegQueryValueExA(key, "SystemBiosVersion",
                    NULL, NULL, buf, &buf_size) == 0) {
                hash = armadillo_hash_combine(buf, buf_size, hash);
            }

            buf_size = sizeof(buf);
            if (RegQueryValueExA(key, "SystemBiosDate",
                    NULL, NULL, buf, &buf_size) == 0) {
                hash = armadillo_hash_combine(buf, buf_size, hash);
            }

            RegCloseKey(key);
        }
    }

    return hash;
}

/* Armadillo_GetComputerNameHash, DLL 0x10008540. */
static u32 armadillo_get_computer_name_hash(u32 seed)
{
    byte  name[16];
    DWORD size = sizeof(name);

    if (GetComputerNameA((char *)name, &size)) {
        seed = armadillo_hash_combine(name, /*strlen*/ (u32)size, seed);
    }
    return seed;
}

/* Armadillo_GetVolumeSerialHash, DLL 0x100085bf. */
static u32 armadillo_get_volume_serial_hash(u32 seed)
{
    DWORD vol_serial;
    DWORD max_len;
    DWORD flags;
    byte  fs_name[16];
    byte  vol_name_buf[16];
    byte  serial_str[16];

    if (GetVolumeInformationA("C:\\",
            (char *)vol_name_buf, sizeof(vol_name_buf),
            &vol_serial, &max_len, &flags,
            (char *)fs_name, sizeof(fs_name))) {
        /* sprintf("%08X", vol_serial) -- standard hex formatting. */
        u32 len = 0;
        seed ^= armadillo_hash_combine(serial_str, len, 0xFFFFFFFF);
    }
    return seed;
}

/*
 * Armadillo_BuildCompositeFingerprint (DLL at 0x10008184)
 *
 * Combines entropy sources based on a bitmask and returns one fingerprint
 * element. Callers build the 31-element license fingerprint arrays by invoking
 * this function for bitmasks 1 << 0 through 1 << 30.
 *
 *   Bit 0: Armadillo_GetSystemEntropy (timestamp, process counters, CPU count)
 *   Bit 1: armadillo_gen_bios_fingerprint (BIOS version + date)
 *   Bit 2: armadillo_get_computer_name_hash
 *   Bit 3: armadillo_get_volume_serial_hash
 *   Bits 4-7: Additional entropy sources
 *
 * Used by Armadillo_LicenseSignatureVerify to construct RSA key components for
 * license binding.
 */
static u32 armadillo_build_composite_fingerprint(u32 bitmask)
{
    u32 hash = 0xFFFFFFFF;

    if (bitmask & 1)
        hash = /*Armadillo_GetSystemEntropy*/ armadillo_hash_combine(NULL, 0, hash);

    if (bitmask & 2)
        hash = armadillo_gen_bios_fingerprint(hash, TRUE);

    if (bitmask & 4)
        hash = armadillo_get_computer_name_hash(hash);

    if (bitmask & 8)
        hash = armadillo_get_volume_serial_hash(hash);

    /* Bits 4-7: additional sources (CPU info, network adapters, etc.)
     *   bit 4 (0x10): FUN_1000865c -- additional source with direction
     *   bit 5 (0x20): FUN_100087b0 -- additional source with direction
     *   bit 6 (0x40): FUN_10008d8f -- additional source
     *   bit 7 (0x80): FUN_100091b4 -- additional source
     *   bit 30 (0x40000000): FUN_1000865c with direction false
     */

    return hash;
}

/* Stand-in for FUN_10006c3b, API entry #7 at DAT_10038114. */
static u32 armadillo_hash_combine(const byte *data, u32 len, u32 seed)
{
    u32 hash = seed;
    u32 i;
    for (i = 0; i < len; i++) {
        hash = (hash << 5) + hash + data[i];  /* djb2-like hash */
    }
    return hash;
}

/*
 * Armadillo_ReadLicenseStorage (DLL at 0x10013e0d)
 *
 * Reads license data from the appropriate storage location based on
 * operation type:
 *
 *   op 5, 6, 0xB, 0xC:  HKLM\Software\Licenses\<name>
 *   op 7, 0xD:          HKLM\<product subkey>  (REG_SZ, Base64 decoded)
 *   op 8, 0xE:          %TEMP%\<product>.key   (file read)
 *   op 9, 0xF:          HKCU\Software\The Silicon Realms Toolworks\...
 *   op 10, 0x10:        HKCU\<product subkey>  (REG_BINARY, raw)
 *
 * When reading REG_SZ, the data is Base64-decoded via Armadillo_Base64Codec
 * (DLL at 0x10013b1b) and optionally XTEA-decrypted.
 *
 * Uses CreateMutexA for serialization to prevent concurrent access.
 */

/*
 * Armadillo_Base64Codec (DLL at 0x10013b1b)
 *
 * Standard Base64 encode/decode:
 *   param_4 == 0: Decode Base64 -> binary
 *   param_4 == 1: Encode binary -> Base64
 *
 * Uses the classic Base64 character set (A-Za-z0-9+/), offset - 0x40
 * for decode and + 0x40 for encode.
 */

/*
 * Armadillo_LicenseSignatureVerify (DLL at 0x100145e5)
 *
 * Verifies the RSA digital signature on the license data block.
 * The license block (read from HKLM\Software\Licenses\<magic_id>
 * where magic_id is "K7C0DB872A3F777C0") contains:
 *   - A 31-word composite fingerprint (hardware ID)
 *   - License data fields (name, key, expiry, etc.)
 *   - RSA signature over the above
 *
 * The signature verification uses the 31-word fingerprint as RSA key
 * components.  If the signature verifies, the license is valid for
 * this specific machine.  If the hardware has changed, the fingerprint
 * won't match and verification fails.
 *
 * Magic constants:
 *   License block identifier:  K7C0DB872A3F777C0
 *   Block header size:         0x900 (2304 bytes)
 *   RSA key:                   31 x 32-bit words
 */

/*
 * Online activation/check subsystem map.
 *
 * 0x10003A73 Armadillo_BuildActivationUrl
 *   Clears a 0x1E00-byte request context, reads `BuyURL`, `KeyURL`, `CsrURL`,
 *   an unnamed field, `DrVersion`, `RequisitionID`, and `ProductID` from the
 *   Armadillo configuration object, then appends:
 *
 *     &hardwareSignature=%08X-%04u-%02u-%02u-%02u-%02u-%02u-%04u
 *
 *   to BuyURL when BuyURL is present. The timestamp is UTC from GetSystemTime.
 *   0x10003B93 requires BuyURL, KeyURL, CsrURL, the unnamed field, and
 *   DrVersion before the online flows are allowed to continue. 0x10003BBE
 *   separately checks RequisitionID for the no-trial key path.
 *
 * 0x100041B0 Armadillo_ParseActivationUrl
 *   Accepts `http://host[:port]/path`, strips the scheme, separates path at the
 *   first slash, parses optional port (default 80), resolves the host with the
 *   Winsock gethostbyname ordinal, and stores `a.b.c.d:port` plus the request
 *   path into the transaction context.
 *
 * 0x100042A4 Armadillo_BuildSoapRequest
 *   Builds an HTTP/1.0 POST with SOAP 1.0 XML body. Operation selector:
 *     0 generateKey
 *     1 reissueKey
 *     2 generateKeyForNoTrial
 *     3 validateLicense
 *     4 activateLicense
 *
 *   Common fields include downloadID, standardHardwareID, enhancedHardwareID,
 *   armVersion, drVersion, hardwareSignature, requisitionID, and serialNumber.
 *   Operation-specific fields include reqID/password, entitlementID/key,
 *   productID/userName, depending on the operation. The HTTP Host header string
 *   is fixed as `localhost`; the actual socket target comes from the parsed
 *   configured URL.
 *
 * 0x10003ED2 Armadillo_OnlineSoapTransaction
 *   Common transport helper. It parses the URL, builds the SOAP request, opens a
 *   socket via Armadillo wrappers, writes the request, pumps the UI message loop,
 *   and reads until the response contains the expected terminator or 50 seconds
 *   elapse. It calls 0x10004843 to parse the SOAP result.
 *
 * 0x10004843 Armadillo_ParseSoapResponse
 *   Extracts `<result>`. Result 0 is success. On success it optionally extracts
 *   `<entitlementID>` and, for key-generation operations, `<userName>` plus an
 *   adjacent `<key>` field and formats them as `name\r\nkey`. Missing XML
 *   fields return -1 or -2; non-zero result values are propagated to the caller.
 *
 * Trigger wrappers:
 *   0x10003BC9 Generate/reissue key dialog path. Called by command/UI handler
 *              0x1001C417 and recursively after browser/manual fallback.
 *   0x10004B74 GenerateKeyForNoTrial serial-to-key path. Called by 0x10018876
 *              during startup/license-state handling when online flags are set
 *              and local state is not already marked complete.
 *   0x10004D95 Periodic validateLicense path. Checks clock/verification-day
 *              state, optionally prompts the user, sends operation 3, stores the
 *              returned status text, and enforces a later shutdown deadline if
 *              verification keeps failing. No direct static caller is present in
 *              this DLL image; keep it mapped as an available Armadillo routine,
 *              not as a proven always-run launch check.
 *   0x1000511A activateLicense path. Called by 0x1001B720, optionally prompts
 *              the user, sends operation 4, and splits `name\r\nkey` output.
 */

static void armadillo_generate_hardware_id(byte result[16])
{
    u32 hash = 0x811C9DC5;  /* FNV-1a offset basis */

    {
        byte    vol_name[260];
        DWORD   vol_serial;
        byte    sys_dir[260];
        DWORD   computer_name_len = 64;
        byte    computer_name[64];
        DWORD   i;

        vol_serial = 0;  /* GetVolumeInformationA("C:\\", ...) */
        { DWORD len = 260; (void)len; }

        for (i = 0; i < sizeof(vol_serial); i++) {
            hash ^= ((byte *)&vol_serial)[i];
            hash *= 0x01000193;
        }
        for (i = 0; i < sizeof(computer_name) && i < computer_name_len; i++) {
            hash ^= computer_name[i];
            hash *= 0x01000193;
        }
    }

    result[0]  = (byte)(hash >> 24);
    result[1]  = (byte)(hash >> 16);
    result[2]  = (byte)(hash >> 8);
    result[3]  = (byte)(hash);
    result[4]  = (byte)((hash * 0x01000193) >> 24);
    result[5]  = (byte)((hash * 0x01000193) >> 16);
    result[6]  = (byte)((hash * 0x01000193) >> 8);
    result[7]  = (byte)((hash * 0x01000193));
    hash *= 0x01000193;
    result[8]  = (byte)(hash >> 24);
    result[9]  = (byte)(hash >> 16);
    result[10] = (byte)(hash >> 8);
    result[11] = (byte)(hash);
    hash *= 0x01000193;
    result[12] = (byte)(hash >> 24);
    result[13] = (byte)(hash >> 16);
    result[14] = (byte)(hash >> 8);
    result[15] = (byte)(hash);
}

/*
 * Armadillo stores license information in:
 *
 *   HKCU\Software\<Vendor>\<Product>\Key
 *     - Encrypted license key block (name + key + hardware ID + expiry)
 *
 *   HKCU\Software\<Vendor>\<Product>\Certificate
 *     - Digital certificate authenticating the license
 *
 *   <InstallDir>\<product>.key
 *     - Alternative file-based license storage
 *
 * The license block is typically RSA-1024 signed to prevent tampering.
 * The public key is embedded in the protector stub.
 */

static BOOL armadillo_check_license(void)
{
    HANDLE key_handle;
    LONG   result;
    byte   lic_data[1024];
    DWORD  lic_size = sizeof(lic_data);
    DWORD  lic_type  = 0;

    result = RegOpenKeyExA(
        NULL,  /* HKEY_CURRENT_USER (NULL = 0x80000001 handled by helper) */
        "Software\\Jamdat\\Lemonade Tycoon 2",
        0, 1,  /* KEY_QUERY_VALUE */
        &key_handle);

    if (result == 0) {
        result = RegQueryValueExA(
            key_handle, "Key",
            NULL, &lic_type, lic_data, &lic_size);
        RegCloseKey(key_handle);

        if (result == 0 && lic_size >= 128) {
            return FALSE;
        }
    }

    {
        byte install_dir[260];
        DWORD dir_len = sizeof(install_dir);
        (void)install_dir;
        (void)dir_len;
    }

    return FALSE;
}

/*
 * Armadillo stores the last known system time in an encrypted registry
 * value or file. If the current time is before the stored time, it
 * detects clock-backdating (a common trial reset trick).
 *
 * The FIXCLOCK command resets this detection, useful when the user
 * legitimately changes their system clock (e.g., timezone travel).
 */

static BOOL armadillo_check_clock_tamper(void)
{
    byte   saved_time[8];   /* Encrypted FILETIME */
    DWORD  saved_size = sizeof(saved_time);
    u64    current_time;
    u64    last_time;
    HANDLE key_handle;

    if (RegOpenKeyExA(NULL,
            "Software\\Jamdat\\Lemonade Tycoon 2",
            0, 1, &key_handle) == 0) {

        RegQueryValueExA(key_handle, "LastRun",
            NULL, NULL, saved_time, &saved_size);
        RegCloseKey(key_handle);

        if (saved_size == 8) {
            byte hw_id[16];
            armadillo_generate_hardware_id(hw_id);

            last_time = ((u64)saved_time[0]) |
                       ((u64)saved_time[1] << 8) |
                       ((u64)saved_time[2] << 16) |
                       ((u64)saved_time[3] << 24) |
                       ((u64)saved_time[4] << 32) |
                       ((u64)saved_time[5] << 40) |
                       ((u64)saved_time[6] << 48) |
                       ((u64)saved_time[7] << 56);

            last_time ^= ((u64 *)hw_id)[0];

            {
                current_time = last_time + 10000000ULL * 3600 * 24;
            }

            if (current_time < last_time) {
                return TRUE;
            }
        }
    }

    return FALSE;
}

static BOOL armadillo_generate_or_reissue_key_flow(void)
{
    /* DLL 0x10003BC9: REGISTER/key retrieval flow, operations 0/1.
     * Called by 0x1001C417. Requires the activation URL config, may open the
     * browser/manual fallback UI, and stores returned `name\r\nkey` material.
     */
    return FALSE;
}

static BOOL armadillo_generate_no_trial_key_flow(void)
{
    /* DLL 0x10004B74: startup serial-to-key flow, operation 2.
     * Called by 0x10018876 when online flags are enabled, RequisitionID exists,
     * and local no-trial state bit 3 is not already set.
     */
    return FALSE;
}

static BOOL armadillo_periodic_validate_license_flow(void)
{
    /* DLL 0x10004D95: periodic Internet validateLicense flow, operation 3.
     * Present in the DLL but no static call xref was found. It checks day/retry
     * state before sending validateLicense and can move into a shutdown warning
     * path after repeated or expired validation failures.
     */
    return FALSE;
}

static BOOL armadillo_activate_license_flow(void)
{
    /* DLL 0x1000511A: activateLicense flow, operation 4.
     * Called through 0x1001B720 and splits the returned `name\r\nkey` pair into
     * caller-provided output buffers.
     */
    return FALSE;
}

static void armadillo_show_license_error(const char *message)
{
    MessageBoxA(NULL,
        message,
        "Lemonade Tycoon 2",
        0x10  /* MB_ICONERROR */);
}

/*
 * Verifies protector section CRCs; failure can terminate or corrupt
 * unpacking behavior.
 */

static BOOL armadillo_verify_section_crc(u32 section_addr, u32 section_size,
                                   u32 expected_crc)
{
    u32 actual_crc;

    actual_crc = armadillo_crc32_compute(
        (const byte *)(uintptr_t)section_addr, section_size, 0);

    if (actual_crc != expected_crc) {
        return FALSE;
    }
    return TRUE;
}

static BOOL armadillo_detect_debugger(void)
{
    BOOL detected = FALSE;

    if (IsDebuggerPresent()) {
        detected = TRUE;
    }

    {
        HANDLE hwnd;
        hwnd = FindWindowA("OLLYDBG", NULL);
        if (hwnd != NULL) detected = TRUE;
        hwnd = FindWindowA("WinDbgFrameClass", NULL);
        if (hwnd != NULL) detected = TRUE;
        hwnd = FindWindowA("ID", NULL);  /* IDA Pro */
        if (hwnd != NULL) detected = TRUE;
    }

    return detected;
}

/*
 * Decompresses the .pdata section into a VirtualAlloc'd buffer.
 * .pdata section: 0x00503000, 0x00080000 (512 KB compressed)
 */

static byte *armadillo_decompress_payload(u32 *out_size)
{
    byte  *compressed_base;
    u32    compressed_size;
    u32    decompressed_size;
    byte  *decompressed;

    compressed_base   = (byte *)0x00503000;
    compressed_size   = 0x00080000;
    decompressed_size = *(u32 *)compressed_base + 4;

    decompressed = (byte *)VirtualAlloc(
        NULL, decompressed_size,
        0x3000,  /* MEM_COMMIT | MEM_RESERVE */
        0x40     /* PAGE_EXECUTE_READWRITE */);

    if (decompressed == NULL) {
        return NULL;
    }

    *out_size = decompressed_size;
    return decompressed;
}

/*
 * After decompressing the payload, the protector resolves the import
 * table for the unpacked program. This involves:
 *
 * 1. Reading the IMAGE_IMPORT_DESCRIPTOR array from the unpacked PE
 * 2. For each DLL: LoadLibraryA, then GetProcAddress for each function
 * 3. Writing resolved addresses into the IAT (Import Address Table)
 * 4. Optionally installing Armadillo API hooks via SetFunctionAddresses
 *
 * String evidence:
 *   0x004e3628: "KERNEL32.DLL"
 *   0x004e383c: "COMCTL32.DLL"
 *   0x004ebbc2: "USER32.dll"
 *   0x004ebc6a: "GDI32.dll"
 *
 * DLL init helper:
 *   0x004e363c: "INITIALIZEDLLADDR"
 */

static BOOL armadillo_resolve_imports(byte *unpacked_base)
{
    return TRUE;
}

/*
 * The unpacked program was compiled with a preferred base address
 * that may differ from its runtime location. The protector applies
 * base relocations to fix up absolute addresses.
 *
 * Error string evidence:
 *   0x004e33d4: "Relocations error"
 */

static BOOL armadillo_apply_relocations(byte *unpacked_base,
                                  u32 actual_base,
                                  u32 preferred_base)
{
    s32 delta = (s32)(actual_base - preferred_base);

    if (delta == 0) return TRUE;  /* Loaded at preferred base, no fixups needed. */

    return TRUE;
}

/*
 * Armadillo_SetFunctionAddresses, DLL 0x10006c67.
 *   Hook #1 (at 0x1001f498): Anti-tamper relocation processor.
 *       Adjusts image base offsets, decrypts code sections with XTEA
 *       when they are first accessed, and encrypts them back afterward.
 *   Hook #2 (at 0x100292a9): Internal data table (configuration values,
 *       encrypted license data pointers, resource indices).
 *   Hook #3 (at 0x10028490): Critical-section-guarded serialization
 *       wrapper.  Calls FUN_100284ad inside Enter/LeaveCriticalSection.
 * One-shot: DAT_10038128 gates subsequent calls.
 */

/* Globals in the protection DLL (at 0x10038100-0x10038128 range) */
static void *g_armadillo_hook_api_entries[11];
static void *g_armadillo_hook_reloc_proc;    /* -> FUN_1001f498 */
static void *g_armadillo_hook_data_table;    /* -> DAT_100292a9 */
static void *g_armadillo_hook_cs_wrapper;    /* -> FUN_10028490 */
static BOOL  g_armadillo_hooks_installed;

static void armadillo_set_function_addresses(
    void *api_1,  void *api_2,  void *api_3,  void *api_4,
    void *api_5,  void *api_6,  void *api_7,  void *api_8,
    void *api_9,  void *api_10, void *api_11,
    void **out_hook_reloc, void **out_hook_data, void **out_hook_cs)
{
    if (g_armadillo_hooks_installed)
        return;

    g_armadillo_hook_api_entries[0]  = api_1;
    g_armadillo_hook_api_entries[1]  = api_2;
    g_armadillo_hook_api_entries[2]  = api_3;
    g_armadillo_hook_api_entries[3]  = api_4;
    g_armadillo_hook_api_entries[4]  = api_5;
    g_armadillo_hook_api_entries[5]  = api_6;
    g_armadillo_hook_api_entries[6]  = api_7;
    g_armadillo_hook_api_entries[7]  = api_8;
    g_armadillo_hook_api_entries[8]  = api_9;
    g_armadillo_hook_api_entries[9]  = api_10;
    g_armadillo_hook_api_entries[10] = api_11;

    *out_hook_reloc = g_armadillo_hook_reloc_proc;
    *out_hook_data  = g_armadillo_hook_data_table;
    *out_hook_cs    = g_armadillo_hook_cs_wrapper;

    g_armadillo_hooks_installed = TRUE;
}

/*
 * Entry stub at 0x004d3000. The protection DLL exports SetFunctionAddresses;
 * encrypted sections are decrypted on first use with XTEA.
 */

void armadillo_entry_point(void)
{
    byte  *unpacked_base;
    u32    unpacked_size;
    BOOL   license_ok;

    {
        byte buf[32];
        if (GetEnvironmentVariableA("ARMDEBUG", (char *)buf, sizeof(buf)) > 0) {
        }
    }

    {
        byte buf[2];
        if (GetEnvironmentVariableA("ARMSPLASHOFF", (char *)buf, sizeof(buf)) > 0) {
        }
    }

    if (armadillo_detect_debugger()) {
    }

    armadillo_crc32_init();

    {
        u32 expected_crc;
        BOOL ok;

        expected_crc = *(u32 *)(g_armadillo_config + 0x200);  /* Stored at build time. */
        ok = armadillo_verify_section_crc(0x004d3000, 0x10000, expected_crc);
        if (!ok) {
            MessageBoxA(NULL, armadillo_error_failed_crc,
                        "Lemonade Tycoon 2", 0x10);
            ExitProcess(1);
        }
    }

    license_ok = armadillo_check_license();
    if (!license_ok) {
        const char *cmdline = GetCommandLineA();

        if (strstr(cmdline, ARMADILLO_CMD_REGISTER)) {
            armadillo_generate_or_reissue_key_flow();

            armadillo_show_license_error(
                "Your temporary key has expired. "
                "If you believe this message is in error, "
                "please contact the program's author.");
            ExitProcess(2);
        }

        if (strstr(cmdline, ARMADILLO_CMD_UNREGISTER)) {
            ExitProcess(0);
        }

        if (strstr(cmdline, ARMADILLO_CMD_TRANSFER)) {
            ExitProcess(0);
        }

        armadillo_show_license_error(
            "Your temporary key has expired. "
            "If you believe this message is in error, "
            "please contact the program's author.");
        ExitProcess(2);
    }

    if (armadillo_check_clock_tamper()) {
    }

    unpacked_base = armadillo_decompress_payload(&unpacked_size);
    if (unpacked_base == NULL) {
        MessageBoxA(NULL, armadillo_error_general,
                    "Lemonade Tycoon 2", 0x10);
        ExitProcess(3);
    }

    {
        s32 preferred_base = 0x00400000;  /* From PE header. */
        if (!armadillo_apply_relocations(unpacked_base,
                                   (u32)(uintptr_t)unpacked_base,
                                   preferred_base)) {
            MessageBoxA(NULL, armadillo_error_relocations,
                        "Lemonade Tycoon 2", 0x10);
            ExitProcess(4);
        }
    }

    if (!armadillo_resolve_imports(unpacked_base)) {
        MessageBoxA(NULL, armadillo_error_find_import,
                    "Lemonade Tycoon 2", 0x10);
        ExitProcess(5);
    }

    {
        void *hook_reloc = NULL;
        void *hook_data = NULL;
        void *hook_cs = NULL;

        armadillo_set_function_addresses(
            NULL, NULL, NULL, NULL,
            NULL, NULL, NULL, NULL,
            NULL, NULL, NULL,
            &hook_reloc, &hook_data, &hook_cs);
    }

    {
        memset((void *)0x004d3000, 0xCC, 0x10000);  /* INT3 sled */
    }

    {
        u32 oep = 0x00401000;
        ((void (*)(void))(uintptr_t)oep)();
    }
}

static void armadillo_show_splash(void)
{
}

static void armadillo_hide_splash(void)
{
}

/*
 * Debugger-as-parent mode is indicated by DebugActiveProcess,
 * WaitForDebugEvent, ContinueDebugEvent, ReadProcessMemory,
 * WriteProcessMemory, GetThreadContext, and SetThreadContext imports.
 */

static void armadillo_debugger_child_mode(void)
{
}

/*
 * The .pdata section at file offset 0x47000 (RVA 0x00103000) is a
 * custom container format used by Armadillo to store compressed assets.
 *
 * CONTAINER STRUCTURE:
 *
 *   [Entry 1]  (always starts with "PDATA000")
 *     +0x00  char sig[8]     "PDATA000" signature
 *     +0x08  uint32[3]       metadata (type, flags, sizes?)
 *     +0x13  78 DA            raw deflate stream (zlib, no container)
 *     <zlib_stream_1>
 *
 *   [Entry 2+] (subsequent entries, 13-byte header)
 *     +0x00  uint32[3]       metadata
 *     +0x0C  byte             flags
 *     +0x0D  78 DA            raw deflate stream
 *     <zlib_stream_N>
 *
 * ENTRY BOUNDARIES:
 *   Entries are not length-prefixed in a header index.  To find the
 *   end of each stream, inflate byte-by-byte until zlib.eof is True.
 *   The next entry's data follows immediately (possibly with padding
 *   to an alignment boundary).
 *
 * STREAM CONTENTS (Lemonade2.exe):
 *   #1  offset 0x00000013  37,464 B  BMP   splash/loading image
 *   #2  offset 0x0000615C 258,048 B  PE32  protection runtime DLL
 *   #3  offset 0x0002785F  14,314 B  TEXT  DRM error strings (UTF-16LE)
 */
