import { describe, it, beforeEach, afterEach, expect } from "vitest";
import { readFile } from "node:fs/promises";
import { resolve } from "node:path";

type VNode = {
  type: any;
  props: Record<string, any> | null;
  children: any[];
};

function h(type: any, props: Record<string, any> | null, ...children: any[]): VNode {
  return { type, props, children };
}

function resolveVNode(node: any): any {
  if (node == null || typeof node === "string" || typeof node === "number" || typeof node === "boolean") {
    return node;
  }
  if (Array.isArray(node)) {
    return node.map(resolveVNode).flat();
  }
  if (typeof node.type === "function") {
    return resolveVNode(node.type(node.props ?? {}));
  }
  return {
    type: node.type,
    props: node.props ?? {},
    children: (node.children ?? []).map(resolveVNode).flat(),
  };
}

function findAll(tree: any, predicate: (node: any) => boolean, out: any[] = []): any[] {
  if (tree == null) {
    return out;
  }
  if (Array.isArray(tree)) {
    for (const node of tree) {
      findAll(node, predicate, out);
    }
    return out;
  }
  if (typeof tree === "object") {
    if (predicate(tree)) {
      out.push(tree);
    }
    if (Array.isArray(tree.children)) {
      for (const child of tree.children) {
        findAll(child, predicate, out);
      }
    }
  }
  return out;
}

async function loadPreactBundle(): Promise<void> {
  const scriptPath = resolve(
    process.cwd(),
    "..",
    "pkg",
    "renderers",
    "preact",
    "assets",
    "formgen-preact.min.js",
  );
  const code = await readFile(scriptPath, "utf8");
  window.eval(code);
  document.dispatchEvent(new Event("DOMContentLoaded"));
}

beforeEach(() => {
  document.body.innerHTML = "";
  (window as any).preact = {
    h,
    render: (vnode: VNode, mount: HTMLElement) => {
      (mount as any).__tree = resolveVNode(vnode);
    },
  };
  (window as any).FormgenRelationships = { initRelationships: () => {} };
});

afterEach(() => {
  delete (window as any).preact;
  delete (window as any).FormgenRelationships;
  document.body.innerHTML = "";
});

describe("preact renderer runtime widgets", () => {
  it("renders json-editor widget as a textarea with JSON content", async () => {
    document.body.innerHTML = `
      <div id="formgen-preact-root"></div>
      <script id="formgen-preact-data" type="application/json"></script>
    `;

    const payload = {
      operationId: "jsonEditor",
      fields: [
        {
          name: "settings",
          type: "object",
          uiHints: { widget: "json-editor" },
          metadata: { "component.name": "json_editor" },
          default: { alpha: "beta" },
        },
      ],
    };
    const dataNode = document.getElementById("formgen-preact-data") as HTMLElement;
    dataNode.textContent = JSON.stringify(payload);

    await loadPreactBundle();

    const mount = document.getElementById("formgen-preact-root") as HTMLElement;
    const tree = (mount as any).__tree;
    const textareas = findAll(tree, (node) => node?.type === "textarea" && node?.props?.name === "settings");
    expect(textareas.length).toBeGreaterThan(0);
    const content = textareas[0].children.join("");
    expect(content).toContain(`"alpha": "beta"`);
  });

  it("renders code-editor widget as a textarea", async () => {
    document.body.innerHTML = `
      <div id="formgen-preact-root"></div>
      <script id="formgen-preact-data" type="application/json"></script>
    `;

    const payload = {
      operationId: "codeEditor",
      fields: [
        {
          name: "config",
          type: "string",
          uiHints: { widget: "code-editor" },
          default: "hello",
        },
      ],
    };
    const dataNode = document.getElementById("formgen-preact-data") as HTMLElement;
    dataNode.textContent = JSON.stringify(payload);

    await loadPreactBundle();

    const mount = document.getElementById("formgen-preact-root") as HTMLElement;
    const tree = (mount as any).__tree;
    const textareas = findAll(tree, (node) => node?.type === "textarea" && node?.props?.name === "config");
    expect(textareas.length).toBeGreaterThan(0);
  });

  it("renders relationship fields as select controls", async () => {
    document.body.innerHTML = `
      <div id="formgen-preact-root"></div>
      <script id="formgen-preact-data" type="application/json"></script>
    `;

    const payload = {
      operationId: "relationship",
      fields: [
        {
          name: "tags",
          type: "array",
          uiHints: { placeholder: "Select tags" },
          relationship: { kind: "has-many", cardinality: "many", target: "Tag" },
        },
      ],
    };
    const dataNode = document.getElementById("formgen-preact-data") as HTMLElement;
    dataNode.textContent = JSON.stringify(payload);

    await loadPreactBundle();

    const mount = document.getElementById("formgen-preact-root") as HTMLElement;
    const tree = (mount as any).__tree;
    const selects = findAll(tree, (node) => node?.type === "select" && node?.props?.name === "tags");
    expect(selects.length).toBeGreaterThan(0);
    expect(selects[0].props.multiple).toBe("multiple");
  });
});
