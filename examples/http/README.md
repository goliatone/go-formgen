# HTTP Demo

This example demonstrates go-formgen's form generation capabilities through a local HTTP server. It showcases multiple renderers, relationship fields, modal forms, and edit/prefill workflows.

## Getting Started

Run the sample server:

```bash
go run ./examples/http
```

Then open [http://localhost:8383/](http://localhost:8383/) to see this documentation with interactive navigation.

---

## Available Examples

### `/form` - Basic Form Rendering

**URL:** [http://localhost:8383/form](http://localhost:8383/form)

The `/form` endpoint renders any operation from the configured OpenAPI document. This is the simplest way to see go-formgen in action.

**What you'll see:**
- A complete HTML form generated from OpenAPI schema
- All field types: text, number, select, textarea, checkbox, etc.
- Validation rules derived from OpenAPI constraints
- Relationship fields with typeahead/chips components

**Query parameters:**
| Parameter | Description | Example |
|-----------|-------------|---------|
| `renderer` | Switch between renderers | `?renderer=preact` |
| `operation` | Select different operations | `?operation=createArticle` |
| `source` | Load a different OpenAPI spec | `?source=https://...` |

**Try these variations:**
- [/form?renderer=preact](http://localhost:8383/form?renderer=preact) - Preact renderer with client-side hydration
- [/form?renderer=vanilla](http://localhost:8383/form?renderer=vanilla) - Server-side vanilla HTML

---

### `/advanced` - Create Actions & Modal Forms

**URL:** [http://localhost:8383/advanced](http://localhost:8383/advanced)

The advanced view demonstrates the full power of go-formgen's relationship system, including modal forms for creating related entities on-the-fly.

**What you'll see:**
- A `post-book:create` form with multiple relationship fields
- "Create Author", "Create Publisher", "Create Tag" modal forms
- Chips component for multi-select relationships (tags)
- Typeahead component for single-select relationships (author, publisher)
- Create actions that open modals, submit via fetch, and update selections in-place

**Key features demonstrated:**
1. **Create Actions**: Click "Create Author" to open a modal, submit a new author, and have it auto-selected
2. **Chips (Multi-select)**: Tags field with search, add/remove chips, and inline create
3. **Typeahead (Single-select)**: Author field with search-as-you-type and option selection
4. **Prefill from Search**: When you search for a non-existent option and click "Create", the modal prefills with your search query

---

### `/form` (Edit Mode) - Prefill & Validation

**URL:** [http://localhost:8383/form?id=article-edit&method=PATCH](http://localhost:8383/form?id=article-edit&method=PATCH)

This demonstrates the edit/prefill workflow for updating existing records.

**What you'll see:**
- Form pre-populated with existing values
- PATCH method with `_method` hidden input for method override
- Relationship fields showing current selections as chips/typeahead values
- Validation error states (if configured)

**RenderOptions in action:**
| Option | Description |
|--------|-------------|
| `Method` | Switches form to PATCH, injects `_method` hidden input |
| `Values` | Pre-populates all fields including relationships |
| `Errors` | Shows server validation errors inline |
| `Subset` | Render only specific groups/tags/sections |

**Try these variations:**
- [/form?id=article-edit&method=PUT](http://localhost:8383/form?id=article-edit&method=PUT) - PUT method override
- [/form?groups=notifications](http://localhost:8383/form?groups=notifications) - Render only the notifications group
- [/form?tags=advanced](http://localhost:8383/form?tags=advanced) - Render only fields tagged "advanced"

---

## UI Schema Configuration

The HTTP demo uses UI schema metadata in `examples/http/ui/schema.json` to configure relationship behavior. This is the preferred approach over hand-authored HTML attributes.

### Create Action Configuration

```json
{
  "operations": {
    "post-book:create": {
      "fields": {
        "author_id": {
          "metadata": {
            "relationship.endpoint.createAction": "true",
            "relationship.endpoint.createActionId": "author",
            "relationship.endpoint.createActionLabel": "Create Author",
            "relationship.endpoint.createActionSelect": "replace"
          }
        }
      }
    }
  }
}
```

### Inline Create (Tags)

```json
{
  "operations": {
    "post-book:create": {
      "fields": {
        "tags": {
          "metadata": {
            "relationship.endpoint.mode": "search",
            "relationship.endpoint.allowCreate": "true"
          }
        }
      }
    }
  }
}
```

### Search Tuning + Manual Refresh

```json
{
  "operations": {
    "post-book:create": {
      "fields": {
        "author_id": {
          "metadata": {
            "relationship.endpoint.searchParam": "q",
            "relationship.endpoint.debounce": "250",
            "relationship.endpoint.throttle": "500",
            "relationship.endpoint.refresh": "manual",
            "relationship.endpoint.refreshOn": "tenant_id"
          }
        }
      }
    }
  }
}
```

---

## Testing Loading Indicators

Relationship endpoints support an optional `_delay` query parameter to simulate network latency and demonstrate loading indicators.

### Using the Built-in Delay Button

The `/advanced` page includes an "Apply 1s Delay" button that properly configures all relationship resolvers with a delay. This is the recommended way to test loading indicators.

### Manual Testing

**For search-mode fields** (most relationship fields), the loading indicator appears when you type and a fetch begins:

1. Open [/advanced](http://localhost:8383/advanced)
2. Click "Apply 1s Delay" button (or use the console snippet below)
3. Type a character in any search input
4. You'll see a spinner with "Loading..." while results are fetched

**Console snippet for custom delays:**
```javascript
// Apply delay to all relationship resolvers
document.querySelectorAll('[data-endpoint-url]').forEach(el => {
  const url = el.getAttribute('data-endpoint-url');
  if (url && !url.includes('_delay')) {
    el.setAttribute('data-endpoint-url', url + (url.includes('?') ? '&' : '?') + '_delay=1s');
  }
});
// Re-initialize to pick up the new URLs
if (window.formgen && window.formgen.initRelationships) {
  window.formgen.initRelationships();
}
```

**Verify delay endpoint directly:**
```bash
curl "http://localhost:8383/api/authors?_delay=2s&format=options"
```

Valid delay formats: `500ms`, `1s`, `2s` (max 10s).

---

## API Endpoints

The demo server provides mock API endpoints for relationship fields:

| Endpoint | Description |
|----------|-------------|
| `/api/authors` | Returns author options for typeahead |
| `/api/publishers` | Returns publisher options |
| `/api/tags` | Returns tag options for chips |
| `/api/categories` | Returns category options |
| `/api/managers` | Returns manager options |
| `/api/uploads/` | File upload endpoint |

All relationship endpoints support:
- `?format=options` - Returns `[{value, label}]` format
- `?q=search` - Filters results by search query
- `?_delay=1s` - Adds artificial latency for testing

---

## Runtime Assets

The runtime JavaScript bundles are served from `/runtime/` via `formgen.RuntimeAssetsFS()`:

- `/runtime/formgen-relationships.min.js` - Chips and typeahead components
- `/runtime/formgen-behaviors.min.js` - Form behaviors (validation, submit handling)

These work with both vanilla and Preact renderers.
