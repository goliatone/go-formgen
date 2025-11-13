# Development Sandbox

This directory contains a live development environment for the formgen client runtime.

## Quick Start

From the `client/` directory, run:

### Vanilla Sandbox (HTML string generation)

**Default (Vanilla renderer):**
```bash
npm run dev
```

**Explicit vanilla sandbox (same view, forces browser open):**
```bash
npm run dev:vanilla
```

**Tailwind theme watch (rebuilds CSS as you edit):**
```bash
npm run dev:theme
```

### Preact Sandbox (Component-based)

```bash
npm run dev:preact
```

This will:
- Start a Vite dev server at http://localhost:5173
- Automatically open your browser with the selected renderer
- Watch for changes in `src/` and reload instantly
- Import TypeScript source files directly (no build step needed)

## Development Workflow

### 1. Live Preview with Hot Reload

```bash
cd client
npm run dev
```

The browser will automatically reload when you edit files in:
- `src/` - Runtime source code
- `dev/index.html` - Sandbox HTML
- Any imported TypeScript/JavaScript files

### 2. Testing Changes

```bash
npm test
```

Run tests in watch mode:
```bash
npm test -- --watch
```

### 3. Building for Production

```bash
npm run build
```

This creates optimized bundles in `dist/`:
- `dist/esm/` - ES modules for npm consumers
- `dist/browser/` - Minified bundles for browser CDN
- `dist/types/` - TypeScript declarations

### 4. Watch Mode (Alternative)

If you prefer working with built files:

```bash
npm run watch
```

This rebuilds the bundles whenever source files change.

## Switching Renderers

You can switch between renderers in two ways:

1. **URL parameter**: Add `?renderer=vanilla` or `?renderer=preact` to the URL
2. **Dropdown selector**: Use the renderer dropdown in the page header

When you change renderers via the dropdown, the page reloads with the new CSS.

### Available Renderers

- **vanilla** - Custom CSS with minimal dependencies (~8KB)
- **preact** - Component-driven preview that mirrors the runtime helper

## Sandbox Features

The sandbox demonstrates:

- **Chips Renderer**: Multi-select with search
- **Typeahead Renderer**: Single-select with autocomplete
- **Relationship Fields**: Dynamic options loading with dependent refresh
- **Shared UI Schema**: `dev/ui-schema.json` keeps layout metadata in sync across vanilla + Preact views
- **Mock API**: In-memory fetch interceptor that honours pagination, tenant scopes, and dynamic params
- **Live Form Generation**: Preact view renders directly from `dev/schema.json` + UI schema
- **Production CSS Preview**: Loads exact CSS that ships with Go package
- **Toolbar Toggles**: Buttons to load a sample record or inject server errors via the `hydrateFormValues` helper, mirroring edit-mode flows

## CSS Development - Exact Production Preview

**Critical:** The sandbox loads the **exact production CSS** via symlinks:
```
dev/formgen-vanilla.css → ../../pkg/renderers/vanilla/assets/formgen-vanilla.css
```

The sandbox has minimal wrapper styling (page background, header) but **does NOT override any form styles**. What you see = What users see.

**How it works:**
- CSS files are **symlinks** to the production CSS
- Always stays in sync (no manual copying needed)
- Vite watches the real files and hot-reloads changes

**To update styles:**

1. **Edit the production CSS file**
   ```bash
   vim ../../pkg/renderers/vanilla/assets/formgen-vanilla.css
   ```

2. **Save and see changes instantly**
   - Vite detects change via symlink
   - Browser auto-reloads
   - No build step needed!

3. **Build client when satisfied**
   ```bash
   npm run build
   ```
   This embeds the updated CSS into the browser bundle.

**Benefits:**
- ✅ **Pixel-perfect production preview** - Symlinked CSS, zero overrides
- ✅ **Zero style drift** - Symlink guarantees sync with production
- ✅ **Instant feedback** - CSS hot reload via Vite
- ✅ **True WYSIWYG** - Exactly what ships to users

Use `npm run dev:theme` when iterating on Tailwind utilities or the generated theme bundle—this runs the Tailwind watcher and writes updates to `dist/themes/formgen.css` so you can copy the output into the Go assets or publish it separately.

## Customizing the Sandbox

Update sandbox data through the dedicated fixtures:

1. **Schema/metadata**  
   - `dev/schema.json` – OpenAPI fixture describing relationships and dynamic parameters  
   - `dev/ui-schema.json` – Layout, sections, and renderer hints used by both sandboxes
2. **Mock API**  
   - `dev/mock-api.ts` – Adjust tenant/category datasets, search behaviour, or pagination logic
3. **Runtime wiring**  
   - `dev/vanilla.ts` – Loads Go-rendered HTML snapshots  
   - `dev/preact/app.tsx` – Hydrates the same scenario via `loadSandboxScenario()`

### Runtime theme overrides

The resolver exposes `registerThemeClasses` so you can replace or extend the default class maps before calling `initRelationships()`:

```ts
import { registerThemeClasses, initRelationships } from "@goliatone/formgen-runtime";

registerThemeClasses({
  chips: {
    container: ["relative", "w-full", "text-sm", "bg-slate-50"],
    actionClear: ["hover:text-red-500"],
  },
  typeahead: {
    control: ["border-slate-400", "focus-within:ring-purple-500"],
  },
});

await initRelationships();
```

Each override replaces the corresponding class list while leaving unspecified keys at their defaults.

## No Go Dependencies Required

This development workflow is **completely independent** of the parent Go package:

- ✅ No Go server needed
- ✅ No Go build process
- ✅ Pure TypeScript/JavaScript development
- ✅ Instant hot reload
- ✅ Mock API responses

You only need the Go server when testing the full integration with server-rendered forms.

## Tips

- **Add console logs**: They'll appear in the browser console
- **Use debugger**: Set breakpoints in the browser DevTools sources tab
- **Inspect network**: Check the Network tab for mock API calls
- **Test responsive**: Use browser DevTools device emulation
