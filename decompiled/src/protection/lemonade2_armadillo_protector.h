/*
 * Lemonade2.exe Armadillo Software Protection / DRM runtime.
 *
 * Source: decompiled/local/lt2_install/Lemonade2.exe (packed/protected).
 *   SHA-256: 784BD579465E3971EA6FE600592B9C13B67A1DBF8EB7243162ACDA45E1CCD3C7
 *
 * The executable is protected with Armadillo (Digital River / SoftwarePassport).
 *
 * On 2026-05-29 we extracted the .pdata section (PDATA000 container, 3 zlib streams):
 *   Stream 1:  37 KB BMP  -- splash/loading image
 *   Stream 2: 258 KB PE32 -- Armadillo protection runtime DLL (990 functions)
 *   Stream 3:  14 KB text -- all DRM error/dialog strings (UTF-16-LE, 7157 chars)
 *
 * Historical note: this header predates the static unpacking promotion. The
 * current authoritative path derives the normalized game EXE with
 * `go/tools/lt2normalize -derive-static-normalized`; `.text1` is loader-stage
 * evidence, not the final original `.text` source of truth.
 *
 * Protection DLL addresses: image base 0x10000000.
 * Packed EXE addresses:      image base 0x00400000.
 */

#ifndef LT2_LEMONADE2_ARMADILLO_PROTECTOR_H
#define LT2_LEMONADE2_ARMADILLO_PROTECTOR_H

#include "common/win32_recovered_types.h"

/* ================================================================== */
/*  1.  PE Section Layout                                              */
/* ================================================================== */

/*
 * Section   V.Addr         V.Size       Raw Size    Notes
 * -------   ------         ------       --------    -----
 * .text     0x00401000     0x000902d6   0           (zero-filled placeholder)
 * .rdata    0x00492000     0x00009aee   0           (zero-filled placeholder)
 * .data     0x0049c000     0x00006578   0           (zero-filled placeholder)
 * .text1    0x004a3000     0x00030000   0x00030000  encrypted loader stage
 * .adata    0x004d3000     0x00010000   0x0000d000  Protector stub (anti-disasm)
 * .data1    0x004e3000     0x00020000   0x00009000  Protector strings/config
 * .pdata    0x00503000     0x00080000   0x00071000  PDATA000 container (zlib)
 * .rsrc     0x00583000     0x00003000   0x00003000  Resources
 *
 * .text, .rdata, .data are zero-filled at rest; the real data is in the
 * encrypted .text1 and the compressed .pdata sections.
 * Entry point: 0x004d3000 (.adata), uses anti-disassembly tricks.
 */

/* ================================================================== */
/*  2.  Command-line / Environment Switches                             */
/* ================================================================== */

/*
 * Environment variables (parsed by the protector stub):
 *   ARMDEBUG=<pid>     Debugger attach target
 *   ARMSPLASHOFF       Disable the "Loading..." splash window
 *
 * Command-line arguments (from GetCommandLineA):
 *   REGISTER            Show registration/license dialog
 *   UNREGISTER          Remove license from this machine
 *   TRANSFER            Transfer license to another machine
 *   QUIETREGISTER       Silent registration (no GUI)
 *   SHOWNETUSERS        Display network licensed users
 *   FIXCLOCK            Reset clock-backdating detection
 *   DISPLAY             Show current license information
 *   SERVER              Run as floating license server
 *   LOADINGWINDOW       Toggle splash on/off
 */

#define ARMADILLO_CMD_REGISTER      "REGISTER"
#define ARMADILLO_CMD_UNREGISTER    "UNREGISTER"
#define ARMADILLO_CMD_TRANSFER      "TRANSFER"
#define ARMADILLO_CMD_QUIETREGISTER "QUIETREGISTER"
#define ARMADILLO_CMD_SHOWNETUSERS  "SHOWNETUSERS"
#define ARMADILLO_CMD_FIXCLOCK      "FIXCLOCK"
#define ARMADILLO_CMD_DISPLAY       "DISPLAY"
#define ARMADILLO_CMD_SERVER        "SERVER"

/* ================================================================== */
/*  3.  XTEA Cipher (core crypto engine)                                */
/* ================================================================== */

