DOCKER ?= docker
# The tag follows the image inputs, so editing them rebuilds on the next call.
IMAGE_TAG := $(shell cat Dockerfile .dockerignore tools/go.mod tools/go.sum 2>/dev/null | cksum | cut -d' ' -f1)
IMAGE ?= fft-decomp-dev:$(IMAGE_TAG)
BEAR ?= bear
EDITOR_CC ?= clang
PCSX_REDUX ?= /Applications/PCSX-Redux.app/Contents/MacOS/PCSX-Redux

.DEFAULT_GOAL := help

HOST_USER := $(shell id -u):$(shell id -g)

# Optional tool overrides, passed only when given: JOBS (compile jobs, default
# one per CPU; permuter threads, default 4), CACHE=0 (recompile everything),
# WORK_DIR (per-function intermediates, default /tmp/fft-build in the
# container; empty keeps them under build/work), REPORT_ALL=1 (make
# patch-check lists every mismatching function) and DURATION (permuter
# seconds, default 300).
TOOL_ENV = $(if $(JOBS),--env TOOLS_JOBS=$(JOBS)) \
	$(if $(CACHE),--env TOOLS_CACHE=$(CACHE)) \
	$(if $(filter-out undefined,$(origin WORK_DIR)),--env TOOLS_WORK_DIR=$(WORK_DIR)) \
	$(if $(REPORT_ALL),--env TOOLS_REPORT_ALL_MISMATCHES=$(REPORT_ALL))

# EDITION=E builds an edition of target/editions.yaml (build, check, warnings,
# decl-check, diff): its options become -D flags and its outputs go under
# build/E. The commands that check the original disc refuse it.
EDITION_ARG = $(if $(strip $(EDITION)),--edition="$(EDITION)")
ORIGINAL_ONLY = @test -z "$(strip $(EDITION))" || { echo "make $@ works on the original disc only: drop EDITION=$(EDITION)"; exit 2; }

# Every command runs in a fresh container as the host user, so files written
# under the bind-mounted checkout keep the caller's ownership. --init forwards
# Ctrl-C and reaps the permuter's children. --log-driver=none keeps output on
# the terminal only; podman's journald default copies every line to the host.
# Rootless podman (also behind a podman-docker shim) needs the caller's uid
# kept inside the user namespace and the SELinux label check disabled, or the
# bind mount is unreadable.
DOCKER_IS_PODMAN := $(shell $(DOCKER) --version 2>/dev/null | grep -qi podman && echo 1)
DOCKER_RUN_FLAGS = $(if $(DOCKER_IS_PODMAN),--userns=keep-id --security-opt label=disable)
DOCKER_RUN = $(DOCKER) run --rm --init --pull=never --log-driver=none $(DOCKER_RUN_FLAGS) \
	--user "$(HOST_USER)" --volume "$(CURDIR):/work" $(strip $(TOOL_ENV))

# tools/ is rebuilt from the working tree on every call (incrementally, via
# build/cache/go), so edits apply on the next command. A container-local copy
# runs, since Docker Desktop can report a just-written bind-mounted binary busy.
TOOLS = $(DOCKER_RUN) $(IMAGE) sh -c 'go build -C tools -o ../build/bin/tools . && cp build/bin/tools /tmp/tools && exec /tmp/tools "$$@"' tools

# clang-format comes from the image, so every host formats identically.
FORMAT = $(DOCKER_RUN) $(IMAGE) bash -o pipefail -c 'find src include -type f \( -name "*.c" -o -name "*.h" \) -print0 | sort -z | xargs -0 -r clang-format --style=file "$$@"' clang-format

.PHONY: help
help: ## Show this help
	@printf 'Final Fantasy Tactics matching decompilation\n\n'
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-18s %s\n", $$1, $$2}'
	@printf '\nFUNC is looked up in target/*.yaml; MODULE=id picks one when several define it.\n'

.PHONY: _docker-build
_docker-build:
	$(DOCKER) build --tag $(IMAGE) .

# Build the image only when it is missing; `make bootstrap` always rebuilds.
.PHONY: image
image:
	@$(DOCKER) image inspect "$(IMAGE)" >/dev/null 2>&1 || $(MAKE) --no-print-directory _docker-build

.PHONY: image-name
image-name:
	@echo "$(IMAGE)"

.PHONY: bootstrap
bootstrap: _docker-build ## Rebuild the Docker image, check-config, and extract scus-94221.bin if present
	@$(TOOLS) check-config
	@if [ -f scus-94221.bin ]; then $(MAKE) --no-print-directory extract; \
	else echo 'No scus-94221.bin: skipped extraction. `make check` and `make build` (function hashes, no disc) work without it.'; fi

.PHONY: extract
extract: image ## Extract scus-94221.bin (automatic for build, patch-check and checksums)
	@$(TOOLS) extract

