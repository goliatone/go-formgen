# Relationship Guide

This guide explains how to model data relationships in go-formgen, configure dynamic option loading, and integrate relationship-aware forms into your application.

## Table of Contents

1. [Overview](#overview)
2. [Relationship Types](#relationship-types)
3. [OpenAPI Configuration](#openapi-configuration)
4. [Endpoint Configuration](#endpoint-configuration)
5. [Programmatic Overrides](#programmatic-overrides)
6. [Rendering Behavior](#rendering-behavior)
7. [Client-Side Integration](#client-side-integration)
8. [Advanced Patterns](#advanced-patterns)
9. [Troubleshooting](#troubleshooting)

---

## Overview

Relationships in go-formgen bridge your **data model associations** with **interactive form controls**. Instead of requiring users to manually type foreign keys, relationships render as dropdowns, searchable selects, or multi-select chips that fetch options from your API.

### What Gets Rendered

| Relationship Type | Typical Widget | User Experience |
|-------------------|----------------|-----------------|
| `belongsTo` | `<select>` dropdown | Choose one related entity (e.g., "Select Author") |
| `hasOne` | Nested fieldset or select | Embedded object or single selection |
| `hasMany` | Multi-select chips/array | Choose multiple entities (e.g., "Select Tags") |

### Data Flow

```
User selects "John Doe" in Author dropdown
         ↓
Form submits author_id: "abc-123"
         ↓
Server receives foreign key value
         ↓
Backend resolves relationship and persists
```

---

## Relationship Types

go-formgen supports three relationship kinds mirroring common ORM patterns:

### belongsTo

**Semantic**: This record **belongs to** one parent entity.

**Example**: An Article belongs to an Author.

**Schema Marker**: The field holds the **foreign key** (e.g., `author_id`).

**Form Behavior**: Renders as dropdown populated with authors from `/api/authors`.

```yaml
properties:
  author_id:
    type: string
    format: uuid
    x-relationships:
      type: belongsTo
      target: "#/components/schemas/Author"
      foreignKey: author_id
```

### hasOne

**Semantic**: This record **has one** associated child entity.

**Example**: An Author has one Profile.

**Schema Marker**: The parent references the child schema; child holds foreign key back to parent.

**Form Behavior**: Can render as nested fieldset or dropdown (if referencing existing profiles).

```yaml
properties:
  profile:
    type: object
    x-relationships:
      type: hasOne
      target: "#/components/schemas/Profile"
      foreignKey: author_id
      sourceField: id
```

### hasMany

**Semantic**: This record **has many** associated child entities.

**Example**: An Author has many Books.

**Schema Marker**: Array field with items referencing child schema.

**Form Behavior**: Multi-select chips or dynamic array editor fetching from `/api/books`.

```yaml
properties:
  books:
    type: array
    items:
      $ref: "#/components/schemas/Book"
    x-relationships:
      type: hasMany
      target: "#/components/schemas/Book"
      cardinality: many
      foreignKey: author_id
      sourceField: id
```

---

## OpenAPI Configuration

### Minimal Relationship Declaration

The `x-relationships` extension defines the association metadata:

```yaml
openapi: 3.0.3
paths:
  /articles:
    post:
      operationId: createArticle
      requestBody:
        content:
          application/json:
            schema:
              type: object
              required:
                - title
                - author_id
              properties:
                title:
                  type: string
                author_id:
                  type: string
                  x-relationships:
                    type: belongsTo
                    target: "#/components/schemas/Author"
                    foreignKey: author_id
```

**Field Attributes**:
- `type` (required): `belongsTo`, `hasOne`, or `hasMany`
- `target` (required): JSON Pointer to related schema (e.g., `#/components/schemas/Author`)
- `foreignKey`: Foreign key field name (optional for belongsTo/hasOne)
- `cardinality`: `one` or `many` (auto-derived from type if omitted)
- `inverse`: Name of reverse relationship on target schema
- `sourceField`: Field on parent used as foreign key value (e.g., `id`)

### Complete Example with hasMany

```yaml
components:
  schemas:
    Author:
      type: object
      properties:
        id:
          type: string
          format: uuid
        name:
          type: string
        books:
          type: array
          items:
            $ref: "#/components/schemas/Book"
          x-relationships:
            type: hasMany
            target: "#/components/schemas/Book"
            cardinality: many
            foreignKey: author_id
            sourceField: id
            inverse: author

    Book:
      type: object
      properties:
        id:
          type: string
        title:
          type: string
        author_id:
          type: string
        author:
          $ref: "#/components/schemas/Author"
          x-relationships:
            type: belongsTo
            target: "#/components/schemas/Author"
            foreignKey: author_id
```

---

## Endpoint Configuration

Relationships need **option data** (label/value pairs) to populate form controls. The `x-endpoint` extension tells the client-side runtime how to fetch these options.

### Basic Endpoint

```yaml
author_id:
  type: string
  x-relationships:
    type: belongsTo
    target: "#/components/schemas/Author"
  x-endpoint:
    url: /api/authors
    method: GET
    labelField: name
    valueField: id
```

**Attributes**:
- `url` (required): API endpoint returning option data
- `method`: HTTP method (default: `GET`)
- `labelField`: Response field to display (e.g., `name`)
- `valueField`: Response field for form submission (e.g., `id`)

### Static Query Parameters

Pass fixed filters to the endpoint:

```yaml
x-endpoint:
  url: /api/authors
  method: GET
  labelField: full_name
  valueField: id
  params:
    active: "true"
    limit: "50"
    order: "name asc"
```

Renders request: `GET /api/authors?active=true&limit=50&order=name+asc`

### Dynamic Parameters (Field References)

Reference other form fields for contextual filtering:

```yaml
category_id:
  type: string
  x-relationships:
    type: belongsTo
    target: "#/components/schemas/Category"
  x-endpoint:
    url: /api/categories
    labelField: name
    valueField: id
    dynamicParams:
      tenant_id: "{{field:tenant_id}}"
```

When user selects `tenant_id`, the category dropdown reloads with:
`GET /api/categories?tenant_id=<selected-value>`

### Search Mode (Autocomplete)

Enable server-side search for large datasets:

```yaml
x-endpoint:
  url: /api/authors
  method: GET
  mode: search
  searchParam: q
  labelField: full_name
  valueField: id
  params:
    limit: "25"
```

User types "John" → `GET /api/authors?q=John&limit=25`

### Authentication Configuration

Specify how to authenticate API requests:

```yaml
x-endpoint:
  url: /api/authors
  method: GET
  labelField: name
  valueField: id
  auth:
    strategy: header
    header: X-Auth-Token
    source: meta:formgen-auth
```

**Auth Strategies**:
- `header` (supported): Add token to request header. For bearer auth, set `header: Authorization` and make the token value include the `Bearer ` prefix (e.g. via the meta tag content).

**Sources**:
- `meta:<name>`: Read from `<meta name="<name>" content="...">`
- `data:<attr>` / `element:<attr>`: Read from a DOM attribute on the field element (e.g. `data-auth-token`)
- `token=<literal>`: Literal token value (mostly useful for demos/tests)

If you need `localStorage`-backed tokens, provide `buildHeaders` when calling `initRelationships(...)`.

### Response Mapping

Transform non-standard API responses:

```yaml
x-endpoint:
  url: /api/authors
  method: GET
  resultsPath: data.authors  # Extract options from nested payload
  mapping:
    value: author_uuid
    label: display_name
```

Example response:
```json
{
  "data": {
    "authors": [
      {"author_uuid": "abc", "display_name": "Jane Doe"}
    ]
  }
}
```

Mapped to: `[{value: "abc", label: "Jane Doe"}]`

### Complete Endpoint Example

```yaml
publisher_id:
  type: string
  x-relationships:
    type: belongsTo
    target: "#/components/schemas/Publisher"
  x-endpoint:
    url: /api/publishers
    method: GET
    mode: search
    searchParam: q
    labelField: name
    valueField: id
    resultsPath: data
    params:
      active: "true"
      format: "options"
    dynamicParams:
      country: "{{field:country_code}}"
    auth:
      strategy: header
      header: X-API-Key
      source: meta:api-key
    mapping:
      value: publisher_id
      label: company_name
```

---

## Programmatic Overrides

When you can't modify the OpenAPI schema (third-party APIs, legacy specs), use **endpoint overrides** in Go code.

### Basic Override

```go
import (
    "github.com/goliatone/go-formgen"
    "github.com/goliatone/go-formgen/pkg/orchestrator"
)

gen := formgen.NewOrchestrator(
    orchestrator.WithEndpointOverrides([]orchestrator.EndpointOverride{
        {
            OperationID: "createArticle",
            FieldPath:   "author_id",
            Endpoint: orchestrator.EndpointConfig{
                URL:        "https://api.example.com/authors",
                Method:     "GET",
                LabelField: "full_name",
                ValueField: "id",
            },
        },
    }),
)
```

### Override with Authentication

```go
orchestrator.WithEndpointOverrides([]orchestrator.EndpointOverride{
    {
        OperationID: "createBook",
        FieldPath:   "publisher_id",
        Endpoint: orchestrator.EndpointConfig{
            URL:        "/api/publishers",
            Method:     "GET",
            LabelField: "name",
            ValueField: "id",
            Auth: &orchestrator.EndpointAuth{
                Strategy: "header",
                Header:   "Authorization",
                Source:   "meta:auth-token",
            },
        },
    },
})
```

### Nested Field Override

Use dot notation for nested fields:

```go
{
    OperationID: "createAuthor",
    FieldPath:   "profile.country_id",  // Nested field
    Endpoint: orchestrator.EndpointConfig{
        URL:        "/api/countries",
        LabelField: "name",
        ValueField: "iso_code",
    },
}
```

### Dynamic Parameters in Overrides

```go
{
    OperationID: "createArticle",
    FieldPath:   "category_id",
    Endpoint: orchestrator.EndpointConfig{
        URL:        "/api/categories",
        LabelField: "name",
        ValueField: "id",
        DynamicParams: map[string]string{
            "tenant_id": "{{field:tenant_id}}",
            "locale":    "{{field:language}}",
        },
    },
}
```

### Override Priority

Overrides are applied **after** the model builder runs and **only if** the field lacks existing endpoint metadata. This ensures OpenAPI schema values take precedence.

---

## Rendering Behavior

### Widget Selection

The widget registry automatically selects components based on relationship metadata:

| Field Pattern | Widget | Template |
|---------------|--------|----------|
| `type: belongsTo` + enum/relationship | `select` | Dropdown |
| `type: hasMany` + array | `chips` | Multi-select chips |
| `type: hasOne` + object with nested | `object` | Nested fieldset |
| `type: hasMany` + endpoint.mode=search | `select` | Searchable dropdown |

### Generated HTML Attributes

Renderers emit `data-*` attributes consumed by client-side JavaScript:

```html
<select
  id="fg-author_id"
  name="author_id"
  data-endpoint-url="/api/authors"
  data-endpoint-method="GET"
  data-endpoint-label-field="name"
  data-endpoint-value-field="id"
  data-relationship-type="belongsTo"
  data-relationship-target="#/components/schemas/Author"
>
  <option value="">Select Author</option>
</select>
```

### Preact Renderer (Interactive)

The `preact` renderer ships a client bundle that reads the same `data-endpoint-*`
and `data-relationship-*` attributes and hydrates richer controls (typeahead,
chips, validation feedback). It also includes the relationship runtime script in
its output page template.

---

## Client-Side Integration

### Including the Runtime

Relationship option loading is powered by the browser runtime bundle
`formgen-relationships.min.js`. You must serve it and include it on pages that
render relationship fields (e.g. vanilla HTML).

```html
<script src="/runtime/formgen-relationships.min.js" defer></script>
```

Serve the runtime bundle from Go:
```go
import (
    "net/http"

    "github.com/goliatone/go-formgen"
)

http.Handle("/runtime/",
  http.StripPrefix("/runtime/",
    http.FileServerFS(formgen.RuntimeAssetsFS()),
  ),
)
```

### Manual Initialization

The runtime attaches to `window.FormgenRelationships` and scans any container
marked with `data-formgen-auto-init` for relationship-enabled fields.
The built-in vanilla form template already sets `data-formgen-auto-init="true"`
on the `<form>`.

```javascript
window.addEventListener("DOMContentLoaded", async () => {
  const api = window.FormgenRelationships;
  if (!api?.initRelationships) return;

  await api.initRelationships({
    logger: console,
    // Example: add headers dynamically (e.g. localStorage tokens).
    buildHeaders: () => {
      const token = window.localStorage?.getItem("authToken");
      return token ? { Authorization: `Bearer ${token}` } : {};
    },
  });
});
```

### Prefilling Current Values

To pre-select relationship values before the runtime loads options, set
`relationship.current` via render options (or `x-current-value` in the schema).
The runtime expects a scalar string (single select) or a JSON array of strings
(multi-select).

```go
html, err := renderer.Render(ctx, form, render.RenderOptions{
    Values: map[string]any{
        "author_id": render.ValueWithProvenance{
            Value:      "abc-123",
            Provenance: "database",
        },
    },
})
```

Renders `data-relationship-current`:
```html
<select name="author_id" data-relationship-current="abc-123">
  <option value="abc-123" selected>abc-123</option>
</select>
```

### Handling Relationship Updates

The runtime dispatches lifecycle events on the field element:
- `formgen:relationship:loading`
- `formgen:relationship:success` (includes `detail.options`)
- `formgen:relationship:error`
- `formgen:relationship:validation`

```javascript
document.addEventListener("formgen:relationship:success", (event) => {
  const { element, options, fromCache } = event.detail;
  console.log("options loaded", { name: element?.getAttribute("name"), fromCache, options });
});

// User selection changes still use native DOM events:
document.addEventListener("change", (event) => {
  const target = event.target;
  if (target instanceof HTMLSelectElement && target.matches("[data-endpoint-url]")) {
    console.log("user selected", target.name, target.value);
  }
});

// Manual refresh for a specific element:
async function refreshRelationship(selector) {
  const el = document.querySelector(selector);
  if (!(el instanceof HTMLElement)) return;
  const registry = await window.FormgenRelationships?.initRelationships?.();
  await registry?.resolve?.(el);
}
```

---

## Advanced Patterns

### Cascading Dropdowns

Configure dependent relationships using `dynamicParams`:

```yaml
properties:
  country_id:
    type: string
    x-relationships:
      type: belongsTo
      target: "#/components/schemas/Country"
    x-endpoint:
      url: /api/countries
      labelField: name
      valueField: id

  state_id:
    type: string
    x-relationships:
      type: belongsTo
      target: "#/components/schemas/State"
    x-endpoint:
      url: /api/states
      labelField: name
      valueField: id
      dynamicParams:
        country_id: "{{field:country_id}}"  # Reload when country changes

  city_id:
    type: string
    x-relationships:
      type: belongsTo
      target: "#/components/schemas/City"
    x-endpoint:
      url: /api/cities
      labelField: name
      valueField: id
      dynamicParams:
        state_id: "{{field:state_id}}"
        country_id: "{{field:country_id}}"
```

User flow:
1. Select "United States" → `state_id` dropdown loads US states
2. Select "California" → `city_id` dropdown loads CA cities

### Polymorphic Relationships

Polymorphic targets are not first-class today (a relationship `target` must be a
static schema pointer), and `x-endpoint.url` is not dynamically templated.
If you need polymorphism, model it as a single endpoint and use `dynamicParams`
to pass the discriminator.

```yaml
attachable_type:
  type: string
  enum: [Article, Video, Podcast]

attachable_id:
  type: string
  x-relationships:
    type: belongsTo
    target: "#/components/schemas/Attachable"
  x-endpoint:
    url: /api/polymorphic
    method: GET
    labelField: title
    valueField: id
    dynamicParams:
      type: "{{field:attachable_type}}"
```

### Many-to-Many with Junction Table

Model many-to-many relationships using arrays:

```yaml
components:
  schemas:
    Article:
      properties:
        tags:
          type: array
          items:
            type: string
          x-relationships:
            type: hasMany
            target: "#/components/schemas/Tag"
            cardinality: many
          x-endpoint:
            url: /api/tags
            method: GET
            mode: search
            searchParam: q
            labelField: name
            valueField: id
            submitAs: json
```

Form submits:
```json
{
  "title": "Go Patterns",
  "tags": ["abc-123", "def-456", "ghi-789"]
}
```

Backend resolves to junction table inserts.

### Create Action Modal (Related Entity Form)

Use the "Create ..." action when related entities require a real form (multiple
fields, permissions, validation) instead of inline creation. The runtime only
triggers the action; your application owns the modal markup and submit logic.

#### 1) Enable create action on the relationship field

```html
<select
  name="author_id"
  data-endpoint-url="/api/authors"
  data-endpoint-renderer="typeahead"
  data-endpoint-mode="search"
  data-endpoint-create-action="true"
  data-endpoint-create-action-id="author"
  data-endpoint-create-action-label="Create Author"
  data-relationship-cardinality="one"
></select>
```

#### 2) Define the modal and form (Pongo2 templates)

Create a modal partial that includes **only the fields you want** for the
related entity. This keeps the create flow focused and fast.

```django
{# templates/partials/modals/author_create.tmpl #}
<div
  id="modal-author-create"
  class="fixed inset-0 hidden items-center justify-center bg-black/50"
  data-fg-create-modal="author"
  aria-hidden="true"
>
  <div class="w-full max-w-lg rounded-xl bg-white p-6 shadow-xl">
    <header class="mb-4">
      <h2 class="text-lg font-semibold text-gray-900">Create Author</h2>
      <p class="text-sm text-gray-500">Add the minimum required details.</p>
    </header>
    <form id="form-author-create" class="space-y-4">
      {% include "partials/forms/author_create_fields.tmpl" %}
      <div class="flex justify-end gap-2 pt-2">
        <button type="button" class="px-4 py-2 text-sm" data-fg-modal-close="author">Cancel</button>
        <button type="submit" class="rounded-lg bg-blue-600 px-4 py-2 text-sm text-white">Create</button>
      </div>
    </form>
  </div>
</div>
```

```django
{# templates/partials/forms/author_create_fields.tmpl #}
<label class="block text-sm font-medium text-gray-700">Name</label>
<input name="name" class="w-full rounded-md border px-3 py-2" />

<label class="block text-sm font-medium text-gray-700">Email</label>
<input name="email" type="email" class="w-full rounded-md border px-3 py-2" />
```

Include the modal partial once on the page where the relationship field lives:

```django
{% include "partials/modals/author_create.tmpl" %}
```

#### 3) Wire the create action to the modal (vanilla TypeScript)

```ts
import { initRelationships } from "@goliatone/formgen-runtime";

function openModal(modal: HTMLElement): void {
  modal.classList.remove("hidden");
  modal.setAttribute("aria-hidden", "false");
}

function closeModal(modal: HTMLElement): void {
  modal.classList.add("hidden");
  modal.setAttribute("aria-hidden", "true");
}

function waitForSubmit(form: HTMLFormElement): Promise<FormData | null> {
  return new Promise((resolve) => {
    const onSubmit = (event: Event) => {
      event.preventDefault();
      form.removeEventListener("submit", onSubmit);
      resolve(new FormData(form));
    };
    const onCancel = () => {
      form.removeEventListener("submit", onSubmit);
      resolve(null);
    };
    form.addEventListener("submit", onSubmit, { once: true });
    form
      .querySelectorAll<HTMLElement>("[data-fg-modal-close]")
      .forEach((btn) => btn.addEventListener("click", onCancel, { once: true }));
  });
}

await initRelationships({
  onCreateAction: async (_context, detail) => {
    if (detail.actionId !== "author") {
      return;
    }

    const modal = document.querySelector<HTMLElement>('[data-fg-create-modal="author"]');
    const form = document.getElementById("form-author-create") as HTMLFormElement | null;
    if (!modal || !form) {
      return;
    }

    // Prefill with the current query if present.
    const nameInput = form.querySelector<HTMLInputElement>('input[name="name"]');
    if (nameInput && detail.query) {
      nameInput.value = detail.query;
    }

    openModal(modal);
    const data = await waitForSubmit(form);
    closeModal(modal);

    if (!data) {
      return;
    }

    const response = await fetch("/api/authors", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(Object.fromEntries(data.entries())),
    });

    const created = await response.json();
    return { value: String(created.id), label: created.name };
  },
});
```

Notes:
- For chips (multi-select), you can return multiple options and choose
  append/replace using `data-endpoint-create-action-select`.
- If you prefer DOM events instead of hooks, listen for
  `formgen:relationship:create-action` and follow the same modal flow.

### Optimistic UI Updates

Optimistic UI hooks are not part of the runtime API today. Use the emitted
events plus normal DOM `change` handlers to implement app-specific behaviour.

---

## Troubleshooting

### Dropdown Shows No Options

**Symptoms**: Select renders but no options populate.

**Checklist**:
1. Verify endpoint URL is reachable: `curl /api/authors`
2. Check browser Network tab for failed requests
3. Confirm response format matches `resultsPath` configuration
4. Validate `labelField` and `valueField` exist in response
5. Check CORS headers if endpoint is cross-origin
6. Verify authentication token is present (check `meta` tags)

**Debug**:
```javascript
await window.FormgenRelationships?.initRelationships?.({ logger: console });
```

### Dynamic Parameters Not Working

**Symptoms**: Dropdown doesn't reload when parent field changes.

**Checklist**:
1. Verify parent field name matches `dynamicParams` key exactly
2. Check parent field has a value before dependent loads
3. Confirm `data-endpoint-refresh-on` attribute exists on dependent field
4. Ensure `formgen-relationships.min.js` is loaded

**Manual Trigger**:
```javascript
const registry = await window.FormgenRelationships?.initRelationships?.();
const el = document.querySelector('[name="state_id"]');
if (el instanceof HTMLElement) {
  await registry?.resolve?.(el);
}
```

### Authentication Failing

**Symptoms**: 401/403 responses from relationship endpoints.

**Checklist**:
1. Verify auth configuration strategy matches server expectations
2. Check meta tag exists: `<meta name="formgen-auth" content="token">`
3. Confirm header name matches server requirements (case-sensitive)
4. Test endpoint with curl using same headers

If you need tokens from `localStorage` or other sources, provide `buildHeaders`
when calling `initRelationships(...)` (see Manual Initialization).

### Wrong Options Displayed

**Symptoms**: Dropdown shows incorrect entities.

**Checklist**:
1. Verify `target` schema matches intended entity
2. Check static `params` aren't over-filtering
3. Confirm `resultsPath` extracts correct array from response
4. Validate `mapping` configuration if using custom fields

**Inspect Metadata**:
```javascript
const select = document.querySelector('[name="author_id"]');
console.log(select.dataset);  // View all data-* attributes
```

### Relationship Data Not Submitted

**Symptoms**: Form submits but relationship field is missing/null.

**Checklist**:
1. Verify `name` attribute exists on select element
2. Check field isn't disabled/readonly
3. Confirm option has `value` attribute set
4. Validate form serialization includes all fields

**Debug Submission**:
```javascript
form.addEventListener('submit', (e) => {
  e.preventDefault();
  const data = new FormData(form);
  console.log(Array.from(data.entries())); // Inspect raw entries (keeps duplicates)
  console.log(data.getAll("tags[]"));      // Example for multi-select default encoding
});
```

---

## Reference

### Relationship Struct (Go)

```go
type Relationship struct {
    Kind        RelationshipKind  // belongsTo, hasOne, hasMany
    Target      string            // JSON Pointer to schema
    ForeignKey  string            // Foreign key field name
    Cardinality string            // "one" or "many"
    Inverse     string            // Reverse relationship name
    SourceField string            // Parent field used as FK value
}
```

### Endpoint Config (Go)

```go
type EndpointConfig struct {
    URL           string
    Method        string
    LabelField    string
    ValueField    string
    ResultsPath   string
    Params        map[string]string
    DynamicParams map[string]string
    Mapping       EndpointMapping
    Auth          *EndpointAuth
    SubmitAs      string
}
```

### Client-Side API

```javascript
// Initialize (returns a registry)
const registry = await window.FormgenRelationships.initRelationships(options);

// Resolve options for a specific element
const el = document.querySelector('[name="author_id"]');
if (el instanceof HTMLElement) {
  await registry.resolve(el);
}

// Listen for events
document.addEventListener('formgen:relationship:loading', handler);
document.addEventListener('formgen:relationship:success', handler);
document.addEventListener('formgen:relationship:error', handler);
document.addEventListener('formgen:relationship:validation', handler);
```

### Data Attributes Reference

| Attribute | Purpose | Example |
|-----------|---------|---------|
| `data-endpoint-url` | API endpoint | `/api/authors` |
| `data-endpoint-method` | HTTP method | `GET` |
| `data-endpoint-label-field` | Display field | `name` |
| `data-endpoint-value-field` | Submit field | `id` |
| `data-endpoint-mode` | Interaction mode | `search` |
| `data-endpoint-search-param` | Query param | `q` |
| `data-endpoint-refresh-on` | Trigger field | `category_id` |
| `data-endpoint-renderer` | Runtime renderer | `chips` |
| `data-endpoint-submit-as` | Submit encoding | `json` |
| `data-relationship-type` | Relationship kind | `belongsTo` |
| `data-relationship-target` | Target schema | `#/components/schemas/Author` |
| `data-relationship-current` | Prefilled value(s) | `abc-123` or `["a","b"]` |
| `data-auth-strategy` | Auth method | `header` |
| `data-auth-header` | Header name | `X-Auth-Token` |
| `data-auth-source` | Token source | `meta:formgen-auth` |

---

## Examples

### Complete Working Example

**OpenAPI Schema** (`api-spec.yaml`):
```yaml
openapi: 3.0.3
info:
  title: Blog API
  version: 1.0.0
paths:
  /articles:
    post:
      operationId: createArticle
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [title, author_id]
              properties:
                title:
                  type: string
                content:
                  type: string
                author_id:
                  type: string
                  format: uuid
                  x-relationships:
                    type: belongsTo
                    target: "#/components/schemas/Author"
                    foreignKey: author_id
                  x-endpoint:
                    url: /api/authors
                    method: GET
                    labelField: full_name
                    valueField: id
                    params:
                      active: "true"
                tags:
                  type: array
                  items:
                    type: string
                  x-relationships:
                    type: hasMany
                    target: "#/components/schemas/Tag"
                    cardinality: many
                  x-endpoint:
                    url: /api/tags
                    method: GET
                    mode: search
                    searchParam: q
                    labelField: name
                    valueField: id
                    submitAs: json
      responses:
        "201":
          description: Created
components:
  schemas:
    Author:
      type: object
      x-formgen-label-field: full_name
      properties:
        id:
          type: string
        full_name:
          type: string
    Tag:
      type: object
      properties:
        id:
          type: string
        name:
          type: string
```

**Go Integration**:
```go
package main

import (
    "context"
    "log"
    "net/http"

    "github.com/goliatone/go-formgen"
    "github.com/goliatone/go-formgen/pkg/openapi"
    "github.com/goliatone/go-formgen/pkg/orchestrator"
    "github.com/goliatone/go-formgen/pkg/render"
    "github.com/goliatone/go-formgen/pkg/renderers/vanilla"
)

func main() {
    ctx := context.Background()

    registry := render.NewRegistry()
    registry.MustRegister(mustVanillaRenderer())

    gen := formgen.NewOrchestrator(
        orchestrator.WithRegistry(registry),
        orchestrator.WithDefaultRenderer("vanilla"),
    )

    http.Handle("/runtime/",
        http.StripPrefix("/runtime/",
            http.FileServerFS(formgen.RuntimeAssetsFS()),
        ),
    )

    http.HandleFunc("/forms/article/new", func(w http.ResponseWriter, r *http.Request) {
        html, err := gen.Generate(ctx, orchestrator.Request{
            Source:      openapi.SourceFromFile("api-spec.yaml"),
            OperationID: "createArticle",
            Renderer:    "vanilla",
        })
        if err != nil {
            http.Error(w, err.Error(), 500)
            return
        }

        w.Header().Set("Content-Type", "text/html; charset=utf-8")
        w.Write(html)
    })

    log.Fatal(http.ListenAndServe(":8080", nil))
}

func mustVanillaRenderer() *vanilla.Renderer {
    r, err := vanilla.New(vanilla.WithDefaultStyles())
    if err != nil {
        panic(err)
    }
    return r
}
```

**HTML Page Template**:
```html
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="formgen-auth" content="{{.AuthToken}}">
    <title>Create Article</title>
</head>
<body>
    <div id="form-container">
        <!-- Rendered form injected here -->
        {{.FormHTML}}
    </div>

    <script src="/runtime/formgen-relationships.min.js" defer></script>
    <script>
        window.addEventListener("DOMContentLoaded", async () => {
            if (window.FormgenRelationships?.initRelationships) {
                await window.FormgenRelationships.initRelationships();
            }
        });

        document.querySelector("form").addEventListener("submit", async (e) => {
            e.preventDefault();
            const data = new FormData(e.target);
            const payload = Object.fromEntries(data);

            // If tags are configured with x-endpoint.submitAs: json, parse them before sending JSON.
            if (typeof payload.tags === "string") {
                try { payload.tags = JSON.parse(payload.tags); } catch (_) {}
            }

            const response = await fetch("/api/articles", {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                    "X-Auth-Token": document.querySelector('meta[name="formgen-auth"]').content,
                },
                body: JSON.stringify(payload),
            });

            if (response.ok) {
                window.location.href = "/articles";
            } else {
                alert("Failed to create article");
            }
        });
    </script>
</body>
</html>
```

This example demonstrates:
- belongsTo relationship (author_id → Author)
- hasMany relationship (tags → Tag)
- Search-enabled multi-select
- Authentication via meta tag
- Form submission with relationship data transformation

---

## Further Reading

- [OpenAPI Extensions](README_SCHEMA.md#supported-custom-extensions) - Complete extension reference
- [Customization Guide](GUIDE_CUSTOMIZATION.md) - UI schemas, widgets, behaviors
- [Architecture Design](../ARCH_DESIGN.md) - System design principles
- [Relationship ADR](../docs/adr/RELATIONSHIP_STRUCT_ADR.md) - Implementation decisions

For questions or issues, see [GitHub Issues](https://github.com/goliatone/go-formgen/issues).
