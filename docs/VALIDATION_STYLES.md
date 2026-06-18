# Preline Validation Styles Implementation

**Date:** 2025-11-12
**Status:** ✅ Complete

## Overview

Implemented Preline validation state styling for form inputs to provide visual feedback when fields have validation errors. The implementation follows Preline's design system from https://preline.co/docs/input.html#validation-states.

---

## Changes Made

### 1. Template Updates (Go)

Updated all input component templates to conditionally apply validation classes based on `validation_state` metadata.

#### Files Modified:
- [pkg/renderers/vanilla/templates/components/input.tmpl](pkg/renderers/vanilla/templates/components/input.tmpl)
- [pkg/renderers/vanilla/templates/components/textarea.tmpl](pkg/renderers/vanilla/templates/components/textarea.tmpl)
- [pkg/renderers/vanilla/templates/components/select.tmpl](pkg/renderers/vanilla/templates/components/select.tmpl)

#### Implementation:

**Before:**
```html
<input class="... border border-gray-200 ... focus:border-blue-500 focus:ring-blue-500 ...">
```

**After:**
```html
<input class="... border ...
  {% if validation_state == "invalid" %}
    border-red-500 focus:border-red-500 focus:ring-red-500 dark:border-red-500
  {% else %}
    border-gray-200 focus:border-blue-500 focus:ring-blue-500 dark:border-gray-700 dark:focus:ring-gray-600
  {% endif %}">
```

**Key Changes:**
- Split `class` attribute to conditionally include border colors
- Invalid state: Red borders (`border-red-500`)
- Valid state: Gray borders (`border-gray-200`)
- Applied to all focus states and dark mode variants

---

### 2. Runtime Error Handler Updates (TypeScript)

Updated the error rendering system to match Preline styling and dynamically manage validation classes.

#### File Modified:
- [client/src/errors.ts](client/src/errors.ts)

#### Changes:

**Error Message Styling (Line 72):**
```typescript
// Before
target.className = "formgen-error text-sm text-red-600 dark:text-red-400";

// After
target.className = "formgen-error text-xs text-red-600 mt-2 dark:text-red-400";
```

**Preline Specifications:**
- `text-xs` - Smaller text size (was `text-sm`)
- `mt-2` - Margin top for spacing below input

**Dynamic Class Management (Lines 108-137):**

Added new `addValidationClasses()` function that:
1. Finds the actual input element (handles containers)
2. Toggles between valid and invalid class sets
3. Supports `<input>`, `<textarea>`, and `<select>` elements

```typescript
function addValidationClasses(element: HTMLElement, isInvalid: boolean): void {
  // Find the actual input/textarea/select element
  let target: HTMLElement | null = element;

  if (!(element instanceof HTMLInputElement ||
        element instanceof HTMLTextAreaElement ||
        element instanceof HTMLSelectElement)) {
    // If element is a container, find the input inside
    target = element.querySelector<HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement>(
      'input, textarea, select'
    );
  }

  if (!target) {
    return;
  }

  const invalidClasses = ['border-red-500', 'focus:border-red-500', 'focus:ring-red-500', 'dark:border-red-500'];
  const validClasses = ['border-gray-200', 'focus:border-blue-500', 'focus:ring-blue-500', 'dark:border-gray-700', 'dark:focus:ring-gray-600'];

  if (isInvalid) {
    // Remove valid classes, add invalid classes
    validClasses.forEach(cls => target!.classList.remove(cls));
    invalidClasses.forEach(cls => target!.classList.add(cls));
  } else {
    // Remove invalid classes, add valid classes
    invalidClasses.forEach(cls => target!.classList.remove(cls));
    validClasses.forEach(cls => target!.classList.add(cls));
  }
}
```

**Integration:**
- Called from `markElementInvalid()` when showing errors
- Called from `clearInvalidState()` when clearing errors

---

## Preline Design System Compliance

### Validation State: Invalid

**Preline Spec:**
```html
<input class="... border-red-500 focus:border-red-500 focus:ring-red-500 ..." />
<p class="text-xs text-red-600 mt-2">Please enter a valid email address.</p>
```

**Our Implementation:**
```html
<!-- Server-rendered (Go template) -->
<input class="... border-red-500 focus:border-red-500 focus:ring-red-500 dark:border-red-500 ..."
       aria-invalid="true"
       data-validation-state="invalid" />

<!-- Runtime-added (TypeScript) -->
<p class="formgen-error text-xs text-red-600 mt-2 dark:text-red-400"
   role="status"
   aria-live="polite">
  Please enter a valid email address.
</p>
```