.PHONY: build
build: image ## Link each module whole from its objects and the bytes between them, taking each name another module places from that module's link, compare with the original (function hashes without the BIN), fail on an image past its RAM window and print each window's room; every module with the BIN also builds the disc in build/disc (MODULE=x); SHIFT=N links a test disc with every module N bytes up in build/shift and checks every word against the original (needs the BIN); EDITION=E builds that edition of target/editions.yaml in build/E
	@if [ -f scus-94221.bin ]; then $(MAKE) --no-print-directory extract; fi
	@$(TOOLS) build $(if $(strip $(MODULE)),--module="$(MODULE)") $(if $(strip $(SHIFT)),--shift="$(SHIFT)") $(EDITION_ARG)

.PHONY: patch-check
patch-check: extract ## Cross-check make build the old way: compile each function alone over a copy of its original module image, require every copy identical and the disc from them in build/patch/disc exact (MODULE=x)
	$(ORIGINAL_ONLY)
	@$(TOOLS) patch-check $(if $(strip $(MODULE)),--module="$(MODULE)")

.PHONY: check
check: image ## Compile and compare with target/ hashes, no BIN needed (MODULE=x, EDITION=E)
	@$(TOOLS) check $(if $(strip $(MODULE)),--module="$(MODULE)") $(EDITION_ARG)

# FUNC names a configured function; MODULE is needed only when it is ambiguous.
function_args = $(if $(strip $(MODULE)),--module="$(MODULE)") "$(FUNC)"

.PHONY: diff
diff: image ## Compare FUNC=name with the original bytes (needs the BIN)
	@test -n "$(FUNC)" || { echo "FUNC is required: make diff FUNC=<function> [MODULE=<id>]"; exit 2; }
	@$(TOOLS) diff $(EDITION_ARG) $(function_args)

.PHONY: library-diff
library-diff: image ## Compare a library candidate with original bytes (ARGS="--name=... --source=... --addr=... --size=...")
	$(ORIGINAL_ONLY)
	@$(TOOLS) library-diff $(ARGS)

.PHONY: permute
permute: image ## Run decomp-permuter on FUNC=name (DURATION=300 JOBS=4)
	$(ORIGINAL_ONLY)
	@test -n "$(FUNC)" || { echo "FUNC is required: make permute FUNC=<function> [MODULE=<id>] [DURATION=300] [JOBS=4]"; exit 2; }
	@$(TOOLS) permute $(if $(DURATION),--duration="$(DURATION)") $(function_args)

.PHONY: checksums
checksums: extract ## Record function hashes in target/ from the original bytes
	$(ORIGINAL_ONLY)
	@$(TOOLS) checksums

.PHONY: check-config
check-config: image ## Validate project metadata (target/, sources, declarations)
	@$(TOOLS) check-config

.PHONY: config-fmt
config-fmt: image ## Rewrite target/*.yaml in canonical form
	@$(TOOLS) config-fmt

.PHONY: decl-check
decl-check: image ## Fail on calls to undeclared functions (cc1 -Wimplicit, syntax only)
	@$(TOOLS) decl-check $(EDITION_ARG)

.PHONY: declarations
declarations: image ## Report conflicting file-scope declarations (ARGS="...")
	@$(TOOLS) declarations $(ARGS)

.PHONY: share-audit
share-audit: image ## Report shared C implementation candidates as JSON (MODULE=x, ARGS="...")
	@$(TOOLS) share-audit $(if $(strip $(MODULE)),--module="$(MODULE)") $(ARGS)

.PHONY: twins
twins: extract ## Group byte-equivalent functions across modules under consistent renames into build/analysis/twins (MODULE=x, ARGS="...")
	$(ORIGINAL_ONLY)
	@$(TOOLS) twins $(if $(strip $(MODULE)),--module="$(MODULE)") $(ARGS)

.PHONY: trial
trial: image ## Compile a candidate source at an entry and report the match or first difference (MODULE=x ENTRY=f SOURCE=path ARGS="--subst OLD=NEW", or BATCH=build/x.yaml, or TWINS=t0001)
	$(ORIGINAL_ONLY)
	@$(TOOLS) trial $(if $(strip $(BATCH)),--batch="$(BATCH)") $(if $(strip $(TWINS)),--twins="$(TWINS)") $(if $(strip $(MODULE)),--module="$(MODULE)") $(if $(strip $(ENTRY)),--entry="$(ENTRY)") $(if $(strip $(SOURCE)),--source="$(SOURCE)") $(ARGS)

.PHONY: reloc-check
reloc-check: extract ## Fail on data words that look like RAM addresses or disc sectors and are neither relocations, disc or ram words nor values, and on RAM addresses split over two instructions outside code; link each module twice shifted to check every changed word, and list the findings in build/reloc-check (MODULE=x)
	$(ORIGINAL_ONLY)
	@$(TOOLS) reloc-check $(if $(strip $(MODULE)),--module="$(MODULE)")

.PHONY: unit-check
unit-check: image ## Rebuild each original translation unit from its members' sources and compare (MODULE=x UNIT=name)
	$(ORIGINAL_ONLY)
	@$(TOOLS) unit-check $(if $(strip $(MODULE)),--module="$(MODULE)") $(if $(strip $(UNIT)),--unit="$(UNIT)")

