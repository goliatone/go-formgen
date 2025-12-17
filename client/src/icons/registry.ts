export type IconProvider = (name: string) => string | null | undefined;

export interface IconInitRecord {
  element: HTMLElement;
  name: string;
  source: string;
  rendered: boolean;
}

export interface IconInitResult {
  records: IconInitRecord[];
}

const providers = new Map<string, IconProvider>();

export function registerIconProvider(source: string, provider: IconProvider): void {
  const normalized = normalize(source);
  if (!normalized || typeof provider !== "function") {
    return;
  }
  providers.set(normalized, provider);
}

export function initIcons(root: Document | HTMLElement = document): IconInitResult {
  const elements = collectIconElements(root);
  const records: IconInitRecord[] = [];

  for (const element of elements) {
    const name = normalize(element.getAttribute("data-icon"));
    const source = normalize(element.getAttribute("data-icon-source"));

    if (!name || !source) {
      continue;
    }

    const hasRaw = normalize(element.getAttribute("data-icon-raw")) !== "";
    if (hasRaw) {
      records.push({ element, name, source, rendered: false });
      continue;
    }

    const provider = providers.get(source);
    if (!provider) {
      records.push({ element, name, source, rendered: false });
      continue;
    }

    const svgMarkup = safeInvokeProvider(provider, name);
    const svg = parseSvgMarkup(svgMarkup, element.ownerDocument ?? document);
    if (!svg) {
      records.push({ element, name, source, rendered: false });
      continue;
    }

    const host = resolveIconHost(element);
    if (!host) {
      records.push({ element, name, source, rendered: false });
      continue;
    }

    while (host.firstChild) {
      host.removeChild(host.firstChild);
    }

    const wrapper = (element.ownerDocument ?? document).createElement("span");
    wrapper.className = "inline-flex size-5 text-current";
    wrapper.setAttribute("aria-hidden", "true");
    wrapper.appendChild(svg);
    host.appendChild(wrapper);

    records.push({ element, name, source, rendered: true });
  }

  return { records };
}

export function __resetIconProvidersForTests(): void {
  providers.clear();
}

function collectIconElements(root: Document | HTMLElement): HTMLElement[] {
  const scope = root instanceof Document ? root : root;
  const elements = Array.from(
    scope.querySelectorAll<HTMLElement>("[data-icon][data-icon-source]"),
  );
  if (root instanceof HTMLElement && root.hasAttribute("data-icon") && root.hasAttribute("data-icon-source")) {
    elements.unshift(root);
  }
  return Array.from(new Set(elements));
}

function resolveIconHost(element: HTMLElement): HTMLElement | null {
  const parent = element.parentElement;
  if (!parent) {
    return null;
  }

  const previous = element.previousElementSibling;
  if (previous instanceof HTMLElement && previous.tagName === "SPAN" && previous.getAttribute("aria-hidden") === "true") {
    return previous;
  }

  return parent.querySelector<HTMLElement>(`:scope > span[aria-hidden="true"]`);
}

function safeInvokeProvider(provider: IconProvider, name: string): string {
  try {
    return provider(name) ?? "";
  } catch (error) {
    console.warn(`[formgen:icons] provider for "${name}" failed:`, error);
    return "";
  }
}

function parseSvgMarkup(markup: string, doc: Document): SVGSVGElement | null {
  const trimmed = markup?.trim();
  if (!trimmed) {
    return null;
  }

  if (typeof DOMParser === "undefined") {
    return null;
  }

  const parser = new DOMParser();
  const parsed = parser.parseFromString(trimmed, "image/svg+xml");
  const svg = parsed.querySelector("svg");
  if (!svg) {
    return null;
  }

  sanitizeSvg(svg);

  if (typeof doc.importNode === "function") {
    return doc.importNode(svg, true) as SVGSVGElement;
  }
  return svg;
}

function sanitizeSvg(svg: SVGSVGElement): void {
  svg.querySelectorAll("script, foreignObject, iframe, object, embed").forEach((node) => {
    node.parentNode?.removeChild(node);
  });

  const all = [svg as unknown as Element, ...Array.from(svg.querySelectorAll("*"))];
  for (const element of all) {
    const attrs = Array.from(element.attributes);
    for (const attr of attrs) {
      const name = attr.name.toLowerCase();
      const value = attr.value.trim().toLowerCase();

      if (name.startsWith("on")) {
        element.removeAttribute(attr.name);
        continue;
      }

      if (name === "href" || name === "xlink:href" || name === "src") {
        const safe =
          value === "" ||
          value.startsWith("#") ||
          value.startsWith("data:image/");
        if (!safe || value.startsWith("javascript:")) {
          element.removeAttribute(attr.name);
        }
      }
    }
  }
}

function normalize(value: string | null | undefined): string {
  return value?.trim().toLowerCase() ?? "";
}

