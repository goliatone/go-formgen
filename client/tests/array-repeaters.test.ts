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
              <input id="fg-columns-0-entries-1-topic_id" name="columns[0].entries[1].topic_id" value="prototype-topic" disabled required>
              <select id="fg-columns-0-entries-1-kind" name="columns[0].entries[1].kind" disabled>
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
    const kind = document.getElementById("fg-columns-0-entries-1-kind") as HTMLSelectElement;

    expect(row).toBeTruthy();
    expect(topic.name).toBe("columns[0].entries[1].topic_id");
    expect(topic.disabled).toBe(false);
    expect(topic.required).toBe(true);
    expect(topic.value).toBe("");
    expect(kind.name).toBe("columns[0].entries[1].kind");
    expect(kind.disabled).toBe(false);
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
});