**Differences:**
- ✅ Added dark mode support (`dark:border-red-500`, `dark:text-red-400`)
- ✅ Added accessibility attributes (`aria-invalid`, `role`, `aria-live`)
- ✅ Added `formgen-error` class for custom targeting

### Validation State: Valid/Normal

**Preline Spec:**
```html
<input class="... border-gray-200 focus:border-blue-500 focus:ring-blue-500 ..." />
```

**Our Implementation:**
```html
<input class="... border-gray-200 focus:border-blue-500 focus:ring-blue-500 dark:border-gray-700 dark:focus:ring-gray-600 ..." />
```

**Differences:**
- ✅ Enhanced dark mode support for all states

---

## How It Works

### Server-Side Flow (Go)

1. **Validation occurs** (e.g., form submission, backend validation)
2. **Renderer receives errors** via `RenderOptions.Errors` (Phase 17)
3. **Field metadata set** to `field.metadata["validation.state"] = "invalid"`
4. **Template conditionally applies** red border classes
5. **HTML rendered** with invalid state baked in

**Example:**
```go
renderer.Render(ctx, form, RenderOptions{
  Errors: map[string]string{
    "email": "Please enter a valid email address",
  },
})
```

Submitted payloads can use the shared parser before rendering errors:

```go
parsed, _ := submission.ParseRequest(form, r)
issues := append(parsed.Issues, submission.Validate(form, parsed.Values)...)
fieldErrors, formErrors := submission.IssuesToFieldAndFormErrors(form, issues)

renderer.Render(ctx, form, render.RenderOptions{
  Values: parsed.Values,
  Errors: fieldErrors,
  FormErrors: formErrors,
})
```

### Client-Side Flow (TypeScript)

1. **Runtime validation fails** (e.g., relationship field, file upload)
2. **`renderFieldError(element, message)`** called
3. **Error message element created/updated** with Preline classes
4. **`markElementInvalid()`** sets attributes:
   - `aria-invalid="true"`
   - `data-validation-state="invalid"`
   - Calls `addValidationClasses(element, true)`
5. **Classes dynamically added** to input element:
   - Removes: `border-gray-200`, `focus:border-blue-500`, etc.
   - Adds: `border-red-500`, `focus:border-red-500`, etc.

**Example:**
```typescript
import { renderFieldError } from '@goliatone/formgen-runtime';

// Show error
renderFieldError(inputElement, 'File size exceeds maximum');

// Clear error
renderFieldError(inputElement, null);
```

---

## Visual Examples

### Normal State
```
┌─────────────────────────────────┐
│ Email Address                   │  ← Label
├─────────────────────────────────┤
│ user@example.com                │  ← Input (gray border)
└─────────────────────────────────┘
```

### Invalid State
```
┌─────────────────────────────────┐
│ Email Address                   │  ← Label
├─────────────────────────────────┤
│ invalid-email                   │  ← Input (RED border)
└─────────────────────────────────┘
  ⚠ Please enter a valid email      ← Error message (red text, xs size)
```

### Focus State (Invalid)
```
┌─────────────────────────────────┐
│ Email Address                   │  ← Label
├═════════════════════════════════┤
│ invalid-email                   │  ← Input (RED border + ring)
└═════════════════════════════════┘  ← Ring effect in red
  ⚠ Please enter a valid email      ← Error message
```

---

## CSS Classes Reference

### Invalid State Classes
```css
border-red-500              /* Main border color */
focus:border-red-500        /* Border on focus */
focus:ring-red-500          /* Ring effect on focus */
dark:border-red-500         /* Dark mode border */
```

### Valid State Classes
```css
border-gray-200             /* Main border color */
focus:border-blue-500       /* Border on focus */
focus:ring-blue-500         /* Ring effect on focus */
dark:border-gray-700        /* Dark mode border */
dark:focus:ring-gray-600    /* Dark mode ring on focus */
```

### Error Message Classes
```css
formgen-error               /* Custom identifier */
text-xs                     /* Extra small text (Preline spec) */
text-red-600                /* Error text color */
mt-2                        /* Margin top (Preline spec) */
dark:text-red-400           /* Dark mode text color */
```

---

## Accessibility Features

All implementations maintain accessibility:

### ARIA Attributes
```html
<input aria-invalid="true"
       data-validation-state="invalid"
       data-validation-message="Error message here" />

<p role="status"
   aria-live="polite"
   aria-atomic="true"
   data-relationship-error="true">
  Error message here
</p>
```

**Purpose:**
- `aria-invalid` - Announces invalid state to screen readers
- `role="status"` - Identifies error container as status region
- `aria-live="polite"` - Announces changes without interrupting
- `aria-atomic="true"` - Reads entire message on change

---

## Browser/Framework Compatibility

### Server-Rendered (Go Templates)
- ✅ Works in all browsers (classes are static HTML)
- ✅ Works without JavaScript (progressive enhancement)
- ✅ SSR-friendly (React, Vue, etc.)

### Client-Rendered (TypeScript)
- ✅ Modern browsers (ES6+)
- ✅ Framework-agnostic (vanilla JS)
- ✅ Works with relationship fields, file uploads, custom validation

---

## Testing

### Manual Testing

**Test invalid state:**
```typescript
import { renderFieldError } from '@goliatone/formgen-runtime';

const input = document.querySelector('#email');
renderFieldError(input, 'This field is required');

// Verify:
// 1. Input has red border
// 2. Error message appears below with red text (xs size, mt-2)
// 3. Focus shows red ring
// 4. Dark mode shows appropriate colors
```

**Test clear state:**
```typescript
renderFieldError(input, null);

// Verify:
// 1. Input returns to gray border
// 2. Error message hidden
// 3. Focus shows blue ring
// 4. Classes properly swapped
```

### Integration Points

**File Uploader:**
- [client/src/components/file-uploader/index.ts:266](client/src/components/file-uploader/index.ts:266) - Calls `clearFieldError()` on success
- [client/src/components/file-uploader/index.ts:271](client/src/components/file-uploader/index.ts:271) - Calls `showError()` which uses `renderFieldError()`

**Validation Runtime:**
- [client/src/validation.ts](client/src/validation.ts) - Uses `renderFieldError()` for validation errors

**Relationship Fields:**
- [client/src/resolver.ts](client/src/resolver.ts) - Uses error system for relationship validation

---

## Migration Notes

### For Existing Projects

**No breaking changes!** The validation styling is backwards compatible:

1. **Server-rendered forms** automatically get new styles on next deployment
2. **Client-side forms** using `renderFieldError()` get new styles automatically
3. **Custom error renderers** can override via `registerErrorRenderer()`

### Customization

**Override error message styling:**
```typescript
import { registerErrorRenderer } from '@goliatone/formgen-runtime';

registerErrorRenderer('custom', ({ element, message }) => {
  if (message) {
    // Your custom error display
    const error = document.createElement('span');
    error.className = 'my-custom-error-class';
    error.textContent = message;
    element.parentElement?.appendChild(error);
  }
});
```

**Use custom renderer:**
```html
<input data-validation-renderer="custom" />
```

---

## Future Enhancements

### Potential Improvements

1. **Success state (green border):**
   ```typescript
   markElementValid(element, 'Looks good!');
   ```

2. **Warning state (yellow border):**
   ```typescript
   markElementWarning(element, 'Please verify this value');
   ```

3. **Icon support:**
   ```html
   <div class="relative">
     <input class="pe-10 ..." />
     <svg class="absolute inset-y-0 end-3 ...">...</svg>
   </div>
   ```

4. **Animated transitions:**
   ```css
   .border-transition {
     transition: border-color 0.2s ease;
   }
   ```

---

## Related Documentation

- [Preline Input Validation](https://preline.co/docs/input.html#validation-states)
- [client/src/errors.ts](client/src/errors.ts) - Error rendering system
- [pkg/renderers/vanilla/templates/components/](pkg/renderers/vanilla/templates/components/) - Go templates
- [Phase 17 (JS_TSK.md)](JS_TSK.md:315-327) - Form prefill & server error rendering
- [Phase N (JS_TSK.md)](JS_TSK.md:221-233) - Validation hooks & error UX

---

## Summary

✅ **Implementation complete** - All templates and runtime code updated
✅ **Preline compliant** - Matches design system specifications
✅ **Accessible** - Full ARIA support maintained
✅ **Backwards compatible** - No breaking changes
✅ **Framework agnostic** - Works server-side and client-side

The validation styling now provides clear, consistent visual feedback following Preline's design language while maintaining our existing accessibility standards and progressive enhancement philosophy.
