UV ?= uv
PYTHON ?= $(UV) run python
WORKFLOW := workflow
INSTRUMENTATION := instrumentation
PY_SOURCES := \
	lib/binary.py \
	lib/common.py \
	lib/capture.py \
	lib/generated_crypto.py \
	lib/mapper.py \
	lib/pe.py \
	lib/reporting.py \
	workflow/report.py \
	inspectors/payload/snapshots.py \
	inspectors/data/guards.py \
	inspectors/data/semantics.py \
	inspectors/generated/pe.py \
	inspectors/generated/timeline.py \
	inspectors/data1/streams.py \
	inspectors/pdata/tail.py \
	inspectors/mapper/validation.py \
	reconstruction/payload/candidate.py \
	reconstruction/generated/teas.py \
	reconstruction/adata/evctx.py \
	inspectors/child/sites.py \
	inspectors/seh/blocks.py \
	tests/test_lib_helpers.py

.PHONY: all iterate run build remote-build check lock sync py-check py-test sh-check c-check clean capture ghidra

all: check

iterate: check capture

run: iterate

build: remote-build

remote-build:
	LEMONADE2_BUILD_ONLY=1 $(WORKFLOW)/capture.sh

check: py-check py-test sh-check c-check

lock:
	$(UV) lock

sync:
	$(UV) sync

py-check:
	$(PYTHON) -m py_compile $(PY_SOURCES)

py-test:
	$(PYTHON) -m unittest discover -s tests -p 'test_*.py'

sh-check:
	bash -n $(WORKFLOW)/capture.sh

c-check:
	@if command -v clang >/dev/null 2>&1; then \
		clang -D__INTELLISENSE__ -fsyntax-only -Wall -Wextra $(INSTRUMENTATION)/api/hook.c; \
		clang -D__INTELLISENSE__ -fsyntax-only -Wall -Wextra $(INSTRUMENTATION)/api/launcher.c; \
	else \
		printf 'skip c-check: clang not found\n'; \
	fi

capture:
	$(WORKFLOW)/capture.sh

ghidra:
	../../ghidra_scripts/build_unpacked_project.sh

clean:
	rm -rf .venv ../../../.venv
