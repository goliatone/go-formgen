import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { addArrayItem, initRelationships, resetGlobalRegistry } from "../src/index";

async function flush(): Promise<void> {
  await Promise.resolve();
  await Promise.resolve();
}

describe("array repeaters", () => {
  beforeEach(() => {
    resetGlobalRegistry();
    Element.prototype.scrollIntoView = vi.fn();
  });

  afterEach(() => {
    document.body.innerHTML = "";
    resetGlobalRegistry();
  });

  it("clones prototype controls with the next array index", () => {
    document.body.innerHTML = `
      <form data-formgen-auto-init="true">
        <div
          data-formgen-array-items="true"
          data-formgen-array-name="columns[0].entries"
          data-formgen-array-next-index="1"
          data-formgen-array-prototype-path="columns[0].entries[1]"
          data-formgen-array-prototype-id-prefix="fg-columns-0-entries-1"
        >
          <div id="fg-columns-0-entries-0">
            <input id="fg-columns-0-entries-0-topic_id" name="columns[0].entries[0].topic_id" value="topic-refuge-id">
          </div>
          <template data-formgen-array-prototype="true">
            <div id="fg-columns-0-entries-1">
              <input id="fg-columns-0-entries-1-present" name="columns[0].entries[1]._present" value="true" type="hidden">
              <input id="fg-columns-0-entries-1-state" name="columns[0].entries[1]._row_state" value="existing" type="hidden">
              <input id="fg-columns-0-entries-1-key" name="columns[0].entries[1]._row_key" value="prototype-key" type="hidden">
              <input id="fg-columns-0-entries-1-delete" name="columns[0].entries[1]._delete" value="true" type="hidden">
              <input id="fg-columns-0-entries-1-topic_id" name="columns[0].entries[1].topic_id" value="prototype-topic" disabled required data-formgen-prototype-disabled="true">
              <input id="fg-columns-0-entries-1-topic_slug" name="columns[0].entries[1].topic_slug" value="old-slug" disabled readonly>
              <select id="fg-columns-0-entries-1-kind" name="columns[0].entries[1].kind" disabled data-formgen-prototype-disabled="true">
                <option value="">Select kind</option>
                <option value="topic" selected>Topic</option>
              </select>
            </div>
          </template>
        </div>
        <button type="button" data-formgen-array-action="add">Add topic entry</button>
      </form>
    `;

    const items = document.querySelector<HTMLElement>("[data-formgen-array-items]")!;
    const added = addArrayItem(items);

    expect(added).toHaveLength(1);
    expect(items.dataset.formgenArrayNextIndex).toBe("2");

    const row = document.getElementById("fg-columns-0-entries-1");
    const topic = document.getElementById("fg-columns-0-entries-1-topic_id") as HTMLInputElement;
    const slug = document.getElementById("fg-columns-0-entries-1-topic_slug") as HTMLInputElement;
    const kind = document.getElementById("fg-columns-0-entries-1-kind") as HTMLSelectElement;
    const present = document.getElementById("fg-columns-0-entries-1-present") as HTMLInputElement;
    const state = document.getElementById("fg-columns-0-entries-1-state") as HTMLInputElement;
    const key = document.getElementById("fg-columns-0-entries-1-key") as HTMLInputElement;
    const deleted = document.getElementById("fg-columns-0-entries-1-delete") as HTMLInputElement;

    expect(row).toBeTruthy();
    expect(present.value).toBe("true");
    expect(state.value).toBe("new");
    expect(key.value).toMatch(/^new-/);
    expect(deleted.value).toBe("false");
    expect(topic.name).toBe("columns[0].entries[1].topic_id");
    expect(topic.disabled).toBe(false);
    expect(topic.required).toBe(true);
    expect(topic.value).toBe("");
    expect(topic.hasAttribute("data-formgen-prototype-disabled")).toBe(false);
    expect(slug.name).toBe("columns[0].entries[1].topic_slug");
    expect(slug.disabled).toBe(true);
    expect(slug.readOnly).toBe(true);
    expect(slug.value).toBe("");
    expect(kind.name).toBe("columns[0].entries[1].kind");
    expect(kind.disabled).toBe(false);
    expect(kind.hasAttribute("data-formgen-prototype-disabled")).toBe(false);
    expect(kind.value).toBe("");
    expect(items.children[1]).toBe(row);
  });

  it("initializes relationship fields inside added rows", async () => {
    document.body.innerHTML = `
      <form data-formgen-auto-init="true">
        <div class="field" data-component="array">
          <div>
            <div
              data-formgen-array-items="true"
              data-formgen-array-name="columns[0].entries"
              data-formgen-array-next-index="0"
              data-formgen-array-prototype-path="columns[0].entries[0]"
              data-formgen-array-prototype-id-prefix="fg-columns-0-entries-0"
            >
              <template data-formgen-array-prototype="true">
                <div id="fg-columns-0-entries-0">
                  <select
                    id="fg-columns-0-entries-0-topic_id"
                    name="columns[0].entries[0].topic_id"
                    disabled
                    data-formgen-prototype-disabled="true"
                    data-endpoint-url="/api/topics"
                    data-endpoint-method="GET"
                    data-endpoint-mode="search"
                    data-endpoint-renderer="typeahead"
                    data-endpoint-label-field="label"
                    data-endpoint-value-field="value"
                    data-relationship-cardinality="one"
                  >
                    <option value="">Select Topic</option>
                  </select>
                </div>
              </template>
            </div>
            <button type="button" data-formgen-array-action="add">Add topic entry</button>
          </div>
        </div>
      </form>
    `;

    const registry = await initRelationships({ searchThrottleMs: 0, searchDebounceMs: 0 });
    const add = document.querySelector<HTMLButtonElement>("[data-formgen-array-action='add']")!;
    add.click();
    await flush();

    const topic = document.getElementById("fg-columns-0-entries-0-topic_id") as HTMLSelectElement;
    expect(topic).toBeTruthy();
    expect(topic.disabled).toBe(false);
    expect(topic.name).toBe("columns[0].entries[0].topic_id");
    expect(registry.get(topic)).toBeDefined();
    expect(document.querySelector("[data-fg-typeahead-root='true']")).toBeTruthy();
  });

  it("soft deletes rows that include an explicit delete intent field", async () => {
    document.body.innerHTML = `
      <form data-formgen-auto-init="true">
        <div
          data-formgen-array-items="true"
          data-formgen-array-name="columns[0].entries"
          data-formgen-array-next-index="1"
          data-formgen-array-prototype-path="columns[0].entries[1]"
          data-formgen-array-prototype-id-prefix="fg-columns-0-entries-1"
        >
          <div data-formgen-array-item="true" data-formgen-array-existing="true">
            <input name="columns[0].entries[0]._present" value="true" type="hidden">
            <input name="columns[0].entries[0]._row_state" value="existing" type="hidden">
            <input name="columns[0].entries[0]._row_key" value="row-0" type="hidden">
            <input name="columns[0].entries[0]._delete" value="false" type="hidden">
            <select name="columns[0].entries[0].topic_id">
              <option value="topic-refuge-id" selected>Refuge</option>
            </select>
            <button type="button" data-formgen-array-action="remove">Remove topic entry</button>
          </div>
          <template data-formgen-array-prototype="true">
            <div data-formgen-array-item="true" data-formgen-array-existing="false">
              <input name="columns[0].entries[1]._delete" value="false" type="hidden">
              <input name="columns[0].entries[1].topic_id" disabled data-formgen-prototype-disabled="true">
              <button type="button" data-formgen-array-action="remove">Remove topic entry</button>
            </div>
          </template>
        </div>
        <button type="button" data-formgen-array-action="add">Add topic entry</button>
      </form>
    `;

    await initRelationships();
    const remove = document.querySelector<HTMLButtonElement>("[data-formgen-array-action='remove']")!;
    remove.click();

    const item = document.querySelector<HTMLElement>("[data-formgen-array-item]")!;
    const deleted = document.querySelector<HTMLInputElement>("[name='columns[0].entries[0]._delete']")!;
    const rowKey = document.querySelector<HTMLInputElement>("[name='columns[0].entries[0]._row_key']")!;
    const topic = document.querySelector<HTMLSelectElement>("[name='columns[0].entries[0].topic_id']")!;

    expect(item.hidden).toBe(true);
    expect(deleted.disabled).toBe(false);
    expect(deleted.value).toBe("true");
    expect(rowKey.disabled).toBe(false);
    expect(rowKey.value).toBe("row-0");
    expect(topic.disabled).toBe(true);
  });

  it("removes newly added rows from the DOM instead of submitting delete intent", async () => {
    document.body.innerHTML = `
      <form data-formgen-auto-init="true">
        <div
          data-formgen-array-items="true"
          data-formgen-array-name="columns[0].entries"
          data-formgen-array-next-index="0"
          data-formgen-array-prototype-path="columns[0].entries[0]"
          data-formgen-array-prototype-id-prefix="fg-columns-0-entries-0"
        >
          <template data-formgen-array-prototype="true">
            <div data-formgen-array-item="true" data-formgen-array-existing="false">
              <input name="columns[0].entries[0]._delete" value="false" type="hidden">
              <input name="columns[0].entries[0].topic_id" disabled data-formgen-prototype-disabled="true">
              <button type="button" data-formgen-array-action="remove">Remove topic entry</button>
            </div>
          </template>
        </div>
        <button type="button" data-formgen-array-action="add">Add topic entry</button>
      </form>
    `;

    await initRelationships();
    document.querySelector<HTMLButtonElement>("[data-formgen-array-action='add']")!.click();
    const added = document.querySelector<HTMLElement>("[data-formgen-array-item]")!;
    expect(added.getAttribute("data-formgen-array-existing")).toBe("false");

    added.querySelector<HTMLButtonElement>("[data-formgen-array-action='remove']")!.click();

    expect(document.querySelector("[data-formgen-array-item]")).toBeNull();
    expect(document.querySelector("[name='columns[0].entries[0]._delete']")).toBeNull();
  });

  it("does not use nested child delete sentinels when removing a parent item", async () => {
    document.body.innerHTML = `
      <form data-formgen-auto-init="true">
        <div
          data-formgen-array-items="true"
          data-formgen-array-name="sections"
          data-formgen-array-next-index="1"
          data-formgen-array-prototype-path="sections[1]"
          data-formgen-array-prototype-id-prefix="fg-sections-1"
        >
          <div data-formgen-array-item="true" data-formgen-array-existing="true" id="section-0">
            <input name="sections[0].title" value="Primary">
            <div data-formgen-array-items="true" data-formgen-array-name="sections[0].links">
              <div data-formgen-array-item="true" data-formgen-array-existing="true">
                <input name="sections[0].links[0]._delete" value="false" type="hidden">
                <input name="sections[0].links[0].label" value="Refuge">
              </div>
              <template data-formgen-array-prototype="true"></template>
            </div>
            <button type="button" data-formgen-array-action="remove" id="remove-section">Remove section</button>
          </div>
          <template data-formgen-array-prototype="true"></template>
        </div>
        <button type="button" data-formgen-array-action="add">Add section</button>
      </form>
    `;

    await initRelationships();
    document.getElementById("remove-section")!.click();

    expect(document.getElementById("section-0")).toBeNull();
    expect(document.querySelector<HTMLInputElement>("[name='sections[0].links[0]._delete']")?.value).toBeUndefined();
  });
});