.PHONY: symbols
symbols: image ## Rename a symbol: ACTION=rename-function|rename-global ARGS="--old A --new B"
	@test -n "$(ACTION)" || { echo "ACTION is required: rename-function or rename-global"; exit 2; }
	@$(TOOLS) symbols "$(ACTION)" $(ARGS)

# MAP_EXPORT copies the vault to a directory outside build/ (a Windows drive:
# Obsidian cannot watch a vault inside WSL), keeping that copy's own .obsidian/
# settings. The copy's docs/ pages, CODEBASE.md and hash manifest come back
# into build/map first, so edits made there get the same protection; only
# changed files are rewritten, so Obsidian re-reads only those.
.PHONY: map
map: image ## Generate the Obsidian codebase map in build/map (MAP_EXPORT=dir also copies it)
	$(ORIGINAL_ONLY)
	@if [ -n "$(MAP_EXPORT)" ] && [ -f "$(MAP_EXPORT)/.docs.sha256" ] && [ -d "$(MAP_EXPORT)/docs" ]; then \
		mkdir -p build/map/docs && \
		rsync -r --checksum --delete "$(MAP_EXPORT)/docs/" build/map/docs/ && \
		rsync -r --checksum --ignore-missing-args "$(MAP_EXPORT)/CODEBASE.md" "$(MAP_EXPORT)/.docs.sha256" build/map/ || exit 1; \
	fi
	@$(TOOLS) map $(ARGS)
	@if [ -n "$(MAP_EXPORT)" ]; then \
		if [ -d "$(MAP_EXPORT)" ] && [ -n "$$(ls -A "$(MAP_EXPORT)")" ] && [ ! -f "$(MAP_EXPORT)/Home.md" ]; then \
			echo "map: $(MAP_EXPORT) is not empty and holds no earlier map; not copying"; exit 1; \
		fi; \
		mkdir -p "$(MAP_EXPORT)" && \
		rsync -r --checksum --delete --exclude .obsidian build/map/ "$(MAP_EXPORT)/" && \
		rsync -a --ignore-existing build/map/.obsidian "$(MAP_EXPORT)/" && \
		echo "map: copied to $(MAP_EXPORT)"; \
	fi

.PHONY: warnings
warnings: image ## List cc1 -Wall warnings per function in build/warnings (MODULE=x)
	@$(TOOLS) warnings $(if $(strip $(MODULE)),--module="$(MODULE)") $(EDITION_ARG)

.PHONY: verify
verify: fmt-check check-config decl-check check ## Run the per-change checks: fmt-check, check-config, decl-check, check (MODULE=x)

.PHONY: test
test: image ## Vet and test the Go tooling
	@$(DOCKER_RUN) $(IMAGE) sh -c 'cd tools && go vet ./... && go test ./...'

# cc1 runs with -gcoff, so moving lines can change the bytes: `make check` after.
.PHONY: fmt
fmt: image ## clang-format every C source and header in place
	@$(FORMAT) -i

.PHONY: fmt-check
fmt-check: image ## Fail if any C source or header is not clang-formatted
	@$(FORMAT) --dry-run --Werror

.PHONY: run
run: extract ## Build the disc and launch PCSX-Redux (PCSX_REDUX=path; default is the macOS app)
	$(MAKE) --no-print-directory build MODULE= SHIFT=
	test -x "$(PCSX_REDUX)"
	"$(PCSX_REDUX)" \
		-iso "$(CURDIR)/build/disc/output-scus-94221.cue" \
		-run

.PHONY: smoke
smoke: ## Boot the original and the shifted test disc in PCSX-Redux and pass each session's checkpoints, after make build and make build SHIFT=16 (PCSX_REDUX=path; logs and screenshots in build/smoke)
	$(ORIGINAL_ONLY)
	test -x "$(PCSX_REDUX)"
	@$(TOOLS) smoke prepare
	PCSX_REDUX="$(PCSX_REDUX)" sh tools/smoke.sh
	@$(TOOLS) smoke report

.PHONY: shell
shell: image ## Open an interactive shell in the Docker image
	@$(DOCKER_RUN) --interactive --tty $(IMAGE) bash

# Runs on the host so paths point into this checkout. Clang rejects a few
# redeclarations GCC 2.6.3 accepts; those files are still recorded.
.PHONY: compile-commands
compile-commands: ## Create editor compilation metadata (host bear and clang)
	@command -v "$(BEAR)" >/dev/null || { echo "Bear is required (set BEAR=/path/to/bear)"; exit 2; }
	@command -v "$(EDITOR_CC)" >/dev/null || { echo "Clang is required (set EDITOR_CC=/path/to/clang)"; exit 2; }
	$(BEAR) --output compile_commands.json -- /bin/sh -c 'set -eu; find src -type f -name "*.c" -print | sort | while IFS= read -r source; do "$(EDITOR_CC)" -target mipsel-none-elf -march=mips1 -mabi=32 -msoft-float -fsyntax-only -undef -nostdinc -Iinclude -Wno-everything "$$source" >/dev/null 2>&1 || :; done'
