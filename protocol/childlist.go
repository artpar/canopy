package protocol

import "encoding/json"

// ChildList is either a static list of component IDs or a template for dynamic expansion.
type ChildList struct {
	Static   []string
	Template *ChildTemplate
}

// ChildTemplate defines dynamic children generated from a data model array.
type ChildTemplate struct {
	ForEach      string        `json:"forEach"`      // JSON Pointer to array in data model
	ForEachFunc  *FunctionCall `json:"-"`             // Function call that returns an array
	TemplateID   string        `json:"templateId"`    // component ID to use as template
	ItemVariable string        `json:"itemVariable"`  // variable name for each item
}

func (c *ChildList) UnmarshalJSON(data []byte) error {
	// Try static array of strings first: ["a", "b"]
	var arr []string
	if err := json.Unmarshal(data, &arr); err == nil {
		c.Static = arr
		return nil
	}

	// Try object format: {"static": [...]} or {"forEach": ...}
	var obj struct {
		Static       []string        `json:"static"`
		ForEach      json.RawMessage `json:"forEach"`
		TemplateID   string          `json:"templateId"`
		ItemVariable string          `json:"itemVariable"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	if len(obj.ForEach) > 0 {
		tmpl := &ChildTemplate{
			TemplateID:   obj.TemplateID,
			ItemVariable: obj.ItemVariable,
		}
		// Try as string first (backward-compatible path)
		var forEachStr string
		if err := json.Unmarshal(obj.ForEach, &forEachStr); err == nil {
			tmpl.ForEach = forEachStr
		} else {
			// Try as object: {"functionCall": ...} or {"path": ...}
			var fcObj struct {
				FunctionCall *FunctionCall `json:"functionCall"`
				Path         string        `json:"path"`
			}
			if err := json.Unmarshal(obj.ForEach, &fcObj); err == nil {
				if fcObj.FunctionCall != nil {
					tmpl.ForEachFunc = fcObj.FunctionCall
				} else if fcObj.Path != "" {
					tmpl.ForEach = fcObj.Path
				}
			}
		}
		c.Template = tmpl
	} else if len(obj.Static) > 0 {
		c.Static = obj.Static
	}
	return nil
}

func (c ChildList) MarshalJSON() ([]byte, error) {
	if c.Template != nil {
		return json.Marshal(c.Template)
	}
	return json.Marshal(c.Static)
}
