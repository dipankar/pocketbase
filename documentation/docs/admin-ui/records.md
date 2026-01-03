# Managing Records

The Admin UI provides tools for browsing, creating, and editing records.

## Viewing Records

### Table View

1. Select a collection from the sidebar
2. Records display in a table format
3. Click column headers to sort
4. Use pagination controls at the bottom

### Customizing Columns

1. Click the columns icon
2. Select/deselect visible columns
3. Drag to reorder columns

### Record Details

Click any record to view its full details in a side panel.

## Filtering Records

### Quick Search

Use the search box to filter across multiple fields:

```
john@example.com
```

### Advanced Filters

Click **Filter** to build complex queries:

```
published = true && views > 100
```

### Filter Operators

| Operator | Description |
|----------|-------------|
| `=` | Equals |
| `!=` | Not equals |
| `>` | Greater than |
| `<` | Less than |
| `>=` | Greater or equal |
| `<=` | Less or equal |
| `~` | Contains (case-insensitive) |
| `!~` | Not contains |

### Example Filters

```
# Published posts
status = "published"

# Recent records
created > "2024-01-01"

# Multiple conditions
status = "active" && role = "admin"

# Contains text
title ~ "hello"

# Null values
deletedAt = null
```

## Sorting Records

### Single Column Sort

Click a column header to sort:

- First click: Ascending (A-Z, 0-9)
- Second click: Descending (Z-A, 9-0)
- Third click: Clear sort

### Multiple Column Sort

Use the filter syntax:

```
sort: -created, title
```

The `-` prefix indicates descending order.

## Creating Records

### Via Form

1. Click **+ New record**
2. Fill in the form fields
3. Click **Create**

### Required Fields

Required fields are marked with an asterisk (*). The form won't submit until all required fields are filled.

### File Uploads

For file fields:

1. Click the upload area
2. Select file(s) from your computer
3. Preview uploads before saving

### Relations

For relation fields:

1. Click the field
2. Search for records
3. Select one or more records
4. Save

## Editing Records

### Edit Form

1. Click on a record
2. Modify fields in the side panel
3. Click **Save**

### Quick Edit

Double-click a cell in table view to edit inline (where supported).

### Relation Management

In the edit form:

- Click **×** to remove a relation
- Click **+** to add relations
- Search to find related records

### File Management

In file fields:

- Click **×** to remove files
- Drop new files to add
- Drag to reorder (multi-file)

## Deleting Records

### Single Record

1. Click on the record
2. Click **Delete** in the panel
3. Confirm deletion

### Multiple Records

1. Select records using checkboxes
2. Click **Delete selected**
3. Confirm deletion

!!! danger "Permanent"
    Deleted records cannot be recovered. Consider soft-delete patterns instead.

## Bulk Actions

### Select Multiple

- Click checkboxes to select individual records
- Use the header checkbox to select all on page

### Available Actions

- **Delete selected** - Remove all selected records
- **Export selected** - Download as JSON

## Exporting Data

### Export Records

1. Filter to desired records
2. Click **Export**
3. Choose format (JSON)
4. Download file

### Export All

Remove filters before exporting to get all records.

## Importing Data

Currently, bulk import is available via:

- API batch operations
- JavaScript hooks
- Custom scripts

## Working with Different Field Types

### Text Fields

- Enter text directly
- Multi-line text uses textarea

### Number Fields

- Enter numeric values
- Validation enforces min/max

### Boolean Fields

- Toggle switch interface
- True/False

### Select Fields

- Dropdown for single select
- Checkboxes for multi-select

### Date Fields

- Date picker interface
- Time selection if configured

### JSON Fields

- Code editor with syntax highlighting
- Validation for valid JSON

### Editor Fields

- Rich text editor
- Toolbar for formatting
- HTML output

### Relation Fields

- Search and select records
- View related record details
- Navigate to related records

### File Fields

- Drag and drop upload
- Preview images
- Multiple file support

## Record History

The Admin UI shows:

- Created timestamp
- Last updated timestamp
- Record ID

## Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| `Ctrl/Cmd + N` | New record |
| `Escape` | Close panel |
| `Enter` | Save record |

## Tips

### Performance

- Use pagination for large datasets
- Add filters to reduce data load
- Create indexes for filtered fields

### Data Entry

- Tab between fields
- Use keyboard for boolean toggles
- Copy IDs by clicking

### Relations

- Open related records in new tabs
- Use expand to see related data

## Troubleshooting

### Record Won't Save

1. Check all required fields
2. Verify field validation
3. Check API rules allow create/update
4. Look for error messages

### Can't Delete Record

1. Check delete rule permissions
2. Check for cascade delete settings
3. Verify you're authenticated

### Slow Loading

1. Add indexes on filtered fields
2. Reduce records per page
3. Use specific filters

## Next Steps

- [Collections Management](collections.md)
- [API Records](../api/records.md)
- [Fields Reference](../features/fields.md)
