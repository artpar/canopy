package engine

import (
	"jview/protocol"
	"sort"
	"testing"
)

func TestTreeUpdateAndGet(t *testing.T) {
	tree := NewTree()

	tree.Update([]protocol.Component{
		{ComponentID: "a", Type: protocol.CompText},
		{ComponentID: "b", Type: protocol.CompButton},
	})

	a, ok := tree.Get("a")
	if !ok || a.Type != protocol.CompText {
		t.Errorf("Get(a) = %v, %v", a, ok)
	}

	_, ok = tree.Get("missing")
	if ok {
		t.Error("expected missing to not exist")
	}
}

func TestTreeRootIDs(t *testing.T) {
	tree := NewTree()

	tree.Update([]protocol.Component{
		{ComponentID: "root", Type: protocol.CompColumn, Children: &protocol.ChildList{Static: []string{"child1", "child2"}}},
		{ComponentID: "child1", Type: protocol.CompText},
		{ComponentID: "child2", Type: protocol.CompText},
	})

	roots := tree.RootIDs()
	if len(roots) != 1 || roots[0] != "root" {
		t.Errorf("roots = %v, want [root]", roots)
	}
}

func TestTreeMultipleRoots(t *testing.T) {
	tree := NewTree()

	tree.Update([]protocol.Component{
		{ComponentID: "a", Type: protocol.CompText},
		{ComponentID: "b", Type: protocol.CompText},
	})

	roots := tree.RootIDs()
	sort.Strings(roots)
	if len(roots) != 2 {
		t.Errorf("roots = %v, want 2 items", roots)
	}
}

func TestTreeChildren(t *testing.T) {
	tree := NewTree()

	tree.Update([]protocol.Component{
		{ComponentID: "parent", Type: protocol.CompColumn, Children: &protocol.ChildList{Static: []string{"c1", "c2"}}},
		{ComponentID: "c1", Type: protocol.CompText},
		{ComponentID: "c2", Type: protocol.CompText},
	})

	children := tree.Children("parent")
	if len(children) != 2 || children[0] != "c1" || children[1] != "c2" {
		t.Errorf("children = %v, want [c1 c2]", children)
	}

	children = tree.Children("c1")
	if len(children) != 0 {
		t.Errorf("leaf children = %v, want []", children)
	}
}

func TestTreeUpdateReturnsChanged(t *testing.T) {
	tree := NewTree()

	changed := tree.Update([]protocol.Component{
		{ComponentID: "a", Type: protocol.CompText},
		{ComponentID: "b", Type: protocol.CompText},
	})

	sort.Strings(changed)
	if len(changed) != 2 || changed[0] != "a" || changed[1] != "b" {
		t.Errorf("changed = %v, want [a b]", changed)
	}
}

func TestTreePruneNoOrphans(t *testing.T) {
	tree := NewTree()

	prevRoots := tree.RootIDs()
	changed := tree.Update([]protocol.Component{
		{ComponentID: "root", Type: protocol.CompColumn, Children: &protocol.ChildList{Static: []string{"c1", "c2"}}},
		{ComponentID: "c1", Type: protocol.CompText},
		{ComponentID: "c2", Type: protocol.CompText},
	})

	removed := tree.Prune(prevRoots, changed)
	if len(removed) != 0 {
		t.Errorf("removed = %v, want empty", removed)
	}

	// All components still present
	if _, ok := tree.Get("root"); !ok {
		t.Error("root missing after prune")
	}
	if _, ok := tree.Get("c1"); !ok {
		t.Error("c1 missing after prune")
	}
}

func TestTreePruneRemovesOrphans(t *testing.T) {
	tree := NewTree()

	// Initial tree: root → [c1, c2]
	tree.Update([]protocol.Component{
		{ComponentID: "root", Type: protocol.CompColumn, Children: &protocol.ChildList{Static: []string{"c1", "c2"}}},
		{ComponentID: "c1", Type: protocol.CompText},
		{ComponentID: "c2", Type: protocol.CompText},
	})

	// Update root to only have c1 as child — c2 becomes orphan
	prevRoots := tree.RootIDs()
	changed := tree.Update([]protocol.Component{
		{ComponentID: "root", Type: protocol.CompColumn, Children: &protocol.ChildList{Static: []string{"c1"}}},
	})

	removed := tree.Prune(prevRoots, changed)
	if len(removed) != 1 || removed[0] != "c2" {
		t.Errorf("removed = %v, want [c2]", removed)
	}

	// c2 should be gone
	if _, ok := tree.Get("c2"); ok {
		t.Error("c2 still in tree after prune")
	}

	// c1 still present
	if _, ok := tree.Get("c1"); !ok {
		t.Error("c1 missing after prune")
	}
}

func TestTreePruneDeepOrphans(t *testing.T) {
	tree := NewTree()

	// root → [parent] → [child]
	tree.Update([]protocol.Component{
		{ComponentID: "root", Type: protocol.CompColumn, Children: &protocol.ChildList{Static: []string{"parent"}}},
		{ComponentID: "parent", Type: protocol.CompColumn, Children: &protocol.ChildList{Static: []string{"child"}}},
		{ComponentID: "child", Type: protocol.CompText},
	})

	// Remove parent from root's children — both parent and child become orphans
	prevRoots := tree.RootIDs()
	changed := tree.Update([]protocol.Component{
		{ComponentID: "root", Type: protocol.CompColumn, Children: &protocol.ChildList{Static: []string{}}},
	})

	removed := tree.Prune(prevRoots, changed)
	sort.Strings(removed)
	if len(removed) != 2 {
		t.Fatalf("removed = %v, want [child parent]", removed)
	}
	if removed[0] != "child" || removed[1] != "parent" {
		t.Errorf("removed = %v, want [child parent]", removed)
	}

	if _, ok := tree.Get("parent"); ok {
		t.Error("parent still in tree")
	}
	if _, ok := tree.Get("child"); ok {
		t.Error("child still in tree")
	}
}
