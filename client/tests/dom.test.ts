import { describe, it, expect, beforeEach } from "vitest";
import {
  locateRelationshipFields,
  readDataset,
  isMultiSelect,
  syncHiddenInputs,
  syncJsonInput,
  attachHiddenInputSync,
} from "../src/dom";

describe("dom helpers", () => {
  beforeEach(() => {
    document.body.innerHTML = `
      <form data-formgen-auto-init="true">
        <select id="author" name="article[author_id]" data-endpoint-url="/api/authors" data-relationship-cardinality="one"></select>
        <select id="tags" name="article[tag_ids]" data-endpoint-url="/api/tags" data-relationship-cardinality="many" multiple>
          <option value="a" selected>A</option>
          <option value="b">B</option>
        </select>
      </form>
    `;
  });

  it("locates fields with endpoint metadata", () => {
    const form = document.querySelector("form")!;
    const fields = locateRelationshipFields(form);
    expect(fields.length).toBe(2);
    expect(fields[0].id).toBe("author");
    expect(fields[1].id).toBe("tags");
  });

  it("reads dataset values", () => {
    const select = document.getElementById("author") as HTMLElement;
    const dataset = readDataset(select);
    expect(dataset.endpointUrl).toBe("/api/authors");
    expect(dataset.relationshipCardinality).toBe("one");
  });

  it("synchronises hidden inputs for multi-select fields", () => {
    const select = document.getElementById("tags") as HTMLSelectElement;
    expect(isMultiSelect(select)).toBe(true);
    attachHiddenInputSync(select);
    const hidden = select.parentElement!.querySelectorAll(
      '[data-relationship-hidden] input[type="hidden"]'
    );
    expect(hidden.length).toBe(1);
    expect(hidden[0].value).toBe("a");

    // Change selection and ensure hidden inputs refresh.
    Array.from(select.options).forEach((option) => {
      option.selected = option.value === "b";
    });
    syncHiddenInputs(select);
    const refreshed = select.parentElement!.querySelectorAll(
      '[data-relationship-hidden] input[type="hidden"]'
    );
    expect(refreshed.length).toBe(1);
    expect(refreshed[0].value).toBe("b");
  });

  it("preserves encoded enum values in hidden and json mirrors", () => {
    const encoded = "__fg_enum_v1:eyJ0IjoiYm9vbCIsInYiOnRydWV9";
    document.body.innerHTML = `
      <form>
        <select id="flags" name="flags" multiple>
          <option value="${encoded}" selected>true</option>
        </select>
        <select id="choice" name="choice">
          <option value="${encoded}" selected>true</option>
        </select>
      </form>
    `;

    const flags = document.getElementById("flags") as HTMLSelectElement;
    syncHiddenInputs(flags);
    const hidden = flags.parentElement!.querySelector<HTMLInputElement>(
      '[data-relationship-hidden] input[type="hidden"]'
    );
    expect(hidden?.name).toBe("flags[]");
    expect(hidden?.value).toBe(encoded);

    const choice = document.getElementById("choice") as HTMLSelectElement;
    syncJsonInput(choice);
    const json = choice.parentElement!.querySelector<HTMLInputElement>(
      '[data-relationship-json]'
    );
    expect(json?.name).toBe("choice");
    expect(json?.value).toBe(JSON.stringify(encoded));
  });
});
