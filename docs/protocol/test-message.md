---
layout: default
title: test
parent: Protocol Reference
nav_order: 22
---

# test

Defines an inline test with assertions and event simulation. Test messages are interleaved in JSONL files and run by the native test runner.

## Example

```json
{
  "type": "test",
  "surfaceId": "main",
  "name": "contact form renders",
  "steps": [
    {
      "assert": "component",
      "componentId": "title",
      "props": {
        "content": "Contact Us"
      }
    },
    {
      "assert": "dataModel",
      "path": "/name",
      "value": ""
    },
    {
      "simulate": "event",
      "componentId": "nameField",
      "event": "change",
      "eventData": "Alice"
    },
    {
      "assert": "dataModel",
      "path": "/name",
      "value": "Alice"
    },
    {
      "assert": "component",
      "componentId": "previewName",
      "props": {
        "content": "Alice"
      }
    }
  ]
}
```

## Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | yes | `"test"` |
| `surfaceId` | string | yes | Surface to test |
| `name` | string | yes | Test name (for reporting) |
| `steps` | array | yes | Ordered sequence of assertions and simulations |

## Assertion Types

### component

Check resolved props of a component (subset match):

```json
{
  "assert": "component",
  "componentId": "title",
  "props": {
    "content": "Hello"
  }
}
```

Optionally check the component type:

```json
{
  "assert": "component",
  "componentId": "title",
  "componentType": "Text",
  "props": {
    "content": "Hello"
  }
}
```

### dataModel

Check a value in the data model:

```json
{
  "assert": "dataModel",
  "path": "/count",
  "value": 5
}
```

### children

Check a component's child IDs:

```json
{
  "assert": "children",
  "componentId": "row",
  "children": [
    "a",
    "b",
    "c"
  ]
}
```

### notExists

Assert a component does not exist:

```json
{
  "assert": "notExists",
  "componentId": "deleted"
}
```

### count

Assert the number of children:

```json
{
  "assert": "count",
  "componentId": "list",
  "count": 3
}
```

### action

Assert that an action was fired:

```json
{
  "assert": "action",
  "name": "submitForm",
  "data": {
    "name": "Alice"
  }
}
```

### layout

Check computed NSView frame properties:

```json
{
  "assert": "layout",
  "componentId": "box",
  "layout": {
    "width": 200,
    "height": 100
  }
}
```

Available properties: `x`, `y`, `width`, `height`.

### style

Check computed visual properties:

```json
{
  "assert": "style",
  "componentId": "title",
  "style": {
    "fontSize": 24,
    "fontWeight": "bold"
  }
}
```

Available properties: `fontSize`, `fontWeight`, `textColor`, `backgroundColor`, `opacity`.

## Event Simulation

### simulate: event

Simulate user interaction on a component:

```json
{
  "simulate": "event",
  "componentId": "nameField",
  "event": "change",
  "eventData": "Alice"
}
```

Event types:

| Event | Component | eventData |
|-------|-----------|-----------|
| `change` | TextField, SearchField | New text value |
| `click` | Button | -- |
| `toggle` | CheckBox | -- |
| `slide` | Slider | New numeric value (as string) |
| `select` | ChoicePicker | Selected value |
| `datechange` | DateTimeInput | ISO date string |

## Running Tests

From the command line:
```bash
build/canopy test testdata/contact_form_test.jsonl
```

Or as a Go test in `engine/testrunner_test.go`:
```go
func TestContactForm(t *testing.T) {
    RunTestFile(t, "testdata/contact_form_test.jsonl")
}
```

## Related

- [updateComponents](update-components) -- set up the UI before testing
- [updateDataModel](update-data-model) -- initialize state before testing
