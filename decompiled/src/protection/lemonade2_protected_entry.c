/*
 * Source: decompiled/local/lt2_install/Lemonade2.exe
 * SHA-256: 784BD579465E3971EA6FE600592B9C13B67A1DBF8EB7243162ACDA45E1CCD3C7
 * Image base: 0x00400000
 * Entry point: 0x004D3000
 *
 * Protected/packed. The real game sections at .text,
 * .rdata, and .data have virtual size but zero raw bytes. Packed/high-entropy
 * sections are .text1 and .pdata, and the entry point lands in .adata. Static
 * analysis sees the loader stub, not the unpacked game logic.
 *
 * Local strings/import evidence:
 * - "Cannot locate protected program data"
 * - "ARMDEBUG="
 * - "IsDebuggerPresent"
 * - "SetFunctionAddresses"
 * - "deflate 1.1.4 Copyright 1995-2002 Jean-loup Gailly"
 * - "inflate 1.1.4 Copyright 1995-2002 Mark Adler"
 * - imports include DebugActiveProcess, ReadProcessMemory, WriteProcessMemory,
 *   GetThreadContext, SetThreadContext, VirtualProtectEx, and CreateProcessW.
 */
#include "common/win32_recovered_types.h"

/*
 * protected_entry_stub
 *
 * The first instructions are deliberately hostile to linear disassembly:
 *
 *   004D3000  PUSHAD
 *   004D3001  CALL 004D3006
 *   004D3006  POP EBP
 *   004D3007  PUSH EAX
 *   004D3008  PUSH ECX
 *   004D3009  BSWAP EDX
 *   004D300B  NOT EDX
 *   004D300D  PUSHFD
 *   004D300E  NOT EDX
 *   004D3010  BSWAP EDX
 *   004D3012  JMP 004D3023
 *   004D301B  JMP 004D302C
 *   004D3023  JMP 004D301B
 *   004D302C  JMP 004D3017
 *
 * Overlapping instructions and bad control-flow data are expected here.
 */
void protected_entry_stub(void)
{
    /*
     * Do not treat this as game logic. It is the unpacker/protector entry.
     * The original client code is represented by the normalized unpacked image
     * at decompiled/local/unpacked/Lemonade2.unpacked.exe.
     */
}
