package engine

import "jview/protocol"

// Tree maintains the component hierarchy for a surface.
// Components are stored in a flat map; the tree structure is derived from ParentID + Children.
type Tree struct {
	components map[string]*protocol.Component
	rootIDs    []string // top-level components (no parent)
}

func NewTree() *Tree {
	return &Tree{
		components: make(map[string]*protocol.Component),
	}
}

// Update adds or replaces components. Returns the IDs of components that changed.
func (t *Tree) Update(comps []protocol.Component) []string {
	var changed []string
	for i := range comps {
		comp := &comps[i]
		t.components[comp.ComponentID] = comp
		changed = append(changed, comp.ComponentID)
	}
	t.rebuildRoots()
	return changed
}

// Get returns a component by ID.
func (t *Tree) Get(id string) (*protocol.Component, bool) {
	c, ok := t.components[id]
	return c, ok
}

// Children returns the child component IDs for a given component.
func (t *Tree) Children(id string) []string {
	comp, ok := t.components[id]
	if !ok || comp.Children == nil {
		return nil
	}
	return comp.Children.Static
}

// RootIDs returns the top-level component IDs.
func (t *Tree) RootIDs() []string {
	return t.rootIDs
}

// All returns all component IDs.
func (t *Tree) All() []string {
	ids := make([]string, 0, len(t.components))
	for id := range t.components {
		ids = append(ids, id)
	}
	return ids
}

// Prune removes orphaned components not reachable from valid roots.
// prevRootIDs are the root IDs from before the update that triggered this prune.
// batchIDs is the set of component IDs from the current update batch.
// A root is valid if it existed before the update or was part of the batch.
// Components that become roots solely because they were dropped from a parent's
// children list are considered orphans.
// Returns the IDs of removed components.
func (t *Tree) Prune(prevRootIDs []string, batchIDs []string) []string {
	prevRoots := make(map[string]bool, len(prevRootIDs))
	for _, id := range prevRootIDs {
		prevRoots[id] = true
	}
	batchSet := make(map[string]bool, len(batchIDs))
	for _, id := range batchIDs {
		batchSet[id] = true
	}

	// Valid roots: were roots before this update, or are current roots AND in the batch
	var validRoots []string
	for _, id := range t.rootIDs {
		if prevRoots[id] || batchSet[id] {
			validRoots = append(validRoots, id)
		}
	}

	// BFS from valid roots to find all reachable components
	reachable := make(map[string]bool, len(t.components))
	queue := make([]string, len(validRoots))
	copy(queue, validRoots)
	for _, id := range validRoots {
		reachable[id] = true
	}

	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		comp, ok := t.components[id]
		if !ok {
			continue
		}
		if comp.Children != nil {
			for _, childID := range comp.Children.Static {
				if !reachable[childID] {
					reachable[childID] = true
					queue = append(queue, childID)
				}
			}
		}
	}

	// Collect and remove orphans
	var removed []string
	for id := range t.components {
		if !reachable[id] {
			removed = append(removed, id)
		}
	}
	for _, id := range removed {
		delete(t.components, id)
	}

	// Rebuild roots after pruning
	if len(removed) > 0 {
		t.rebuildRoots()
	}

	return removed
}

// rebuildRoots recalculates which components are root-level.
// A root component either has no parentId or its parentId references a non-existent component.
func (t *Tree) rebuildRoots() {
	t.rootIDs = nil
	// Build set of IDs that are children of something
	childSet := make(map[string]bool)
	for _, comp := range t.components {
		if comp.Children != nil && comp.Children.Static != nil {
			for _, childID := range comp.Children.Static {
				childSet[childID] = true
			}
		}
	}

	for id := range t.components {
		if !childSet[id] {
			t.rootIDs = append(t.rootIDs, id)
		}
	}
}
