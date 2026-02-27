BINARY    := jview
BUILD_DIR := build
SNAP_DIR  := $(BUILD_DIR)/screenshots
FIXTURES  := $(wildcard testdata/*.jsonl)
SNAP_WAIT := 2
GO        ?= $(shell command -v go1.25.0 2>/dev/null || echo go)

.PHONY: build test verify verify-fixture check clean \
       generate-apps generate-app run-app regen-app clean-apps \
       eval-apps eval-notes eval-loop eval-aggregate eval-full

# ── Build ───────────────────────────────────────────
build:
	$(GO) build -o $(BUILD_DIR)/$(BINARY) .

# ── Test ────────────────────────────────────────────
# Headless unit + integration tests via mock renderer.
# No CGo, no AppKit, no display needed. Safe for CI.
test:
	$(GO) test ./protocol/ ./engine/ ./transport/ ./eval/ -v -count=1 -race

# ── Verify ──────────────────────────────────────────
# Build, launch every fixture, capture screenshot, kill.
# Requires macOS with a display. Screenshots land in build/screenshots/
verify: build
	@mkdir -p $(SNAP_DIR)
	@failed=0; \
	for f in $(FIXTURES); do \
		name=$$(basename $$f .jsonl); \
		echo "==> $$name"; \
		$(BUILD_DIR)/$(BINARY) $$f & pid=$$!; \
		sleep $(SNAP_WAIT); \
		wid=$$(swift -e 'import Foundation; import CoreGraphics; let pid = Int32(CommandLine.arguments[1])!; guard let info = CGWindowListCopyWindowInfo(.optionOnScreenOnly, kCGNullWindowID) as NSArray? else { exit(0) }; for case let w as NSDictionary in info { if let p = w["kCGWindowOwnerPID"] as? Int, p == Int(pid), let n = w["kCGWindowNumber"] as? Int, let ly = w["kCGWindowLayer"] as? Int, ly == 0 { print(n); break } }' $$pid); \
		if [ -n "$$wid" ]; then screencapture -x -o -l $$wid $(SNAP_DIR)/$$name.png; else echo "    WARN: no window found"; fi; \
		kill $$pid 2>/dev/null; wait $$pid 2>/dev/null; \
		if [ -f $(SNAP_DIR)/$$name.png ]; then \
			echo "    screenshot: $(SNAP_DIR)/$$name.png"; \
		else \
			echo "    FAIL: no screenshot captured"; \
			failed=1; \
		fi; \
	done; \
	if [ $$failed -eq 1 ]; then echo "\nSome fixtures failed."; exit 1; fi; \
	echo "\nAll fixtures verified. Screenshots in $(SNAP_DIR)/"

# Verify a single fixture: make verify-fixture F=testdata/hello.jsonl
verify-fixture: build
	@mkdir -p $(SNAP_DIR)
	@name=$$(basename $(F) .jsonl); \
	echo "==> $$name"; \
	$(BUILD_DIR)/$(BINARY) $(F) & pid=$$!; \
	sleep $(SNAP_WAIT); \
	wid=$$(swift -e 'import Foundation; import CoreGraphics; let pid = Int32(CommandLine.arguments[1])!; guard let info = CGWindowListCopyWindowInfo(.optionOnScreenOnly, kCGNullWindowID) as NSArray? else { exit(0) }; for case let w as NSDictionary in info { if let p = w["kCGWindowOwnerPID"] as? Int, p == Int(pid), let n = w["kCGWindowNumber"] as? Int, let ly = w["kCGWindowLayer"] as? Int, ly == 0 { print(n); break } }' $$pid); \
	if [ -n "$$wid" ]; then screencapture -x -o -l $$wid $(SNAP_DIR)/$$name.png; else echo "    WARN: no window found"; fi; \
	kill $$pid 2>/dev/null; wait $$pid 2>/dev/null; \
	echo "    screenshot: $(SNAP_DIR)/$$name.png"

# ── Check ───────────────────────────────────────────
# Full pipeline: headless tests first, then visual verification.
# This is the gate. Run before any commit.
check: test verify
	@echo "\n✓ All tests passed. All fixtures rendered. Review screenshots."

# ── Sample Apps ─────────────────────────────────────
SAMPLE_APPS := $(wildcard sample_apps/*/prompt.txt)

# Generate all sample apps (headless, no window)
generate-apps: build
	@for f in $(SAMPLE_APPS); do \
		name=$$(basename $$(dirname $$f)); \
		echo "==> generating $$name"; \
		$(BUILD_DIR)/$(BINARY) --prompt-file $$f --generate-only; \
	done

# Generate a single sample app: make generate-app A=calculator
generate-app: build
	$(BUILD_DIR)/$(BINARY) --prompt-file sample_apps/$(A)/prompt.txt --generate-only

# Run a sample app (opens window from cache or LLM): make run-app A=calculator
run-app: build
	$(BUILD_DIR)/$(BINARY) --prompt-file sample_apps/$(A)/prompt.txt

# Force-regenerate a sample app: make regen-app A=calculator
regen-app: build
	$(BUILD_DIR)/$(BINARY) --prompt-file sample_apps/$(A)/prompt.txt --regenerate --generate-only

# Remove all cached JSONL and hash files
clean-apps:
	rm -f sample_apps/*/*.jsonl sample_apps/*/*.jsonl.tmp sample_apps/*/.*.hash

# ── Eval ────────────────────────────────────────────
# Evaluate all cached sample apps against their references (where available)
eval-apps: build
	@for d in sample_apps/*/; do \
		name=$$(basename $$d); \
		gen=$$d/prompt.jsonl; \
		ref=sample_apps/notes/prompt.jsonl; \
		if [ ! -f "$$gen" ]; then echo "SKIP $$name (no prompt.jsonl)"; continue; fi; \
		echo "==> eval $$name"; \
		if [ "$$name" != "notes" ] && [ -f "$$ref" ]; then \
			$(BUILD_DIR)/$(BINARY) eval "$$gen" --ref "$$ref"; \
		else \
			$(BUILD_DIR)/$(BINARY) eval "$$gen"; \
		fi; \
	done

# Evaluate notes_llm against hand-crafted notes reference
eval-notes: build
	$(BUILD_DIR)/$(BINARY) eval sample_apps/notes_llm/prompt.jsonl --ref sample_apps/notes/prompt.jsonl

# Full inner loop for a single app: make eval-loop A=notes_llm
eval-loop: build
	$(BUILD_DIR)/$(BINARY) --prompt-file sample_apps/$(A)/prompt.txt --generate-only --regenerate \
		--eval-ref sample_apps/notes/prompt.jsonl --eval-max-attempts 3

# Cross-app aggregate analysis
eval-aggregate: build
	$(BUILD_DIR)/$(BINARY) eval --aggregate

# Full pipeline: generate all apps, evaluate, aggregate
eval-full: generate-apps eval-apps eval-aggregate

# ── Clean ───────────────────────────────────────────
clean:
	rm -rf $(BUILD_DIR)