/*
 * Decompiled from DLL at 0x10001363 (Armadillo_XTEA_Cipher).
 *
 * Standard XTEA (eXtended TEA), 64-bit blocks, 128-bit key, 32 rounds.
 *
 * Master key:  0xF1C62847
 * Delta:        0x9E3779B9  = (sqrt(5) - 1) * 2^31
 * Neg delta:   -0x61C88647  (used for decryption)
 *
 * Key schedule (Armadillo_SetCipherKey at 0x100014ac):
 *   k[0] = master_key;
 *   k[1] = (master_key << 24) | (master_key >> 8);
 *   k[2] = (master_key >> 8) << 24 | (k[1] >> 8);
 *   k[3] = (k[1] >> 8) << 24 | (k[2] >> 8);
 *
 * Modes (param 'encrypt'):
 *   < 1 : Decrypt
 *   < 0 : Decrypt with CBC-like chaining
 *   >=1 : Encrypt
 *   > 1 : Encrypt with CBC-like chaining
 *
 * Used by 37 callers for on-the-fly code section decryption (code
 * virtualization), license data encryption in registry, and resource
 * string decryption.
 */

/* ================================================================== */
/*  4.  Complete Named Function Table (Protection DLL)                  */
/* ================================================================== */

/*
 * Core Crypto:
 *   0x10001363  Armadillo_XTEA_Cipher               XTEA 32-round encrypt/decrypt
 *   0x100014ac  Armadillo_SetCipherKey              Key rotation + XTEA call
 *   0x10001dcc  Armadillo_RotateBits                32-bit bit rotation utility
 *   0x100013b1b Armadillo_Base64Codec              Base64 encode/decode license data
 *
 * Hardware Fingerprinting:
 *   0x100083ef  Armadillo_GenerateHardwareFingerprint   BIOS ver/date from registry
 *   0x10008211  Armadillo_GetSystemEntropy              Timestamp/process counters
 *   0x10008540  Armadillo_GetComputerNameHash           GetComputerNameA -> hash
 *   0x100085bf  Armadillo_GetVolumeSerialHash           GetVolumeInformationA -> hash
 *   0x10008184  Armadillo_BuildCompositeFingerprint     Combine 8 entropy sources
 *
 * License Storage (Registry + Files):
 *   0x100013e0d Armadillo_ReadLicenseStorage       Read from HKLM/HKCU/temp/.key files
 *   0x10001423f Armadillo_WriteLicenseStorage      Write to HKLM/HKCU/temp/.key files
 *   0x1000148c6 Armadillo_ExportLicenseToAhclFile   Save license as .AHCL backup file
 *
 * Online Activation (SOAP XML-RPC):
 *   0x10003a73 Armadillo_BuildActivationUrl        Build activation URL with params
 *   0x10003b93 Armadillo_HasActivationConfig       Require online config fields
 *   0x10003bbe Armadillo_HasRequisitionId          Require RequisitionID field
 *   0x100041b0 Armadillo_ParseActivationUrl        Parse http://host:port/path
 *   0x100042a4 Armadillo_BuildSoapRequest          Build SOAP XML-RPC envelope
 *   0x10004843 Armadillo_ParseSoapResponse         Parse <result>/<entitlementID>/<key>
 *   0x10003ed2 Armadillo_OnlineSoapTransaction     Common socket/SOAP helper
 *   0x10003bc9 Armadillo_GenerateOrReissueKeyFlow  REGISTER generate/reissue flow
 *   0x10004b74 Armadillo_GenerateNoTrialKeyFlow    Startup no-trial key flow
 *   0x10004d95 Armadillo_PeriodicValidateLicenseFlow  validateLicense routine
 *   0x1000511a Armadillo_ActivateLicenseFlow       activateLicense routine
 *
 * License Validation:
 *   0x1000145e5 Armadillo_LicenseSignatureVerify    RSA sig verification (31 primes)
 *
 * API Hook Infrastructure:
 *   0x10006c67 Armadillo_SetFunctionAddresses     Store 11 API addrs + 3 hook outs
 *   0x1001f498 (hook #1)                    Anti-tamper relocation processor
 *   0x1002929a (hook #2)                    Internal config data table
 *   0x10028490 (hook #3)                    CS-guarded serialization wrapper
 *
 * DLL Lifecycle:
 *   0x1000293f5 (DllMain init)                InitCriticalSection, store hInstance
 *   0x10002e2e5 entry                         DLL entry point (chains orig DllMain)
 *
 * Support / Utility:
 *   0x100020f8 (resource release)            177 xrefs, refcounted free
 *   0x1000773b (resource loader)             XOR + XTEA resource string loader
 *   0x10006c3b (indirect hash)               Hash function pointer (API hook #7)
 *   0x10002951f (seed derivation)             Obfuscated key seed calculator
 */

