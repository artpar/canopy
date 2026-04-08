---
layout: default
title: Trees
parent: Functions
nav_order: 7
---

# Tree Functions

Functions for manipulating hierarchical (tree-structured) data. Trees are arrays of objects where each object can have a `children` array containing nested items of the same structure.

## Reference

| Function | Args | Returns | Description |
|----------|------|---------|-------------|
| `appendToTree` | `tree, parentId, item` | array | Insert item as child of the node with matching ID |
| `removeFromTree` | `tree, id` | array | Remove the node with matching ID (searches recursively) |

## Tree Structure

Tree functions expect data in this format:

```json
[
  {
    "id": "root",
    "name": "Root",
    "children": [
      {
        "id": "child1",
        "name": "Child 1",
        "children": []
      },
      {
        "id": "child2",
        "name": "Child 2",
        "children": [
          {
            "id": "grandchild1",
            "name": "Grandchild 1",
            "children": []
          }
        ]
      }
    ]
  }
]
```

Each node must have an `id` field and a `children` array.

## Examples

### appendToTree

Add a new node as a child of an existing node:

```json
{
  "functionCall": {
    "name": "appendToTree",
    "args": [
      {
        "path": "/folders"
      },
      "child2",
      {
        "id": {
          "functionCall": {
            "name": "uuid",
            "args": []
          }
        },
        "name": "New Folder",
        "children": []
      }
    ]
  }
}
```

This inserts the new folder as a child of the node with `id` "child2".

If `parentId` is empty or null, the item is appended to the root array:

```json
{
  "functionCall": {
    "name": "appendToTree",
    "args": [
      {
        "path": "/folders"
      },
      "",
      {
        "id": "top-level",
        "name": "Top Level",
        "children": []
      }
    ]
  }
}
```

### removeFromTree

Remove a node by ID, searching the entire tree:

```json
{
  "functionCall": {
    "name": "removeFromTree",
    "args": [
      {
        "path": "/folders"
      },
      {
        "path": "/selectedFolderId"
      }
    ]
  }
}
```

The search is recursive -- the node is found and removed regardless of its depth in the tree.

## Common Pattern: Sidebar with OutlineView

Tree functions pair naturally with the [OutlineView](../components/outlineview) component:

```json
{
  "type": "updateDataModel",
  "surfaceId": "main",
  "ops": [
    {
      "op": "add",
      "path": "/folders",
      "value": [
        {
          "id": "inbox",
          "name": "Inbox",
          "icon": "tray",
          "children": []
        },
        {
          "id": "archive",
          "name": "Archive",
          "icon": "archivebox",
          "children": [
            {
              "id": "2024",
              "name": "2024",
              "children": []
            },
            {
              "id": "2025",
              "name": "2025",
              "children": []
            }
          ]
        }
      ]
    }
  ]
}
```

Add a folder:

```json
{
  "op": "replace",
  "path": "/folders",
  "value": {
    "functionCall": {
      "name": "appendToTree",
      "args": [
        {
          "path": "/folders"
        },
        {
          "path": "/selectedFolderId"
        },
        {
          "id": {
            "functionCall": {
              "name": "uuid",
              "args": []
            }
          },
          "name": "New Folder",
          "icon": "folder",
          "children": []
        }
      ]
    }
  }
}
```

Delete a folder:

```json
{
  "op": "replace",
  "path": "/folders",
  "value": {
    "functionCall": {
      "name": "removeFromTree",
      "args": [
        {
          "path": "/folders"
        },
        {
          "path": "/selectedFolderId"
        }
      ]
    }
  }
}
```
