---
layout: default
title: Arrays
parent: Functions
nav_order: 4
---

# Array Functions

Functions for manipulating arrays (JSON arrays stored in the data model).

## Reference

| Function | Args | Returns | Description |
|----------|------|---------|-------------|
| `append` | `array, element` | array | Append element to array |
| `removeLast` | `array` | array | Remove last element |
| `slice` | `array, start, end?` | array | Extract sub-array (start to end, exclusive) |
| `filter` | `array, key, value` | array | Items where `item[key] == value` |
| `filterContains` | `array, key, substring` | array | Items where `item[key]` contains substring (case-insensitive) |
| `filterContainsAny` | `array, keys, substring` | array | Items where any of the listed keys contains substring (case-insensitive) |
| `find` | `array, key, value` | object | First item where `item[key] == value` |
| `sort` | `array, key, descending?` | array | Sort by key; descending is optional (default false) |
| `remove` | `array, key, value` | array | Items where `item[key] != value` (inverse of filter) |
| `insertAt` | `array, index, item` | array | Insert item at index position |
| `length` | `array` | number | Number of elements (also works on strings) |
| `countWhere` | `array, key, value` | number | Count items where `item[key] == value` |

## Examples

### append

Add an item to an array:

```json
{
  "functionCall": {
    "name": "append",
    "args": [
      {
        "path": "/items"
      },
      {
        "title": "New Item",
        "done": false
      }
    ]
  }
}
```

Commonly used in an updateDataModel action:

```json
{
  "op": "replace",
  "path": "/items",
  "value": {
    "functionCall": {
      "name": "append",
      "args": [
        {
          "path": "/items"
        },
        {
          "title": "New",
          "done": false
        }
      ]
    }
  }
}
```

### filter

Get items matching a condition:

```json
{
  "functionCall": {
    "name": "filter",
    "args": [
      {
        "path": "/todos"
      },
      "done",
      false
    ]
  }
}
```

Returns all todo items where `done` is `false`.

### filterContains

Search by substring (case-insensitive):

```json
{
  "functionCall": {
    "name": "filterContains",
    "args": [
      {
        "path": "/contacts"
      },
      "name",
      {
        "path": "/searchQuery"
      }
    ]
  }
}
```

### filterContainsAny

Search across multiple fields:

```json
{
  "functionCall": {
    "name": "filterContainsAny",
    "args": [
      {
        "path": "/notes"
      },
      [
        "title",
        "body"
      ],
      {
        "path": "/searchQuery"
      }
    ]
  }
}
```

### find

Get the first matching item:

```json
{
  "functionCall": {
    "name": "find",
    "args": [
      {
        "path": "/users"
      },
      "id",
      {
        "path": "/selectedId"
      }
    ]
  }
}
```

### sort

Sort by a field:

```json
{
  "functionCall": {
    "name": "sort",
    "args": [
      {
        "path": "/items"
      },
      "createdAt",
      true
    ]
  }
}
```

The third argument (`true`) sorts in descending order.

### remove

Remove items matching a condition (opposite of filter):

```json
{
  "functionCall": {
    "name": "remove",
    "args": [
      {
        "path": "/items"
      },
      "id",
      {
        "path": "/deleteId"
      }
    ]
  }
}
```

### insertAt

Insert at a specific position:

```json
{
  "functionCall": {
    "name": "insertAt",
    "args": [
      {
        "path": "/items"
      },
      0,
      {
        "title": "First"
      }
    ]
  }
}
```

### countWhere

Count matching items:

```json
{
  "functionCall": {
    "name": "countWhere",
    "args": [
      {
        "path": "/todos"
      },
      "done",
      true
    ]
  }
}
```

### length

Get array length:

```json
{
  "functionCall": {
    "name": "length",
    "args": [
      {
        "path": "/items"
      }
    ]
  }
}
```