/* ================================================================== */
/*  5.  Hardware Fingerprint Algorithm                                  */
/* ================================================================== */

/*
 * Armadillo_BuildCompositeFingerprint (0x10008184) takes a bitmask and returns
 * one hardware fingerprint hash element. Callers invoke it for bitmasks
 * `1 << 0` through `1 << 30` to build the 31-element arrays used for RSA key
 * construction and license binding:
 *
 *   Bit 0: Armadillo_GetSystemEntropy      Timestamp + process counter + CPU enum
 *   Bit 1: Armadillo_GenerateHardwareFingerprint  BIOS version + BIOS date (registry)
 *   Bit 2: Armadillo_GetComputerNameHash          GetComputerNameA
 *   Bit 3: Armadillo_GetVolumeSerialHash          GetVolumeInformationA (C:\ serial)
 *   Bit 4: (FUN_1000865c)                    Additional entropy source
 *   Bit 5: (FUN_100087b0)                    Additional entropy source
 *   Bit 6: (FUN_10008d8f)                    Additional entropy source
 *   Bit 7: (FUN_100091b4)                    Additional entropy source
 *
 * Each source feeds into the previous result through FUN_10006c3b
 * (a hash function pointer).  The composite result is a 31-word
 * array used as the hardware-locked license fingerprint.
 *
 * Registry keys read for BIOS info:
 *   HKLM\Hardware\Description\System\SystemBiosVersion
 *   HKLM\Hardware\Description\System\SystemBiosDate
 *
 * Registry values stored for fingerprints:
 *   FINGERPRINT     (standard hardware ID)
 *   ENHFINGERPRINT  (enhanced hardware ID)
 */

/* ================================================================== */
/*  6.  ShortV3 Signed-Key Provenance                                  */
/* ================================================================== */

/*
 * The Go keygen constants in go/internal/lsx/keygen/signed.go are grounded in
 * the decoded Armadillo mapper metadata, not in the unpacked game code.
 *
 * Static derivation path:
 *   packed Lemonade2.exe .pdata tail
 *   -> FUN_1001F498 PRNG-XOR mapper window, seed 0x031E1692
 *   -> zlib metadata stream, size 0xC8B, checksum 0x7A192EDC
 *   -> product metadata and certificate records
 *
 * Public ShortV3 certificate record:
 *   kind:       0x19 (decimal 25, ShortV3 level 25)
 *   id/check:   0xD074508D
 *   payload:    9c c5 0e 4d 25 41 64 64 b9
 *   public y:   0x9CC50E4D25416464B9
 *
 * The matching license seed used by the validated natural key path is
 * 0xCCF0580A. `tools/lt2normalize -derive-validation-entry` confirms this seed
 * reaches FUN_10019D2B, selects validation entry 1/tag 0x04, repeat count 1161,
 * and produces complement trailer 0x5F8779E1/0xA078861E.
 *
 * Important: the private signing exponent is not embedded in Lemonade2.exe or
 * the Armadillo DLL, but it is part of this recovered project state. It was
 * recovered by solving the discrete log:
 *
 *   g^x mod p == y
 *   g = 0xF3C7E00A4B58155299
 *   p = 2^(9*8) + 0xE3B
 *   y = 0x9CC50E4D25416464B9
 *   x = 0x70301169DE7C75D66F
 *
 * signed_test.go verifies this relationship. The decompiled evidence is the
 * public certificate/seed path; the private exponent below is the mathematical
 * recovery from that evidence.
 */

/* ================================================================== */
/*  7.  SOAP XML-RPC License Activation Protocol                        */
/* ================================================================== */

