import { describe, it, beforeEach, afterEach, expect } from "vitest";
import { initIcons, registerIconProvider, __resetIconProvidersForTests } from "../src/icons";
import { initBehaviors, __resetBehaviorsForTests } from "../src/behaviors";

beforeEach(() => {
  __resetIconProvidersForTests();
  __resetBehaviorsForTests();
});

afterEach(() => {
  document.body.innerHTML = "";
});

describe("icons runtime", () => {
  it("replaces the vanilla placeholder glyph when a provider resolves an icon", () => {
    document.body.innerHTML = `
      <div class="relative">
        <span class="absolute inset-y-0 start-3 flex items-center" aria-hidden="true">
          <span class="inline-flex size-4 rounded-full bg-gray-200"></span>
        </span>
        <input id="fg-title" data-icon="search" data-icon-source="iconoir">
      </div>
    `;

    registerIconProvider("iconoir", () => {
      return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M1 1h1"/></svg>`;
    });

    initIcons();

    const host = document.querySelector(`span[aria-hidden="true"]`) as HTMLElement;
    expect(host.querySelector("svg")).not.toBeNull();
    expect(host.querySelector(".rounded-full")).toBeNull();
  });

  it("skips replacement when data-icon-raw is present", () => {
    document.body.innerHTML = `
      <div class="relative">
        <span class="absolute inset-y-0 start-3 flex items-center" aria-hidden="true">
          <span class="inline-flex size-5 text-current" aria-hidden="true">
            <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" data-existing="true">
              <path d="M1 1h1"/>
            </svg>
          </span>
        </span>
        <input id="fg-title" data-icon="search" data-icon-source="iconoir" data-icon-raw="<svg></svg>">
      </div>
    `;

    registerIconProvider("iconoir", () => {
      return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M2 2h2"/></svg>`;
    });

    initIcons();

    expect(document.querySelector("svg[data-existing='true']")).not.toBeNull();
    expect(document.querySelector("path[d='M2 2h2']")).toBeNull();
  });

  it("sanitizes unsafe svg content from providers", () => {
    document.body.innerHTML = `
      <div class="relative">
        <span class="absolute inset-y-0 start-3 flex items-center" aria-hidden="true">
          <span class="inline-flex size-4 rounded-full bg-gray-200"></span>
        </span>
        <input id="fg-title" data-icon="search" data-icon-source="iconoir">
      </div>
    `;

    registerIconProvider("iconoir", () => {
      return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">
        <script>alert(1)</script>
        <path d="M1 1h1" onclick="alert(2)"></path>
      </svg>`;
    });

    initIcons();

    const svg = document.querySelector("svg") as SVGSVGElement;
    expect(svg).not.toBeNull();
    expect(svg.querySelector("script")).toBeNull();
    expect(svg.querySelector("path")?.getAttribute("onclick")).toBeNull();
  });

  it("runs as part of initBehaviors", () => {
    document.body.innerHTML = `
      <div class="relative">
        <span class="absolute inset-y-0 start-3 flex items-center" aria-hidden="true">
          <span class="inline-flex size-4 rounded-full bg-gray-200"></span>
        </span>
        <input id="fg-title" data-icon="search" data-icon-source="iconoir">
      </div>
    `;

    registerIconProvider("iconoir", () => {
      return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M1 1h1"/></svg>`;
    });

    initBehaviors();

    expect(document.querySelector("svg")).not.toBeNull();
  });
});

