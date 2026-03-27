BINARY    := jview
BUILD_DIR := build
SNAP_DIR  := $(BUILD_DIR)/screenshots
FIXTURES  := $(wildcard testdata/*.jsonl)
SNAP_WAIT := 2
GO        ?= $(shell command -v go1.25.0 2>/dev/null || echo go)

.PHONY: build test verify verify-fixture check clean \
       generate-apps generate-app run-app regen-app clean-apps \
       app clean-app

# ── Build ───────────────────────────────────────────
build:
	$(GO) build -o $(BUILD_DIR)/$(BINARY) .

# ── Test ────────────────────────────────────────────
# Headless unit + integration tests via mock renderer.
# No CGo, no AppKit, no display needed. Safe for CI.
test:
	$(GO) test ./protocol/ ./engine/ ./transport/ -v -count=1 -race

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

# ── App Bundle ─────────────────────────────────────
APP_NAME   := jview.app
APP_DIR    := $(BUILD_DIR)/$(APP_NAME)
CONTENTS   := $(APP_DIR)/Contents
MACOS_DIR  := $(CONTENTS)/MacOS
RES_DIR    := $(CONTENTS)/Resources

# Build macOS .app bundle
app: build
	@mkdir -p $(MACOS_DIR) $(RES_DIR)
	cp $(BUILD_DIR)/$(BINARY) $(MACOS_DIR)/$(BINARY)
	cp packaging/Info.plist $(CONTENTS)/Info.plist
	@if [ -f packaging/AppIcon.icns ]; then \
		cp packaging/AppIcon.icns $(RES_DIR)/AppIcon.icns; \
	fi
	@echo "Built $(APP_DIR)"

clean-app:
	rm -rf $(APP_DIR)

# ── Clean ───────────────────────────────────────────
clean:
	rm -rf $(BUILD_DIR)
