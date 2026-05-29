MAKEFLAGS += --no-builtin-rules

BASH_BUILD := ./build.sh
PS_BUILD := ./build.ps1
BUILD_ARGS ?=

POWERSHELL ?= pwsh
SHELLCHECK ?= shellcheck
SHFMT ?= shfmt

BUILD_SH_CMD := $(BASH_BUILD)
BUILD_PS_CMD := $(POWERSHELL) -NoLogo -NoProfile -ExecutionPolicy Bypass -File $(PS_BUILD)

ifeq ($(OS),Windows_NT)
BUILD_CMD := $(BUILD_PS_CMD)
BUILD_BACKEND := build.ps1
else
BUILD_CMD := $(BUILD_SH_CMD)
BUILD_BACKEND := build.sh
endif

SUBMAKE_DIRS := \
	go/tools/keyvis_search \
	decompiled/analysis/lemonade2_static_unpacking

SH_SCRIPTS := \
	build.sh \
	go/tools/rbdecompress/c/build.sh \
	go/tools/shortv3derive/recover.sh \
	go/tools/compile-ghidra-scripts.sh \
	decompiled/ghidra_scripts/build_unpacked_project.sh \
	decompiled/analysis/lemonade2_static_unpacking/workflow/capture.sh

PS_SCRIPTS := \
	build.ps1 \
	decompiled/analysis/lemonade2_static_unpacking/workflow/remote.ps1

PS_SCRIPT_LIST := "build.ps1","decompiled/analysis/lemonade2_static_unpacking/workflow/remote.ps1"

.DEFAULT_GOAL := help

.PHONY: help all build build-sh build-ps lint lint-go lint-web lint-shell lint-powershell lint-python lint-c lint-cpp check check-go check-web check-shell check-powershell check-submakes check-c-tools check-decompiled-c checks test tools submakes recover-shortv3 $(SUBMAKE_DIRS) clean clean-submakes list-targets

help:
	@printf 'LSX Server build targets\n\n'
	@printf '  make build            Build release binaries with auto-selected script (%s)\n' '$(BUILD_BACKEND)'
	@printf '  make build-sh         Build release binaries with build.sh\n'
	@printf '  make build-ps         Build release binaries with build.ps1 via %s\n' '$(POWERSHELL)'
	@printf '  make lint             Lint/static-check all language projects\n'
	@printf '  make check            Run lint plus tests/compile checks for all projects\n'
	@printf '  make all              Build root release binaries and nested Makefile projects\n'
	@printf '  make checks           Run build.sh checks without rebuilding tools\n'
	@printf '  make test             Run build.sh tests/checks without rebuilding tools\n'
	@printf '  make tools            Build nested Makefile projects only\n'
	@printf '  make recover-shortv3  Re-derive Armadillo seeds and ShortV3 private exponent\n'
	@printf '  make clean            Remove dist outputs and clean nested Makefiles\n'
	@printf '  make list-targets     Print supported Go release targets\n\n'
	@printf 'Run lint/check from the Nix dev shell for pwsh, shellcheck, and shfmt:\n'
	@printf '  nix develop -c make lint\n'
	@printf '  nix develop -c make check\n\n'
	@printf 'Pass script options with BUILD_ARGS, for example:\n'
	@printf '  make build BUILD_ARGS="--target linux/amd64 --skip-checks"\n'
	@printf '  make build-ps BUILD_ARGS="-Target windows/amd64 -SkipChecks"\n'

all: build submakes

build:
	$(BUILD_CMD) $(BUILD_ARGS)

build-sh:
	$(BUILD_SH_CMD) $(BUILD_ARGS)

build-ps:
	$(BUILD_PS_CMD) $(BUILD_ARGS)

lint: lint-go lint-web lint-shell lint-powershell lint-python lint-c lint-cpp

