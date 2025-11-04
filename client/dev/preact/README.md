# Preact Sandbox

This sandbox demonstrates using the formgen runtime with **Preact components** and the `useRelationshipOptions` hook.

## Quick Start

**Vanilla renderer:**
```bash
npm run dev:preact
```

**Basecoat renderer:**
```bash
npm run dev:preact:basecoat
```

This opens http://localhost:5173/preact/ with a Preact-rendered form.

## What's Different

Unlike the vanilla sandbox (`dev/index.html`) which generates HTML strings, this sandbox:

- ✅ Uses **Preact components** with JSX
- ✅ Demonstrates the **`useRelationshipOptions` hook**
- ✅ Shows **reactive state management** in Preact
- ✅ Provides **TypeScript type safety** for components
- ✅ Uses the **same relationship fields** (chips, typeahead)

## Architecture

### Components

**`SelectField`** - Single-select relationship field with typeahead:
```tsx
<SelectField
  fieldId="article-author"
  name="article[author_id]"
  label="Author"
  required
  endpoint={{
    url: '/api/authors',
    renderer: 'typeahead',
    mode: 'search',
    searchParam: 'q',
  }}
/>
```

**`MultiSelectField`** - Multi-select relationship field with chips:
```tsx
<MultiSelectField
  fieldId="article-tags"
  name="article[tags][]"
  label="Tags"
  endpoint={{
    url: '/api/tags',
    renderer: 'chips',
    mode: 'search',
    searchParam: 'q',
  }}
/>
```

### Hook Usage

Both components use the `useRelationshipOptions` hook:

```tsx
import { useRelationshipOptions } from "../../src/frameworks/preact";

function SelectField({ endpoint, ... }) {
  const selectRef = useRef<HTMLSelectElement>(null);
  const { options, loading, error } = useRelationshipOptions(selectRef.current);

  return (
    <select ref={selectRef} data-endpoint-url={endpoint.url}>
      {/* The hook automatically populates options */}
    </select>
  );
}
```

The hook provides:
- **`options`** - Array of `{ value, label }` from the API
- **`loading`** - Boolean indicating fetch in progress
- **`error`** - Error object if fetch failed
- **`refresh()`** - Function to manually refetch options

## Features

### 1. **Reactive State**
The hook automatically:
- Initializes the relationship registry
- Fetches options from the endpoint
- Updates when dependencies change
- Cleans up on unmount

### 2. **Type Safety**
TypeScript provides intellisense for:
- Component props
- Hook return values
- Event handlers

### 3. **Loading States**
Shows loading/error feedback:
```tsx
{loading && <small>Loading...</small>}
{error && <small>Error: {error.message}</small>}
{!loading && !error && <small>{options.length} options loaded</small>}
```

### 4. **Renderer Support**
Switch between vanilla and basecoat renderers using the dropdown or URL param.

## Workflow

1. **Edit Preact components**: Modify `app.tsx`
2. **Edit the hook**: Modify `src/frameworks/preact.ts`
3. **Save changes**: Vite hot-reloads instantly
4. **Test in browser**: See changes immediately

Perfect for:
- Building custom Preact form components
- Testing the `useRelationshipOptions` hook
- Developing type-safe form UIs
- Integrating with Preact/React apps

## Mock API

The sandbox includes a mock API that responds to:
- `/api/authors` - Returns author options
- `/api/categories` - Returns category options
- `/api/tags` - Returns tag options
- `/api/managers` - Returns manager options

All endpoints support `?q=search` for filtering.

## Switching Renderers

Use the dropdown in the header or URL params:
- `?renderer=vanilla` - Custom CSS (~8KB)
- `?renderer=basecoat` - Tailwind CSS (~111KB)

## Comparison: Vanilla vs Preact

| Approach | Pros | Cons |
|----------|------|------|
| **Vanilla** (`dev/index.html`) | No framework overhead, plain HTML | Manual DOM manipulation |
| **Preact** (`dev/preact/`) | Reactive state, components, type safety | Requires Preact dependency |

Use vanilla for server-rendered forms, Preact for client-side apps.