/*
 * The Armadillo runtime uses SOAP 1.0 XML-RPC over HTTP to communicate with
 * Digital River activation servers. Five operation modes are assembled by
 * Armadillo_BuildSoapRequest at 0x100042A4:
 *
 *   Operation  | SOAP Action / Endpoint
 *   -----------|-------------------------------------------------------
 *   0          | generateKey         (get new license key)
 *   1          | reissueKey          (get replacement key)
 *   2          | generateKeyForNoTrial
 *   3          | validateLicense     (check if license is valid)
 *   4          | activateLicense     (activate a new license)
 *
 * SOAP actions discovered in the DLL:
 *   SOAPAction: "http://digitalriver.com/DigitalRight/activateLicense"
 *   SOAPAction: "http://digitalriver.com/DigitalRight/validateLicense"
 *   SOAPAction: "http://digitalriver.com/DigitalRight/generateKey"
 *   User-Agent: "ArmadilloDRM/1.0"
 *   Content-Type: text/xml; charset=utf-8
 *
 * XML fields in SOAP requests:
 *   entitlementID, productID, userName, key, password, downloadID,
 *   standardHardwareID, enhancedHardwareID, armVersion, drVersion,
 *   hardwareSignature, requisitionID, serialNumber, reqID
 *
 * The SOAP response is parsed for:
 *   <result>       -- integer status code (0 = success)
 *   <entitlementID> -- license entitlement identifier
 *   <userName>     -- registered user name
 *   <key>          -- returned ShortV3/user key for key-returning operations
 *
 * Activation URL parameters (built by Armadillo_BuildActivationUrl):
 *   BuyURL, KeyURL, CsrURL, DrVersion, RequisitionID, ProductID
 *   &hardwareSignature=<timestamp>
 *
 * Trigger map:
 *   0x10003BC9  generate/reissue key dialog path; called by REGISTER handler
 *               0x1001C417 and recursively for browser/manual fallback.
 *   0x10004B74  generateKeyForNoTrial / serial-to-key path; called from
 *               startup/license-state handling at 0x10018876 when online flags
 *               are enabled and the local state is not already marked complete.
 *   0x10004D95  validateLicense periodic check. It compares stored verification
 *               day fields against the current Armadillo day count, optionally
 *               prompts the user, then sends operation 3. This routine has no
 *               direct static call xref in the recovered DLL image; it is mapped
 *               as a present Armadillo routine, not a proven always-run launch
 *               check.
 *   0x1000511A  activateLicense path; called by API/helper 0x1001B720 and sends
 *               operation 4 after an optional user prompt.
 *   0x10003ED2  common HTTP/SOAP transaction helper used by all operation paths.
 */

/* ================================================================== */
/*  8.  License Storage Format                                          */
/* ================================================================== */

/*
 * License data is stored in three tiers:
 *
 * TIER 1 -- Registry (primary):
 *   HKLM\Software\Licenses\<magic_id>
 *   HKCU\Software\<Vendor>\<Product>\<value_name>
 *   HKCU\Software\The Silicon Realms Toolworks\Armadillo\<product>
 *   HKCU\Software\The Silicon Realms Toolworks\<product>
 *
 *   Magic identifier for license blocks:
 *     K7C0DB872A3F777C0
 *
 *   Registry value types:
 *     REG_SZ  (1): Base64-encoded XTEA-encrypted data (via Armadillo_ReadLicenseStorage)
 *     REG_BINARY (3): Raw binary (via Armadillo_WriteLicenseStorage)
 *
 *   License data is:
 *     1. XTEA-encrypted with derived keys
 *     2. Base64-encoded for REG_SZ storage
 *     3. CRC/hash-protected for integrity
 *
 * TIER 2 -- File system (backup):
 *   %TEMP%\<product>.key           -- temporary license backup
 *   <InstallDir>\<product>.AHCL    -- Armadillo Hardware Change Log export
 *
 *   .key files: Created via CreateFileA + WriteFile, timestamps set
 *               with SetFileTime for clock-tampering detection.
 *   .AHCL files: Export format via Armadillo_ExportLicenseToAhclFile,
 *                saved via GetSaveFileNameA dialog.
 *
 * TIER 3 -- License certificate:
 *   Stored as a digitally-signed blob (RSA signature over license data).
 *   Verified by Armadillo_LicenseSignatureVerify using the 31-element composite
 *   fingerprint as the RSA public key modulus components.
 */

/* ================================================================== */
/*  9.  Code Virtualization (Anti-Tamper)                               */
/* ================================================================== */

/*
 * The DLL encrypts its own code sections at rest.  On first access:
 *
 *   1. Enter critical section
 *   2. Adjust relocation pointers (subtract image base delta)
 *   3. Compute decryption key via FUN_1002951f + FUN_10006c3b
 *      Key constant: -0x31E6B6BF (= 0xCE194941)
 *   4. Decrypt code with Armadillo_SetCipherKey (XTEA)
 *   5. Execute decrypted code
 *   6. Optionally re-encrypt with Armadillo_SetCipherKey
 *   7. Restore relocation pointers
 *   8. Leave critical section
 *
 * Three major encrypted code blocks exist, each with its own key:
 *   Block A: FUN_1001dc85  (license check bootstrap)
 *   Block B: FUN_100204d5  (anti-tamper runtime)
 *   Block C: FUN_100273f5  (additional protection logic)
 *
 * .text1 decryption requires additional runtime data:
 *   Key = hash(base_addr + section_offset, data_ptr,
 *              seed + 0xCE194941 + image_base_delta)
 *   The seed is computed by FUN_1002951f which is itself obfuscated
 *   with overlapping instructions and bad control flow.
 */

