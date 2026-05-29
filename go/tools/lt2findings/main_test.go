package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadFindingsAndRenderCIndex(t *testing.T) {
	dir := t.TempDir()
	findingsPath := filepath.Join(dir, "findings.ini")
	data := strings.Join([]string{
		"[fmod.start]",
		"source_file = decompiled/local/lt2_install/fmod.dll",
		"program = fmod.dll",
		"address = 10000000",
		"kind = pre",
		"label = fmod_audio_library_start",
		"title = Third-party audio",
		"c_symbol = ",
		"comment = Dependency context only.",
		"",
	}, "\n")
	if err := os.WriteFile(findingsPath, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	findings, err := loadFindings(findingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 {
		t.Fatalf("len(findings) = %d, want 1", len(findings))
	}
	if findings[0].SourceFile != "decompiled/local/lt2_install/fmod.dll" {
		t.Fatalf("source = %q", findings[0].SourceFile)
	}

	cSymbols := []cSymbol{{
		Kind:      "function",
		Name:      "fmod_start_probe",
		Source:    "decompiled/src/audio/fmod.c",
		Line:      12,
		FindingID: "fmod.start",
	}}
	rendered := string(renderCIndex(findingsPath, findings, cSymbols, nil))
	if !strings.Contains(rendered, `LT2_FINDING("fmod.start", "decompiled/local/lt2_install/fmod.dll", "fmod.dll", 0x10000000U`) {
		t.Fatalf("rendered index did not include X-macro record:\n%s", rendered)
	}
	if !strings.Contains(rendered, `LT2_C_SYMBOL("function", "fmod_start_probe", "decompiled/src/audio/fmod.c", 12, "fmod.start")`) {
		t.Fatalf("rendered index did not include C symbol record:\n%s", rendered)
	}
	if !strings.Contains(rendered, `LT2_FINDING_TAG("fmod.start", "LT2_FINDING_fmod_start")`) {
		t.Fatalf("rendered index did not include finding tag record:\n%s", rendered)
	}
	if !strings.Contains(rendered, `LT2_C_SYMBOL_TAG("function", "fmod_start_probe", "decompiled/src/audio/fmod.c", 12, "LT2_C_FUNCTION")`) {
		t.Fatalf("rendered index did not include C symbol tag record:\n%s", rendered)
	}
	if !strings.Contains(rendered, `LT2_C_SYMBOL_TAG("function", "fmod_start_probe", "decompiled/src/audio/fmod.c", 12, "LT2_LINKED_FINDING_fmod_start")`) {
		t.Fatalf("rendered index did not include linked finding tag record:\n%s", rendered)
	}
	if strings.Contains(rendered, "container input") {
		t.Fatalf("rendered index mentioned the container input:\n%s", rendered)
	}
}

func TestLoadResolvedCRTSymbols(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "resolved_crt_wrappers.jsonl")
	data := strings.Join([]string{
		`{"schema":"lt2.analysis_event.v1","stage":"crt_resolver","program":"lemonade2_stream2_unpacked.dll","address":"10001000","action":"renamed","symbol_name":"CRT_RefCountRelease","old_name":"FUN_10001000","new_name":"CRT_RefCountRelease","category":"crt_wrapper","confidence":92,"decision":"accepted","evidence":"refcount"}`,
		`{"schema":"lt2.analysis_event.v1","stage":"crt_resolver","program":"lemonade2_stream2_unpacked.dll","address":"10001010","action":"confirmed_existing","symbol_name":"CRT_StringAssign_10001010","category":"crt_wrapper","confidence":100,"decision":"accepted","evidence":"existing resolved CRT name"}`,
		`{"schema":"lt2.analysis_event.v1","stage":"crt_resolver","program":"lemonade2_stream2_unpacked.dll","address":"10001020","action":"renamed","symbol_name":"IAT_KERNEL32_free","old_name":"FUN_10001020","new_name":"IAT_KERNEL32_free","category":"iat_thunk","confidence":95,"decision":"accepted","evidence":"import"}`,
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	symbols, err := loadResolvedCRTSymbols(path, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(symbols) != 2 {
		t.Fatalf("len(symbols) = %d, want 2", len(symbols))
	}
	if symbols[0].Kind != "resolved_crt_wrapper" || symbols[0].Name != "CRT_RefCountRelease" {
		t.Fatalf("unexpected symbol: %#v", symbols[0])
	}
	rendered := string(renderCIndex("findings.ini", nil, nil, symbols))
	if !strings.Contains(rendered, "Resolved CRT Wrappers") {
		t.Fatalf("missing resolved CRT section:\n%s", rendered)
	}
	if !strings.Contains(rendered, `LT2_C_SYMBOL("resolved_crt_wrapper", "CRT_RefCountRelease"`) {
		t.Fatalf("missing resolved CRT symbol:\n%s", rendered)
	}
	if !strings.Contains(rendered, `LT2_C_SYMBOL_TAG("resolved_crt_wrapper", "CRT_RefCountRelease",`) ||
		!strings.Contains(rendered, `"LT2_RESOLVED_CRT_WRAPPER")`) {
		t.Fatalf("missing resolved CRT tag:\n%s", rendered)
	}
}

func TestLoadResolvedCRTSymbolsRejectsConflicts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "resolved_crt_wrappers.jsonl")
	data := strings.Join([]string{
		`{"schema":"lt2.analysis_event.v1","stage":"crt_resolver","program":"prog","address":"1000","action":"renamed","symbol_name":"known_label","old_name":"FUN_1000","new_name":"known_label","category":"crt_wrapper","confidence":92,"decision":"accepted","evidence":"refcount"}`,
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := loadResolvedCRTSymbols(path, []finding{{ID: "known", Label: "known_label"}}, nil)
	if err == nil || !strings.Contains(err.Error(), "conflicts") {
		t.Fatalf("expected conflict error, got %v", err)
	}
}

func TestScanCSymbolsInFile(t *testing.T) {
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "sample.c")
	source := strings.Join([]string{
		"#define LT2_LIMIT 32",
		"#ifndef LT2_SAMPLE_H",
		"#define LT2_SAMPLE_H",
		"typedef enum {",
		"    LT2_ZERO = 0,",
		"    LT2_ONE,",
		"} lt2_mode;",
		"static const unsigned int lt2_seed = 7;",
		"static int lt2_helper(",
		"    int value)",
		"{",
		"    int local_noise = value;",
		"    return value + LT2_LIMIT;",
		"}",
		"typedef int (*lt2_callback)(void *self);",
		"static void **lt2_vtable(void *self)",
		"{",
		"    return self == 0 ? 0 : *(void ***)self;",
		"}",
		"#endif",
		"",
	}, "\n")
	if err := os.WriteFile(sourcePath, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}

	symbols, err := scanCSymbolsInFile(sourcePath, map[string]string{
		"lt2_helper": "sample.helper",
	})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]cSymbol{}
	for _, symbol := range symbols {
		got[symbol.Kind+":"+symbol.Name] = symbol
	}
	for _, key := range []string{
		"macro:LT2_LIMIT",
		"macro:LT2_SAMPLE_H",
		"enum:LT2_ZERO",
		"enum:LT2_ONE",
		"type:lt2_mode",
		"type:lt2_callback",
		"global:lt2_seed",
		"function:lt2_helper",
		"function:lt2_vtable",
	} {
		if _, ok := got[key]; !ok {
			t.Fatalf("missing %s in %#v", key, symbols)
		}
	}
	if got["function:lt2_helper"].FindingID != "sample.helper" {
		t.Fatalf("linked finding = %q", got["function:lt2_helper"].FindingID)
	}
	if _, ok := got["global:local_noise"]; ok {
		t.Fatalf("local function variable was indexed as a global: %#v", symbols)
	}
}

func TestScanCSymbolsDoesNotIndexLocalsInNestedFunctionBodies(t *testing.T) {
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "nested.c")
	source := strings.Join([]string{
		"int lt2_copy_or_download_file(char *local_path, char *source_url,",
		"                              u32 source_size, u32 progress_start,",
		"                              u32 progress_end)",
		"{",
		"    if (installer_strnicmp_locale((byte *)source_url, \"file://\", 7) == 0) {",
		"        char converted[260];",
		"        char *p = source_url + 7;",
		"        if (*p == '/') *p = '\\\\';",
		"        return CopyFileA(converted, local_path, FALSE) != 0;",
		"    }",
		"    {",
		"        HANDLE hUrl = g_pInternetOpenUrlA(",
		"            g_hInternet, source_url, NULL, 0, 0, 0);",
		"        if (hUrl != NULL) {",
		"            DWORD bytes_read = 0;",
		"        }",
		"    }",
		"    return 1;",
		"}",
		"",
	}, "\n")
	if err := os.WriteFile(sourcePath, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}

	symbols, err := scanCSymbolsInFile(sourcePath, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, symbol := range symbols {
		got[symbol.Kind+":"+symbol.Name] = true
	}
	if !got["function:lt2_copy_or_download_file"] {
		t.Fatalf("missing function in %#v", symbols)
	}
	for _, key := range []string{"global:converted", "global:p", "global:hUrl", "global:bytes_read"} {
		if got[key] {
			t.Fatalf("local variable %s was indexed as a global: %#v", key, symbols)
		}
	}
}

func TestLoadFindingsRejectsKeyBeforeSection(t *testing.T) {
	dir := t.TempDir()
	findingsPath := filepath.Join(dir, "findings.ini")
	if err := os.WriteFile(findingsPath, []byte("program = Lemonade2.exe\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := loadFindings(findingsPath)
	if err == nil || !strings.Contains(err.Error(), "before first finding section") {
		t.Fatalf("expected key-before-section error, got %v", err)
	}
}

func TestIsAllowedSourceFileAllowsLicenseInstallerTarget(t *testing.T) {
	f := finding{
		ID:         "license.hash_lookup",
		SourceFile: "decompiled/local/installers/Lemonade Tycoon 2 - New York City.exe",
		Program:    installerProgram,
	}
	if !isAllowedSourceFile(f) {
		t.Fatal("expected explicit installer-side license target to be allowed")
	}
	f.Program = "Lemonade2.exe"
	if isAllowedSourceFile(f) {
		t.Fatal("installer target should not be allowed for unrelated programs")
	}
	f.Program = installerProgram
	f.ID = "lemonade2.protected_entry"
	if isAllowedSourceFile(f) {
		t.Fatal("installer target should only be allowed for explicit license findings")
	}
}

func TestIsAllowedSourceFileAllowsExtractedPdataAsset(t *testing.T) {
	f := finding{
		ID:         "armadillo.xtea_cipher",
		SourceFile: "decompiled/local/pdata_assets/pdata_002_0x04D15C_pe32_dll.dll",
		Program:    "pdata_002_0x04D15C_pe32_dll.dll",
	}
	if !isAllowedSourceFile(f) {
		t.Fatal("expected extracted .pdata DLL asset to be allowed")
	}
}

func TestGeneratedCIndexCoversScannedCSourceFiles(t *testing.T) {
	root := repoRoot(t)
	t.Chdir(root)

	findingsPath := defaultFindingsPath
	cSearchDir := defaultCSearchDir
	cIndexPath := defaultCIndexPath
	resolvedCRTPath := defaultResolvedCRTPath

	findings, err := loadFindings(findingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateFindings(findings, cSearchDir); err != nil {
		t.Fatal(err)
	}
	cSymbols, err := scanCSymbols(cSearchDir, findings)
	if err != nil {
		t.Fatal(err)
	}
	resolvedCRT, err := loadResolvedCRTSymbols(resolvedCRTPath, findings, cSymbols)
	if err != nil {
		t.Fatal(err)
	}

	current, err := os.ReadFile(cIndexPath)
	if err != nil {
		t.Fatal(err)
	}
	generated := renderCIndex(findingsPath, findings, cSymbols, resolvedCRT)
	if !bytes.Equal(current, generated) {
		t.Fatalf("%s is out of sync; run go run ./tools/lt2findings -write", defaultCIndexPath)
	}

	scannedSources := map[string]bool{}
	if err := walkCSourceFiles(cSearchDir, func(path string) error {
		scannedSources[filepath.ToSlash(path)] = false
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	functions := 0
	taggedFunctions := map[string]bool{}
	for _, symbol := range cSymbols {
		scannedSources[symbol.Source] = true
		if symbol.Kind == "function" {
			functions++
			taggedFunctions[symbol.Name] = false
		}
	}
	for line := range strings.SplitSeq(string(current), "\n") {
		if !strings.HasPrefix(line, `LT2_C_SYMBOL_TAG("function", `) {
			continue
		}
		for name := range taggedFunctions {
			if strings.Contains(line, `"`+name+`"`) && strings.Contains(line, `"LT2_C_FUNCTION"`) {
				taggedFunctions[name] = true
			}
		}
	}
	for source, present := range scannedSources {
		if !present {
			t.Fatalf("missing LT2_C_SYMBOL records for scanned source %s", source)
		}
	}
	for name, tagged := range taggedFunctions {
		if !tagged {
			t.Fatalf("missing LT2_C_FUNCTION tag for scanned function %s", name)
		}
	}
	if functions == 0 {
		t.Fatal("no C functions were scanned")
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "decompiled")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root")
		}
		dir = parent
	}
}
