package engine

import (
	"fmt"
	"strconv"
	"strings"
)

// DataModel is a JSON data store supporting JSON Pointer operations.
type DataModel struct {
	root interface{}
}

func NewDataModel() *DataModel {
	return &DataModel{root: map[string]interface{}{}}
}

// Get retrieves a value by JSON Pointer. "" returns the root.
func (dm *DataModel) Get(pointer string) (interface{}, bool) {
	if pointer == "" || pointer == "/" {
		return dm.root, true
	}
	tokens := parsePointer(pointer)
	return navigate(dm.root, tokens)
}

// Set sets a value at a JSON Pointer path, creating intermediate objects as needed.
// Returns the list of changed paths (the target path and any parent paths created).
func (dm *DataModel) Set(pointer string, value interface{}) ([]string, error) {
	if pointer == "" || pointer == "/" {
		dm.root = value
		return []string{"/"}, nil
	}

	tokens := parsePointer(pointer)
	changed := []string{pointer}

	parent := dm.root
	for i := 0; i < len(tokens)-1; i++ {
		next, ok := getChild(parent, tokens[i])
		if !ok {
			// Create intermediate map
			newMap := map[string]interface{}{}
			if err := setChild(&parent, tokens[i], newMap); err != nil {
				return nil, err
			}
			if i == 0 {
				dm.root = parent
			}
			next = newMap
			changed = append(changed, "/"+strings.Join(tokens[:i+1], "/"))
		}
		parent = next
	}

	lastToken := tokens[len(tokens)-1]
	if err := setChild(&parent, lastToken, value); err != nil {
		return nil, err
	}
	if len(tokens) == 1 {
		dm.root = parent
	}

	return changed, nil
}

// Delete removes a value at a JSON Pointer path.
func (dm *DataModel) Delete(pointer string) ([]string, error) {
	if pointer == "" || pointer == "/" {
		dm.root = map[string]interface{}{}
		return []string{"/"}, nil
	}

	tokens := parsePointer(pointer)

	// For single-token paths, operate directly on root
	if len(tokens) == 1 {
		if err := deleteChild(&dm.root, tokens[0]); err != nil {
			return nil, nil
		}
		return []string{pointer}, nil
	}

	// Walk to the parent of the target
	parent := dm.root
	for i := 0; i < len(tokens)-2; i++ {
		next, ok := getChild(parent, tokens[i])
		if !ok {
			return nil, nil
		}
		parent = next
	}

	// We need a pointer to the penultimate container so deleteChild can resize slices
	penultimateToken := tokens[len(tokens)-2]
	lastToken := tokens[len(tokens)-1]

	// Get the direct parent container via pointer
	target, ok := getChild(parent, penultimateToken)
	if !ok {
		return nil, nil
	}

	if err := deleteChild(&target, lastToken); err != nil {
		return nil, nil
	}

	// Write the possibly-resized container back
	setChild(&parent, penultimateToken, target)

	return []string{pointer}, nil
}

func deleteChild(parent *interface{}, token string) error {
	switch p := (*parent).(type) {
	case map[string]interface{}:
		delete(p, token)
		return nil
	case []interface{}:
		idx, err := strconv.Atoi(token)
		if err != nil || idx < 0 || idx >= len(p) {
			return fmt.Errorf("invalid index")
		}
		*parent = append(p[:idx], p[idx+1:]...)
		return nil
	default:
		return fmt.Errorf("not a container")
	}
}

// parsePointer splits "/a/b/c" into ["a", "b", "c"].
func parsePointer(pointer string) []string {
	if pointer == "" {
		return nil
	}
	// Strip leading /
	if pointer[0] == '/' {
		pointer = pointer[1:]
	}
	parts := strings.Split(pointer, "/")
	// Unescape JSON Pointer: ~1 → /, ~0 → ~
	for i, p := range parts {
		p = strings.ReplaceAll(p, "~1", "/")
		p = strings.ReplaceAll(p, "~0", "~")
		parts[i] = p
	}
	return parts
}

func navigate(root interface{}, tokens []string) (interface{}, bool) {
	current := root
	for _, token := range tokens {
		child, ok := getChild(current, token)
		if !ok {
			return nil, false
		}
		current = child
	}
	return current, true
}

func getChild(parent interface{}, token string) (interface{}, bool) {
	switch p := parent.(type) {
	case map[string]interface{}:
		v, ok := p[token]
		return v, ok
	case []interface{}:
		idx, err := strconv.Atoi(token)
		if err != nil || idx < 0 || idx >= len(p) {
			return nil, false
		}
		return p[idx], true
	default:
		return nil, false
	}
}

func setChild(parent *interface{}, token string, value interface{}) error {
	switch p := (*parent).(type) {
	case map[string]interface{}:
		p[token] = value
		return nil
	case []interface{}:
		if token == "-" {
			// JSON Patch: "-" means append to end of array
			*parent = append(p, value)
			return nil
		}
		idx, err := strconv.Atoi(token)
		if err != nil {
			return fmt.Errorf("cannot index array with %q", token)
		}
		if idx < 0 {
			return fmt.Errorf("negative index %d", idx)
		}
		for idx >= len(p) {
			p = append(p, nil)
		}
		p[idx] = value
		*parent = p
		return nil
	default:
		// Create map
		m := map[string]interface{}{token: value}
		*parent = m
		return nil
	}
}
