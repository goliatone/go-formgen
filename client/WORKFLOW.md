# Client Development Workflow

## TL;DR

```bash
# Start live dev server (no Go needed!)
npm run dev

# Edit src/ files → browser auto-refreshes
# When done, run tests
npm test

# Build for production
npm run build
```

---

## Full Workflow Guide

### 1. Feature Development (Pure JavaScript/TypeScript)

**Goal**: Develop and test features without needing the Go server.

```bash
cd client/
npm run dev
```

**What happens:**
- Vite starts at http://localhost:5173
- Browser opens automatically
- `dev/index.html` sandbox loads
- Hot reload enabled

**Your workflow:**
1. Edit files in `src/`
2. Browser refreshes automatically
3. Check console/DevTools
4. Iterate until feature works

**Example: Adding a new renderer**

```typescript
// 1. Create src/renderers/my-renderer.ts
export function myRenderer(context) {
  const { element, options } = context;
  // Render logic here
}

// 2. Register in src/index.ts or src/registry.ts
import { myRenderer } from "./renderers/my-renderer";
registry.registerRenderer("my-renderer", myRenderer);

// 3. Test in dev/index.html
// Add: data-endpoint-renderer="my-renderer" to a field

// 4. Browser auto-reloads, your renderer is active!
```

**Event wiring tip (relationships)**
- Prefer `formgen:relationship:update` (see `src/relationship-events.ts`) for internal wiring (`kind:"options" | "selection" | "search"`).
- Avoid listening to native `change` inside renderers/components; keep it for external integration only.

### 2. CSS/Styling Changes

**Prototyping:**
```html
<!-- Edit dev/index.html <style> block for quick experiments -->
<style>
  .fg-chip-select {
    /* Your experimental styles */
  }
</style>
```

**Making permanent:**
```bash
# 1. Edit the source CSS (in parent Go package)
vim ../pkg/renderers/vanilla/assets/formgen-vanilla.css

# 2. Rebuild client to embed updated CSS
npm run build

# 3. If testing with Go server, restart it to pick up changes
cd ../../examples/http
go run main.go
```

### 3. Testing

**Run tests once:**
```bash
npm test
```

**Watch mode (recommended during development):**
```bash
npm test -- --watch
```

**Test workflow:**
```typescript
// tests/my-feature.test.ts
import { describe, it, expect } from "vitest";

describe("my feature", () => {
  it("works correctly", () => {
    // Your test
  });
});
```

Save the file → tests auto-run!

### 4. Building for Production

```bash
npm run build
```

**Output:**
- `dist/esm/` - For npm/bundlers
- `dist/browser/` - For Go server and CDN
- `dist/types/` - TypeScript definitions

**Bundle analysis:**
```bash
npm run build:stats
```

### 5. Integration with Go Server

**When to do this:** After features work in the dev sandbox.

```bash
# 1. Build the client
cd client/
npm run build

# 2. Start Go server
cd ../examples/http
go run main.go --source schema.json --renderer vanilla

# 3. Open http://localhost:8080
```

The Go server serves built files from `client/dist/browser/`.

**Watch mode (auto-rebuild):**

Terminal 1:
```bash
cd client/
npm run watch
```

Terminal 2:
```bash
cd examples/http
go run main.go
```

Now client rebuilds automatically, but you need to refresh the browser.

---

## Workflow Comparison

| Task | Dev Server | Go Server |
|------|-----------|-----------|
| **Feature development** | ✅ Recommended | ❌ Slow |
| **Hot reload** | ✅ Instant | ❌ Manual refresh |
| **TypeScript errors** | ✅ Shows in browser | ❌ Need to build |
| **Mock data** | ✅ Built-in | ❌ Need backend |
| **Integration testing** | ❌ | ✅ Full stack |
| **Production verification** | ❌ | ✅ Real server |

---

## Common Scenarios

### Scenario: Add a new configuration option

