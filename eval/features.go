package eval

import (
	"fmt"
	"strings"

	"jview/engine"
	"jview/protocol"
)

// Feature defines a feature that can be extracted from prompt text and verified against a session.
type Feature struct {
	Name     string
	Keywords []string // any of these in prompt text triggers this feature
	Verify   func(sess *engine.Session) (bool, string)
}

// ExtractFeatures returns the set of features detected in the prompt text.
func ExtractFeatures(promptText string) []Feature {
	lower := strings.ToLower(promptText)
	var matched []Feature
	for _, f := range allFeatures {
		for _, kw := range f.Keywords {
			if strings.Contains(lower, strings.ToLower(kw)) {
				matched = append(matched, f)
				break
			}
		}
	}
	return matched
}

// VerifyFeatures checks each feature against the session state.
func VerifyFeatures(features []Feature, sess *engine.Session) []FeatureResult {
	results := make([]FeatureResult, len(features))
	for i, f := range features {
		satisfied, evidence := f.Verify(sess)
		results[i] = FeatureResult{
			Feature:   f.Name,
			Satisfied: satisfied,
			Evidence:  evidence,
		}
	}
	return results
}

// allFeatures defines the known feature patterns.
var allFeatures = []Feature{
	{
		Name:     "SplitView layout",
		Keywords: []string{"SplitView", "three-pane", "3-pane", "split view"},
		Verify: func(sess *engine.Session) (bool, string) {
			return findComponentOfType(sess, protocol.CompSplitView)
		},
	},
	{
		Name:     "OutlineView",
		Keywords: []string{"OutlineView", "folder tree", "outline view", "sidebar tree"},
		Verify: func(sess *engine.Session) (bool, string) {
			return findComponentOfType(sess, protocol.CompOutlineView)
		},
	},
	{
		Name:     "RichTextEditor",
		Keywords: []string{"RichTextEditor", "rich text", "rich editor", "formatted text"},
		Verify: func(sess *engine.Session) (bool, string) {
			return findComponentOfType(sess, protocol.CompRichTextEditor)
		},
	},
	{
		Name:     "SearchField",
		Keywords: []string{"search", "SearchField"},
		Verify: func(sess *engine.Session) (bool, string) {
			found, evidence := findComponentOfType(sess, protocol.CompSearchField)
			if !found {
				return false, "no SearchField component found"
			}
			// Also check for /searchQuery in data model
			for _, sid := range sess.SurfaceIDs() {
				surf := sess.GetSurface(sid)
				if surf == nil {
					continue
				}
				if _, ok := surf.DM().Get("/searchQuery"); ok {
					return true, evidence + "; /searchQuery in data model"
				}
			}
			return true, evidence + "; /searchQuery not in data model"
		},
	},
	{
		Name:     "Toolbar",
		Keywords: []string{"toolbar", "NSToolbar"},
		Verify: func(sess *engine.Session) (bool, string) {
			// Can't check updateToolbar messages from session alone,
			// but we can check for toolbar callback registrations
			// This is a best-effort check
			return false, "toolbar verification requires JSONL scanning (not session-only)"
		},
	},
	{
		Name:     "Context menu",
		Keywords: []string{"context menu", "right-click menu"},
		Verify: func(sess *engine.Session) (bool, string) {
			for _, sid := range sess.SurfaceIDs() {
				surf := sess.GetSurface(sid)
				if surf == nil {
					continue
				}
				for _, compID := range surf.Tree().All() {
					comp, ok := surf.Tree().Get(compID)
					if !ok {
						continue
					}
					if comp.Props.ContextMenu != nil {
						return true, fmt.Sprintf("component %s has contextMenu", compID)
					}
				}
			}
			return false, "no component with contextMenu prop found"
		},
	},
	{
		Name:     "Delete shortcut",
		Keywords: []string{"Backspace", "Delete key", "delete shortcut"},
		Verify: func(sess *engine.Session) (bool, string) {
			// Check for keyEquivalent containing backspace (\b) in JSONL
			// Session doesn't retain menu data, so this is best-effort
			return false, "delete shortcut verification requires JSONL scanning"
		},
	},
}

// findComponentOfType searches all surfaces for a component of the given type.
func findComponentOfType(sess *engine.Session, compType protocol.ComponentType) (bool, string) {
	for _, sid := range sess.SurfaceIDs() {
		surf := sess.GetSurface(sid)
		if surf == nil {
			continue
		}
		for _, compID := range surf.Tree().All() {
			comp, ok := surf.Tree().Get(compID)
			if !ok {
				continue
			}
			if comp.Type == compType {
				return true, fmt.Sprintf("found %s component: %s", compType, compID)
			}
		}
	}
	return false, fmt.Sprintf("no %s component found", compType)
}

// ExtractFeaturesFromJSONL supplements feature verification by scanning raw JSONL
// for message types that can't be detected from session state alone.
func ExtractFeaturesFromJSONL(jsonlContent string) map[string]bool {
	found := map[string]bool{}
	for _, line := range strings.Split(jsonlContent, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.Contains(line, `"updateToolbar"`) {
			found["toolbar"] = true
		}
		if strings.Contains(line, `"updateMenu"`) {
			found["menu"] = true
		}
		if strings.Contains(line, `\\b`) || strings.Contains(line, `\b`) {
			found["backspace_key"] = true
		}
		if strings.Contains(line, `"contextMenu"`) {
			found["context_menu"] = true
		}
	}
	return found
}