/* ================================================================== */
/*  10.  SetFunctionAddresses API Hook Engine                           */
/* ================================================================== */

/*
 * Exported from the protection DLL at 0x10006c67.
 *
 * Called by the protector stub after unpacking to install IAT hooks.
 * Receives 11 API function addresses and returns 3 hook pointers:
 *
 *   Input:  11 API addresses stored in global array at 0x10038100
 *   Output: Hook #1 (0x1001f498) -- relocation processor + code decrypt
 *           Hook #2 (0x100292a9) -- internal data table pointer
 *           Hook #3 (0x10028490) -- CS-guarded serialization wrapper
 *
 * One-shot: after first call, DAT_10038128 is set and subsequent calls
 * return immediately.
 *
 * The 11 API entries represent real Windows API functions that Armadillo
 * intercepts.  Hook #1 (0x1001f498) is the most critical: it wraps
 * every intercepted API call, decrypts/encrypts protected code sections
 * around the call, and verifies relocations haven't been tampered with.
 */

/* ================================================================== */
/*  11.  Current Map Status                                              */
/* ================================================================== */

/*
 * This file is a working map of the packed/protector layer. The authoritative
 * restored game-code target is Lemonade2.unpacked.exe; this header tracks the
 * Armadillo runtime DLL and packed-loader evidence.
 *
 * COMPLETE (35 functions named):
 *   Crypto:        Armadillo_XTEA_Cipher, Armadillo_SetCipherKey, Armadillo_RotateBits,
 *                  Armadillo_Base64Codec
 *   Fingerprint:   Armadillo_BuildCompositeFingerprint, Armadillo_GetSystemEntropy,
 *                  Armadillo_GenerateHardwareFingerprint, Armadillo_GetComputerNameHash,
 *                  Armadillo_GetVolumeSerialHash, Armadillo_GetDiskDriveSerial,
 *                  Armadillo_GetSmartDriveIdentity, Armadillo_GetMacAddress,
 *                  Armadillo_GetRamSizeHash, Armadillo_ComputeFingerprintHash
 *   License I/O:   Armadillo_ReadLicenseStorage, Armadillo_WriteLicenseStorage,
 *                  Armadillo_ExportLicenseToAhclFile
 *   Validation:    Armadillo_LoadAndVerifyLicense, Armadillo_ValidateLicenseKey,
 *                  Armadillo_VerifyKeyType, Armadillo_LicenseSignatureVerify,
 *                  Armadillo_SerializeLicenseState
 *   Trial/Clock:   Armadillo_CheckTrialValidity, Armadillo_WriteLastRunDate,
 *                  Armadillo_FixClockHandler
 *   Activation:    Armadillo_BuildActivationUrl, Armadillo_ParseActivationUrl,
 *                  Armadillo_BuildSoapRequest, Armadillo_ParseSoapResponse,
 *                  Armadillo_OnlineSoapTransaction,
 *                  Armadillo_GenerateOrReissueKeyFlow,
 *                  Armadillo_GenerateNoTrialKeyFlow,
 *                  Armadillo_PeriodicValidateLicenseFlow,
 *                  Armadillo_ActivateLicenseFlow
 *   Engine:        Armadillo_SetFunctionAddresses
 *   Support:       Armadillo_LaunchWebBrowser, Armadillo_DumpLicenseDiagnostics
 *
 * Restored game code:
 *   [x] Normalized game target: decompiled/local/unpacked/Lemonade2.unpacked.exe
 *   [x] LSX upload/account/checksum/path functions verified in Ghidra
 *   [ ] Clean runnable PE import/OEP repair remains separate normalization work
 *
 * FOR THE GAME:
 *   Vendor:  "Jamdat"
 *   Product: "Lemonade Tycoon 2"
 *   License magic: K7C0DB872A3F777C0
 */

#endif /* LT2_LEMONADE2_ARMADILLO_PROTECTOR_H */