```bash
# 1. Start dev server
npm run dev

# 2. Edit src/config.ts
export interface GlobalConfig {
  myNewOption?: boolean;  // Add this
}

# 3. Edit src/index.ts to use it
export async function initRelationships(config?: GlobalConfig) {
  if (config?.myNewOption) {
    // Your logic
  }
}

# 4. Test in dev/index.html
<script type="module">
  import { initRelationships } from "../src/index.ts";
  await initRelationships({ myNewOption: true });
</script>

# 5. Browser reloads → test it!

# 6. Write test
# tests/config.test.ts

# 7. Run tests
npm test

# 8. Build
npm run build
```

### Scenario: Fix a bug in the chips renderer

```bash
# 1. Start dev server
npm run dev

# 2. Open browser DevTools

# 3. Edit src/renderers/chips.ts

# 4. Save → browser refreshes → bug fixed?

# 5. Write regression test
# tests/chips.test.ts

# 6. Verify test catches the bug
npm test

# 7. Build for production
npm run build
```

### Scenario: Exercise the typeahead renderer

```bash
# 1. Start dev server
npm run dev

# 2. Open the sandbox form in the browser (Author field uses typeahead)

# 3. Edit src/renderers/typeahead.ts (and shared helpers in src/renderers/relationship-utils.ts when behaviour overlaps with chips)

# 4. Interact in the browser: type a query, navigate with Arrow keys, press Enter/Escape, Clear, and ensure the native select stays in sync

# 5. Extend tests in tests/runtime.test.ts

# 6. Run the suite
npm test -- --run tests/runtime.test.ts

# 7. Ship it
npm run build
```

### Scenario: Update dependencies

```bash
# 1. Update package.json

# 2. Install
npm install

# 3. Test still works
npm test

# 4. Dev server still works
npm run dev

# 5. Build still works
npm run build

# 6. Verify bundle size didn't explode
npm run build:stats
```

---

## Tips & Tricks

### Fast Iteration
- Keep `npm run dev` running in one terminal
- Edit files in your IDE
- Check browser (auto-refreshes)
- Use browser DevTools for debugging

### Debugging
```typescript
// Add debug logging
console.log('[DEBUG]', { element, options });

// Use debugger
debugger; // Browser will pause here

// Check network calls
// Open Network tab in DevTools
```

### Mock API Customization
Edit `dev/index.html` around line 550:

```javascript
window.fetch = async (input, init) => {
  const url = new URL(typeof input === "string" ? input : input.url);

  // Add your custom endpoints
  switch (url.pathname) {
    case "/api/my-endpoint":
      return response([{ value: "1", label: "My Option" }]);
  }
};
```

### CSS Hot Reload Trick
For instant CSS changes without rebuilding:

1. Start dev server: `npm run dev`
2. Edit `<style>` in `dev/index.html`
3. See changes instantly
4. When satisfied, copy to `pkg/renderers/vanilla/assets/formgen-vanilla.css`
5. Build: `npm run build`

### Testing Edge Cases
Edit the mock data in `dev/index.html`:

```javascript
// Test with empty array
const tags = [];

// Test with huge array
const tags = Array.from({ length: 1000 }, (_, i) => ({
  value: `tag-${i}`,
  label: `Tag ${i}`
}));

// Test with long labels
const tags = [{
  value: "1",
  label: "This is an extremely long label that should truncate..."
}];
```

---

## Troubleshooting

**Dev server won't start:**
```bash
# Kill any processes on port 5173
lsof -ti:5173 | xargs kill -9

# Try again
npm run dev
```

**Changes not reflecting:**
- Check browser console for errors
- Hard refresh: Cmd+Shift+R (Mac) or Ctrl+Shift+R (Windows)
- Check if file saved correctly
- Restart dev server

**Build fails:**
```bash
# Clean and rebuild
rm -rf dist/
npm run build
```

**Tests fail:**
```bash
# Run specific test file
npm test -- tests/my-test.test.ts

# Run with verbose output
npm test -- --reporter=verbose
```

---

## Next Steps

- Read [dev/README.md](dev/README.md) for sandbox details
- Check [../JS_TDD.md](../JS_TDD.md) for technical design
- See test files in `tests/` for examples
