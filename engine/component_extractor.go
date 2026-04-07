package engine

import (
	"encoding/json"
	"fmt"
	"canopy/protocol"
	"os"
)

// ExtractComponent reads a cached JSONL file and extracts all components
// into a DefineComponent suitable for saving to the library.
// It strips surface-level messages (createSurface, setTheme, etc.) and
// rewrites the root component's ID to "_root" per defineComponent convention.
func ExtractComponent(jsonlPath, name string) (*protocol.DefineComponent, error) {
	file, err := os.Open(jsonlPath)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", jsonlPath, err)
	}
	defer file.Close()

	parser := protocol.NewParser(file)

	// Collect all components from updateComponents messages
	var allComps []protocol.Component
	for {
		msg, err := parser.Next()
		if err != nil {
			break
		}
		if msg == nil {
			break
		}
		if msg.Type == protocol.MsgUpdateComponents {
			uc := msg.Body.(protocol.UpdateComponents)
			allComps = append(allComps, uc.Components...)
		}
	}

	if len(allComps) == 0 {
		return nil, fmt.Errorf("no components found in %s", jsonlPath)
	}

	// Build a set of all IDs that are referenced as children
	childIDs := make(map[string]bool)
	for _, comp := range allComps {
		if comp.Children != nil {
			for _, cid := range comp.Children.Static {
				childIDs[cid] = true
			}
		}
	}

	// Find the root: a component whose ID is not a child of any other
	rootID := ""
	for _, comp := range allComps {
		if !childIDs[comp.ComponentID] {
			rootID = comp.ComponentID
			break
		}
	}
	if rootID == "" {
		rootID = allComps[0].ComponentID
	}

	// Rewrite root ID to _root and update child references
	for i := range allComps {
		if allComps[i].ComponentID == rootID {
			allComps[i].ComponentID = "_root"
		}
		if allComps[i].Children != nil {
			for j, cid := range allComps[i].Children.Static {
				if cid == rootID {
					allComps[i].Children.Static[j] = "_root"
				}
			}
		}
	}

	// Marshal each component to json.RawMessage
	rawComps := make([]json.RawMessage, len(allComps))
	for i, comp := range allComps {
		data, err := json.Marshal(comp)
		if err != nil {
			return nil, fmt.Errorf("marshal component %s: %w", comp.ComponentID, err)
		}
		rawComps[i] = data
	}

	return &protocol.DefineComponent{
		Type:       protocol.MsgDefineComponent,
		Name:       name,
		Components: rawComps,
	}, nil
}