lint-go:
	@unformatted=$$(find go -name '*.go' -not -path '*/vendor/*' -print0 | xargs -0 gofmt -l); \
	test -z "$$unformatted" || { \
		printf 'gofmt required for:\n'; \
		printf '%s\n' "$$unformatted"; \
		exit 1; \
	}
	cd go && golangci-lint run ./...
	cd go && golangci-lint run --enable=modernize --enable=usestdlibvars --enable=exptostd --enable=prealloc --enable=perfsprint --enable=gocritic ./...

lint-web:
	npm ci
	npm run lint
	npm --prefix go/web run check

lint-shell:
	$(SHELLCHECK) $(SH_SCRIPTS)
	@diff=$$($(SHFMT) -d -ln bash $(SH_SCRIPTS)); \
	test -z "$$diff" || { \
		printf '%s\n' "$$diff"; \
		exit 1; \
	}

lint-powershell:
	$(POWERSHELL) -NoLogo -NoProfile -Command 'Import-Module PSScriptAnalyzer; $$result = Invoke-ScriptAnalyzer -Path $(PS_SCRIPT_LIST); if ($$result) { $$result | Format-Table -AutoSize; exit 1 }'

lint-python:
	$(MAKE) -C decompiled/analysis/lemonade2_static_unpacking py-check

lint-c:
	@if command -v clang >/dev/null 2>&1; then \
		find decompiled/src -name '*.c' -print0 | xargs -0 -n 1 clang -fsyntax-only -Idecompiled/src -Idecompiled/src/common; \
		$(MAKE) -C decompiled/analysis/lemonade2_static_unpacking c-check; \
	else \
		printf 'skip lint-c: clang not found\n'; \
	fi

lint-cpp:
	$(MAKE) -C go/tools/keyvis_search

check: lint check-go check-web check-shell check-powershell check-submakes check-c-tools check-decompiled-c

check-go:
	CGO_ENABLED=$$(go env CGO_ENABLED) go -C go vet ./...
	CGO_ENABLED=$$(go env CGO_ENABLED) go -C go test ./...
	CGO_ENABLED=1 go -C go run ./tools/lt2findings -check

check-web:
	npm --prefix go/web run generate:openapi
	npm --prefix go/web run build

check-shell:
	@for script in $(SH_SCRIPTS); do \
		bash -n $$script; \
	done

check-powershell:
	$(POWERSHELL) -NoLogo -NoProfile -Command '$$tokens = $$null; foreach ($$path in @($(PS_SCRIPT_LIST))) { $$errors = $$null; $$null = [System.Management.Automation.Language.Parser]::ParseFile($$path, [ref]$$tokens, [ref]$$errors); if ($$errors) { $$errors | ForEach-Object { Write-Error $$_ }; exit 1 } }'

check-submakes:
	$(MAKE) -C go/tools/keyvis_search smoke
	$(MAKE) -C decompiled/analysis/lemonade2_static_unpacking check

check-c-tools:
	cd go/tools/rbdecompress/c && ./build.sh

check-decompiled-c:
	@if command -v clang >/dev/null 2>&1; then \
		find decompiled/src -name '*.c' -print0 | xargs -0 -n 1 clang -fsyntax-only -Idecompiled/src -Idecompiled/src/common; \
	else \
		printf 'skip check-decompiled-c: clang not found\n'; \
	fi

checks:
	$(BASH_BUILD) --skip-tools $(BUILD_ARGS)

test:
	$(BASH_BUILD) --skip-tools $(BUILD_ARGS)

tools submakes: $(SUBMAKE_DIRS)

$(SUBMAKE_DIRS):
	$(MAKE) -C $@

recover-shortv3:
	bash go/tools/shortv3derive/recover.sh

clean: clean-submakes
	rm -rf dist

clean-submakes:
	@for dir in $(SUBMAKE_DIRS); do \
		$(MAKE) -C $$dir clean; \
	done

list-targets:
	$(BASH_BUILD) --list-targets
