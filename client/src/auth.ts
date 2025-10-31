import type { EndpointAuth } from "./config";

function datasetKey(attribute: string): string {
  return attribute
    .replace(/^data-/, "")
    .split(/[-_:]/)
    .filter(Boolean)
    .map((part, index) =>
      index === 0 ? part.toLowerCase() : part.charAt(0).toUpperCase() + part.slice(1).toLowerCase()
    )
    .join("");
}

function readMetaContent(name: string): string | undefined {
  if (typeof document === "undefined") {
    return undefined;
  }
  const meta = document.querySelector(`meta[name="${name}"]`);
  return meta?.getAttribute("content") ?? undefined;
}

function readDataAttribute(element: HTMLElement | null, attribute: string): string | undefined {
  if (!element) {
    return undefined;
  }
  if (element.hasAttribute(attribute)) {
    return element.getAttribute(attribute) ?? undefined;
  }
  const key = datasetKey(attribute);
  const value = (element.dataset as Record<string, string | undefined>)[key];
  return value ?? undefined;
}

function resolveToken(source: string | undefined, element: HTMLElement | null): string | undefined {
  if (!source) {
    return undefined;
  }
  if (source.startsWith("meta:")) {
    return readMetaContent(source.slice("meta:".length));
  }
  if (source.startsWith("data:")) {
    return readDataAttribute(element, source.slice("data:".length));
  }
  if (source.startsWith("element:")) {
    return readDataAttribute(element, source.slice("element:".length));
  }
  if (source.includes("=")) {
    // Allow literal token declarations such as "token=abc" for testing.
    const [, value] = source.split("=");
    return value;
  }
  return readDataAttribute(element, source) ?? readMetaContent(source);
}

/**
 * resolveAuthHeaders produces authentication headers based on runtime context.
 */
export function resolveAuthHeaders(
  auth: EndpointAuth | undefined,
  element: HTMLElement | null
): Record<string, string> {
  if (!auth || auth.strategy !== "header") {
    return {};
  }

  const header = auth.header ?? "Authorization";
  const token = resolveToken(auth.source ?? "data-auth-token", element);

  if (!token) {
    return {};
  }

  const value = auth.prefix ? `${auth.prefix.trim()} ${token}`.trim() : token;
  return { [header]: value };
}
