BINARY    := jview
BUILD_DIR := build
SNAP_DIR  := $(BUILD_DIR)/screenshots
FIXTURES  := $(wildcard testdata/*.jsonl)
SNAP_WAIT := 2
GO        ?= $(shell command -v go1.25.0 2>/dev/null || echo go)

.PHONY: build test verify verify-fixture check clean

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

# ── Clean ───────────────────────────────────────────
clean:
	rm -rf $(BUILD_DIR)
