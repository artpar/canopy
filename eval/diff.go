package eval

import "jview/engine"

// CompareStructure compares two surfaces' component trees and returns a diff.
// refSurf is the reference (hand-crafted), genSurf is the generated output.
// Either may be nil; if refSurf is nil, only component coverage is computed.
func CompareStructure(refSurf, genSurf *engine.Surface) *StructuralDiff {
	if refSurf == nil || genSurf == nil {
		return nil
	}

	refIDs := makeSet(refSurf.Tree().All())
	genIDs := makeSet(genSurf.Tree().All())

	// Filter out internal IDs (prefixed with __ or _computed)
	filterInternal := func(s map[string]bool) map[string]bool {
		out := make(map[string]bool, len(s))
		for id := range s {
			if len(id) >= 2 && id[:2] == "__" {
				continue
			}
			out[id] = true
		}
		return out
	}
	refIDs = filterInternal(refIDs)
	genIDs = filterInternal(genIDs)

	diff := &StructuralDiff{}

	// Missing: in ref but not in gen
	for id := range refIDs {
		if !genIDs[id] {
			diff.MissingComponents = append(diff.MissingComponents, id)
		}
	}

	// Extra: in gen but not in ref
	for id := range genIDs {
		if !refIDs[id] {
			diff.ExtraComponents = append(diff.ExtraComponents, id)
		}
	}

	// For matching IDs: check type and children
	for id := range refIDs {
		if !genIDs[id] {
			continue
		}
		refComp, refOK := refSurf.Tree().Get(id)
		genComp, genOK := genSurf.Tree().Get(id)
		if !refOK || !genOK {
			continue
		}

		// Type mismatch
		if refComp.Type != genComp.Type {
			diff.TypeMismatches = append(diff.TypeMismatches, TypeMismatch{
				ComponentID: id,
				RefType:     string(refComp.Type),
				GenType:     string(genComp.Type),
			})
		}

		// Children diff
		refChildren := refSurf.Tree().Children(id)
		genChildren := genSurf.Tree().Children(id)
		if !slicesEqual(refChildren, genChildren) {
			diff.ChildrenDiffs = append(diff.ChildrenDiffs, ChildrenDiff{
				ComponentID: id,
				RefChildren: refChildren,
				GenChildren: genChildren,
			})
		}
	}

	// Jaccard similarity: |intersection| / |union|
	intersection := 0
	for id := range refIDs {
		if genIDs[id] {
			intersection++
		}
	}
	union := len(refIDs) + len(genIDs) - intersection
	if union > 0 {
		diff.Similarity = float64(intersection) / float64(union)
	}

	return diff
}

// CompareDataModel compares data model values at key paths between ref and gen.
// Returns a score (0.0 to 1.0) of matching paths.
func CompareDataModel(refSurf, genSurf *engine.Surface, paths []string) float64 {
	if refSurf == nil || genSurf == nil || len(paths) == 0 {
		return 1.0 // no reference means no penalty
	}

	matching := 0
	for _, path := range paths {
		refVal, refOK := refSurf.DM().Get(path)
		genVal, genOK := genSurf.DM().Get(path)
		if !refOK && !genOK {
			matching++
			continue
		}
		if refOK && genOK && valuesMatch(refVal, genVal) {
			matching++
		}
	}
	return float64(matching) / float64(len(paths))
}

func makeSet(items []string) map[string]bool {
	s := make(map[string]bool, len(items))
	for _, item := range items {
		s[item] = true
	}
	return s
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// valuesMatch loosely compares two data model values.
// For slices, checks length equality. For maps, checks key sets match.
// For primitives, compares directly.
func valuesMatch(a, b interface{}) bool {
	switch av := a.(type) {
	case []interface{}:
		bv, ok := b.([]interface{})
		if !ok {
			return false
		}
		return len(av) == len(bv)
	case map[string]interface{}:
		bv, ok := b.(map[string]interface{})
		if !ok {
			return false
		}
		if len(av) != len(bv) {
			return false
		}
		for k := range av {
			if _, ok := bv[k]; !ok {
				return false
			}
		}
		return true
	default:
		return a == b
	}
}
