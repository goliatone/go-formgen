import { describe, expect, it } from "vitest";
import rendererGolden from "@assets/vanilla/testdata/form_output.golden.html?raw";
import { vanillaFormHtml } from "../dev/templates";

type Violation = {
  field: string;
  type: string;
  count: number;
};

function collectChromeViolations(markup: string): Violation[] {
  const dom = new DOMParser().parseFromString(markup, "text/html");
  const wrappers = Array.from(dom.querySelectorAll<HTMLElement>("[data-component]"));
  const violations: Violation[] = [];

  for (const wrapper of wrappers) {
    const chromeChildren = Array.from(wrapper.children).filter(
      (child): child is HTMLElement =>
        child instanceof HTMLElement && typeof child.dataset.formgenChrome === "string"
    );
    if (chromeChildren.length === 0) {
      continue;
    }
    const counts = chromeChildren.reduce<Record<string, number>>((acc, child) => {
      const type = child.dataset.formgenChrome ?? "unknown";
      acc[type] = (acc[type] ?? 0) + 1;
      return acc;
    }, {});

    for (const type of ["label", "description", "help"]) {
      const count = counts[type] ?? 0;
      if (count > 1) {
        violations.push({
          field: identifyWrapper(wrapper),
          type,
          count,
        });
      }
    }
  }

  return violations;
}

function identifyWrapper(wrapper: HTMLElement): string {
  const component = wrapper.getAttribute("data-component") ?? "unknown";
  const id =
    wrapper.querySelector<HTMLElement>(":scope > [data-formgen-chrome='label']")?.getAttribute("for") ??
    wrapper.querySelector<HTMLElement>(":scope > [id]")?.getAttribute("id") ??
    "anonymous";
  return `${component}:${id}`;
}

describe("chrome regression", () => {
  it("ensures renderer golden remains single-source for chrome markup", () => {
    expect(collectChromeViolations(rendererGolden)).toEqual([]);
  });

  it("keeps sandbox fixture chrome identical to renderer output", () => {
    expect(collectChromeViolations(vanillaFormHtml)).toEqual([]);
  });
});
