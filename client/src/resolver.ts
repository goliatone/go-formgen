import {
  type EndpointConfig,
  type FieldConfig,
  type Option,
  type RendererContext,
  type ResolverContext,
  type ResolverRequest,
  type ResolvedGlobalConfig,
  type CacheAdapter,
  type FetchResult,
  type CacheConfig,
} from "./config";
import { ResolverError, ResolverAbortError } from "./errors";
import { resolveAuthHeaders } from "./auth";
import {
  attachHiddenInputSync,
  attachJsonInputSync,
  isMultiSelect,
  syncHiddenInputs,
  syncJsonInput,
  setFieldError,
  clearFieldError,
  readElementValue,
} from "./dom";

const DYNAMIC_TOKEN_PATTERN = /\{\{\s*([^}]+)\s*\}\}/g;
const DEFAULT_ERROR_MESSAGE = "Unable to load options.";

function now(): number {
  if (typeof performance !== "undefined" && typeof performance.now === "function") {
    return performance.now();
  }
  return Date.now();
}

function delay(ms: number): Promise<void> {
  return new Promise((resolve) => {
    setTimeout(resolve, ms);
  });
}

function matchesFieldName(candidate: string | undefined, reference: string): boolean {
  if (!candidate) {
    return false;
  }
  if (candidate === reference) {
    return true;
  }
  if (candidate === `${reference}[]`) {
    return true;
  }
  if (candidate.endsWith(`[${reference}]`)) {
    return true;
  }
  if (candidate.endsWith(`[${reference}][]`)) {
    return true;
  }
  const sanitizedCandidate = candidate.replace(/[^a-z0-9]/gi, "").toLowerCase();
  const sanitizedReference = reference.replace(/[^a-z0-9]/gi, "").toLowerCase();
  return sanitizedCandidate.endsWith(sanitizedReference);
}

function formatDynamicValue(value: string | string[] | null): string | undefined {
  if (value == null) {
    return undefined;
  }
  if (Array.isArray(value)) {
    if (value.length === 0) {
      return undefined;
    }
    return value.join(",");
  }
  return value === "" ? undefined : value;
}

function isAbortError(error: unknown): boolean {
  return (
    !!error &&
    typeof error === "object" &&
    ((error as { name?: string }).name === "AbortError" || error instanceof ResolverAbortError)
  );
}

export type ResolverEventName = "loading" | "success" | "error";

export interface ResolverEventDetail {
  element: HTMLElement;
  field: FieldConfig;
  endpoint: EndpointConfig;
  request: ResolverRequest;
  response: Response | null;
  options?: Option[];
  error?: ResolverError | Error;
  fromCache: boolean;
  durationMs?: number;
  attempt?: number;
}

export type ResolverEventDispatcher = (
  element: HTMLElement,
  name: ResolverEventName,
  detail: ResolverEventDetail
) => void;

export type Renderer = (context: RendererContext) => void | Promise<void>;

export interface CustomResolver {
  name: string;
  matches: (context: ResolverContext) => boolean;
  resolve: (context: ResolverContext) => Option[] | Promise<Option[] | undefined> | undefined;
}

export interface ResolverOptions {
  element: HTMLElement;
  field: FieldConfig;
  endpoint: EndpointConfig;
  config: ResolvedGlobalConfig;
  cacheAdapter?: CacheAdapter;
  cacheConfig: CacheConfig;
  dispatchEvent: ResolverEventDispatcher;
  renderers: Map<string, Renderer>;
  defaultRenderer: string;
  customResolvers: CustomResolver[];
}

export interface ResolverResult {
  options: Option[];
  fromCache: boolean;
}

export class Resolver {
  private readonly element: HTMLElement;
  private readonly field: FieldConfig;
  private readonly endpoint: EndpointConfig;
  private readonly config: ResolvedGlobalConfig;
  private readonly cacheAdapter?: CacheAdapter;
  private readonly cacheConfig: CacheConfig;
  private readonly dispatchEvent: ResolverEventDispatcher;
  private readonly renderers: Map<string, Renderer>;
  private readonly defaultRenderer: string;
  private readonly customResolvers: CustomResolver[];
  private abortController: AbortController | null = null;

  constructor(options: ResolverOptions) {
    this.element = options.element;
    this.field = options.field;
    this.endpoint = options.endpoint;
    this.config = options.config;
    this.cacheAdapter = options.cacheAdapter;
    this.cacheConfig = options.cacheConfig;
    this.dispatchEvent = options.dispatchEvent;
    this.renderers = options.renderers;
    this.defaultRenderer = options.defaultRenderer;
    this.customResolvers = options.customResolvers;

    if (this.element instanceof HTMLSelectElement) {
      if (this.field.submitAs === "json") {
        attachJsonInputSync(this.element);
      } else if (isMultiSelect(this.element)) {
        attachHiddenInputSync(this.element);
      }
    }
  }

  async resolve(): Promise<ResolverResult> {
    this.cancelInFlight();
    const startedAt = now();
    const request = await this.buildRequest();
    let fromCache = false;
    let options: Option[] | undefined;

    this.dispatchEvent(this.element, "loading", {
      element: this.element,
      field: this.field,
      endpoint: this.endpoint,
      request,
      response: null,
      fromCache: false,
    });

    if (request.cacheKey && this.cacheAdapter) {
      const cached = await this.cacheAdapter.get(request.cacheKey);
      if (Array.isArray(cached) && cached.length > 0) {
        options = cached;
        fromCache = true;
      }
    }

    if (!options) {
      const context = this.createContext(request, false);
      for (const custom of this.customResolvers) {
        try {
          if (custom.matches(context)) {
            const customOptions = await custom.resolve(context);
            if (Array.isArray(customOptions)) {
              options = customOptions;
              break;
            }
          }
        } catch (err) {
          this.handleError(err, request, null, startedAt, 0);
          throw err;
        }
      }
    }

    let fetchResult: FetchResult | null = null;

    try {
      if (!options) {
        fetchResult = await this.fetchOptions(request, startedAt);
        options = fetchResult.options;
        fromCache = fetchResult.fromCache;
      }
    } catch (error) {
      if (error instanceof ResolverAbortError || isAbortError(error)) {
        return { options: [], fromCache: false };
      }
      // Error already handled by handleError() which dispatched the error event
      // and set data-state="error". Don't rethrow to prevent unhandled rejections.
      return { options: [], fromCache: false };
    }

    const context = this.createContext(request, fromCache);

    if (this.config.transformOptions) {
      options = await this.config.transformOptions(context, options);
    }

    await this.renderOptions(options, fromCache);
    clearFieldError(this.element);

    const detail: ResolverEventDetail = {
      element: this.element,
      field: this.field,
      endpoint: this.endpoint,
      request,
      response: fetchResult?.response ?? null,
      options,
      fromCache,
      durationMs: now() - startedAt,
    };

    this.dispatchEvent(this.element, "success", detail);

    if (request.cacheKey && this.cacheAdapter && !fromCache) {
      await this.cacheAdapter.set(request.cacheKey, options, {
        ttlMs: this.cacheConfig.ttlMs,
        field: this.field,
        endpoint: this.endpoint,
      });
    }

    if (fetchResult) {
      await this.config.afterFetch?.(context, fetchResult);
    }

    if (this.element instanceof HTMLSelectElement) {
      if (this.element.getAttribute("data-relationship-submit-mode") === "json") {
        syncJsonInput(this.element);
      } else if (this.element.multiple) {
        syncHiddenInputs(this.element);
      }
    } else if (isMultiSelect(this.element)) {
      syncHiddenInputs(this.element);
    }

    this.abortController = null;

    return { options, fromCache };
  }

  private cancelInFlight(): void {
    if (this.abortController) {
      this.abortController.abort();
      this.abortController = null;
    }
  }

  private async fetchOptions(request: ResolverRequest, startedAt: number): Promise<FetchResult> {
    const context = this.createContext(request, false);
    const fetcher = globalThis.fetch?.bind(globalThis);
    if (!fetcher) {
      const error = new ResolverError("Fetch API unavailable in current environment");
      this.handleError(error, request, null, startedAt, 0);
      throw error;
    }

    const maxAttempts = Math.max(0, this.config.retryAttempts ?? 0) + 1;
    const delayMs = Math.max(0, this.config.retryDelayMs ?? 0);

    let attempt = 0;
    let lastError: unknown = null;

    while (attempt < maxAttempts) {
      if (context.request.init.signal?.aborted) {
        throw new ResolverAbortError();
      }

      await this.config.beforeFetch?.(context);
      try {
        const response = await fetcher(request.url, request.init);

        if (!response.ok) {
          const body = await safeJson(response);
          const error = new ResolverError(
            `Resolver request failed with status ${response.status}`,
            response.status,
            body
          );

          if (attempt < maxAttempts - 1 && shouldRetryStatus(response.status)) {
            attempt += 1;
            lastError = error;
            await delay(delayMs * attempt);
            continue;
          }

          this.handleError(error, request, response, startedAt, attempt);
          throw error;
        }

        const payload = await safeJson(response);
        const options = this.extractOptions(payload);

        this.config.logger?.info?.("resolver:success", {
          field: this.field.name,
          url: request.url,
          status: response.status,
          durationMs: now() - startedAt,
          attempt: attempt + 1,
        });

        return {
          response,
          payload,
          options,
          fromCache: false,
        };
      } catch (err) {
        if (isAbortError(err)) {
          throw new ResolverAbortError();
        }

        lastError = err;
        if (attempt < maxAttempts - 1) {
          attempt += 1;
          await delay(delayMs * attempt);
          continue;
        }

        // Convert error to ResolverError with proper message extraction
        const resolverError =
          err instanceof ResolverError
            ? err
            : new ResolverError(
                err instanceof Error ? err.message : String(err ?? "Unknown error"),
                undefined,
                err instanceof Error ? { name: err.name, stack: err.stack } : undefined
              );
        this.handleError(resolverError, request, null, startedAt, attempt);
        throw resolverError;
      }
    }

    if (lastError instanceof ResolverError) {
      throw lastError;
    }
    throw new ResolverError("Resolver exhausted retries", undefined, lastError);
  }

  private async buildRequest(): Promise<ResolverRequest> {
    const method = (this.endpoint.method || "GET").toUpperCase();
    const params = new URLSearchParams(this.endpoint.params ?? {});
    this.applyDynamicParams(params);

    if (this.field.mode === "search") {
      const searchValue = this.getSearchValue();
      if (this.field.searchParam) {
        if (searchValue && searchValue.length > 0) {
          params.set(this.field.searchParam, searchValue);
        } else {
          params.delete(this.field.searchParam);
        }
      }
    }

    if (!this.endpoint.resultsPath && !this.endpoint.mapping) {
      const hasFormatParam = Array.from(params.keys()).some(
        (key) => key.toLowerCase() === "format"
      );
      if (!hasFormatParam) {
        params.set("format", "options");
      }
    }

    const baseUrl = this.normaliseBaseUrl(this.endpoint.url ?? "");
    const query = params.toString();
    const url = query ? `${baseUrl}?${query}` : baseUrl;

    const headers: Record<string, string> = {
      Accept: "application/json",
    };

    Object.assign(headers, resolveAuthHeaders(this.endpoint.auth, this.element));

    this.abortController = new AbortController();

    const request: ResolverRequest = {
      url,
      cacheKey: this.computeCacheKey(method, url),
      init: {
        method,
        headers,
        signal: this.abortController.signal,
      },
    };

    const context = this.createContext(request, false);

    if (this.config.buildHeaders) {
      const custom = await this.config.buildHeaders(context);
      if (custom && typeof custom === "object") {
        Object.assign(request.init.headers as Record<string, string>, custom);
      }
    }

    return request;
  }

  private applyDynamicParams(params: URLSearchParams): void {
    const dynamic = this.endpoint.dynamicParams ?? {};
    Object.entries(dynamic).forEach(([key, template]) => {
      const value = this.resolveDynamicTemplate(template);
      if (value == null || value === "") {
        params.delete(key);
      } else {
        params.set(key, value);
      }
    });
  }

  private resolveDynamicTemplate(template: string): string | undefined {
    if (!template) {
      return undefined;
    }

    let replaced = false;
    const result = template.replace(DYNAMIC_TOKEN_PATTERN, (_, raw: string) => {
      replaced = true;
      const token = raw.trim();
      if (!token) {
        return "";
      }
      if (token.startsWith("field:")) {
        const name = token.slice("field:".length);
        return this.getFieldReferenceValue(name) ?? "";
      }
      if (token === "self" || token === "value") {
        return this.getSelfValue() ?? "";
      }
      return "";
    });

    if (!replaced) {
      return template;
    }

    const trimmed = result.trim();
    return trimmed.length > 0 ? trimmed : undefined;
  }

  private getSelfValue(): string | undefined {
    if (this.field.mode === "search") {
      const searchValue = this.getSearchValue();
      if (searchValue !== undefined) {
        return searchValue === "" ? undefined : searchValue;
      }
    }
    const value = readElementValue(this.element);
    return formatDynamicValue(value);
  }

  private getSearchValue(): string | undefined {
    if (!(this.element instanceof HTMLElement)) {
      return undefined;
    }
    const attr = this.element.getAttribute("data-endpoint-search-value");
    if (attr != null) {
      return attr;
    }
    const datasetValue = (this.element.dataset as Record<string, string | undefined>).endpointSearchValue;
    return datasetValue ?? undefined;
  }

  private getFieldReferenceValue(reference: string): string | undefined {
    const elements = this.findDependencyElements(reference);
    for (const element of elements) {
      const raw = readElementValue(element);
      const formatted = formatDynamicValue(raw);
      if (formatted !== undefined) {
        return formatted;
      }
    }
    return undefined;
  }

  private findDependencyElements(reference: string): HTMLElement[] {
    const matches: HTMLElement[] = [];
    const seen = new Set<HTMLElement>();

    const form = this.element.closest("form");
    const scope: HTMLElement[] = [];

    if (form) {
      const elements = Array.from(form.elements) as Array<Element>;
      for (const el of elements) {
        if (el instanceof HTMLElement) {
          scope.push(el);
        }
      }
    } else {
      const doc = this.element.ownerDocument ?? document;
      scope.push(...Array.from(doc.querySelectorAll<HTMLElement>("[name], [data-field-name], [id]")));
    }

    for (const candidate of scope) {
      if (seen.has(candidate)) {
        continue;
      }

      const datasetName = (candidate.dataset as Record<string, string | undefined>).fieldName;
      if (datasetName && datasetName === reference) {
        matches.push(candidate);
        seen.add(candidate);
        continue;
      }

      const nameAttr = (candidate as HTMLInputElement).name;
      if (matchesFieldName(nameAttr, reference)) {
        matches.push(candidate);
        seen.add(candidate);
        continue;
      }

      const idAttr = candidate.id;
      if (idAttr && idAttr === reference) {
        matches.push(candidate);
        seen.add(candidate);
      }
    }

    return matches;
  }

  private normaliseBaseUrl(url: string): string {
    if (/^https?:/i.test(url)) {
      return url;
    }
    const base = this.config.baseUrl?.replace(/\/$/, "") ?? "";
    const relative = url.startsWith("/") ? url : `/${url}`;
    return `${base}${relative}` || url;
  }

  private computeCacheKey(method: string, url: string): string | undefined {
    if (typeof this.field.cacheKey === "string" && this.field.cacheKey.length > 0) {
      return this.field.cacheKey;
    }

    if (this.config.cache?.keyFactory) {
      return this.config.cache.keyFactory(
        this.createContext(
          {
            url,
            init: { method },
          },
          false
        )
      );
    }

    return `${method.toUpperCase()}::${url}`;
  }

  private createContext(request: ResolverRequest, fromCache: boolean): ResolverContext {
    return {
      element: this.element,
      field: this.field,
      endpoint: this.endpoint,
      request,
      fromCache,
      config: this.config,
    };
  }

  private resolveRenderer(): Renderer {
    const rendererName = this.field.renderer ?? this.defaultRenderer;
    const renderer = this.renderers.get(rendererName) ?? this.renderers.get(this.defaultRenderer);
    if (!renderer) {
      throw new ResolverError(`Renderer '${rendererName}' is not registered`);
    }
    return renderer;
  }

  private async renderOptions(options: Option[], fromCache: boolean): Promise<void> {
    const renderer = this.resolveRenderer();
    const context: RendererContext = {
      element: this.element,
      field: this.field,
      options,
      fromCache,
      config: this.config,
    };
    await renderer(context);
  }

  private extractOptions(payload: unknown): Option[] {
    const array = this.unwrapPayload(payload);
    return array.map((item) => this.mapOption(item));
  }

  private unwrapPayload(payload: unknown): any[] {
    if (!this.endpoint.resultsPath) {
      if (Array.isArray(payload)) {
        return payload;
      }
      if (payload && typeof payload === "object" && "data" in (payload as any)) {
        const data = (payload as any).data;
        if (Array.isArray(data)) {
          return data;
        }
      }
      throw new ResolverError("Resolver response payload is not an array");
    }

    const parts = this.endpoint.resultsPath.split(".").filter(Boolean);
    let current: any = payload;
    for (const part of parts) {
      if (current == null) {
        break;
      }
      current = current[part];
    }
    if (!Array.isArray(current)) {
      throw new ResolverError("Resolver resultsPath did not resolve to an array");
    }
    return current;
  }

  private mapOption(item: any): Option {
    if (item == null) {
      return { value: "", label: "" };
    }
    if (typeof item !== "object") {
      const stringified = String(item);
      return { value: stringified, label: stringified, raw: item };
    }

    const valuePath = this.endpoint.mapping?.value ?? this.endpoint.valueField ?? "value";
    const labelPath = this.endpoint.mapping?.label ?? this.endpoint.labelField ?? "label";
    const metaPath = this.endpoint.mapping?.meta;

    const record = typeof item === "object" && item !== null ? (item as Record<string, unknown>) : null;
    let value = getByPath(item, valuePath);
    let label = getByPath(item, labelPath);
    const meta = metaPath ? getByPath(item, metaPath) : undefined;

    if ((value === undefined || value === null) && record) {
      if (record["value"] != null) {
        value = record["value"];
      } else if (record["id"] != null) {
        value = record["id"];
      }
    }

    if ((label === undefined || label === null) && record) {
      if (record["label"] != null) {
        label = record["label"];
      } else if (record["name"] != null) {
        label = record["name"];
      }
    }

    const stringValue = value == null ? "" : String(value);
    const stringLabel = label == null ? stringValue : String(label);

    return {
      value: stringValue,
      label: stringLabel,
      meta,
      raw: item,
    };
  }

  private handleError(
    error: unknown,
    request: ResolverRequest,
    response: Response | null,
    startedAt: number,
    attempt: number
  ): void {
    const resolverError =
      error instanceof ResolverError ? error : new ResolverError(String(error || ""));

    setFieldError(this.element, resolverError.message || DEFAULT_ERROR_MESSAGE);

    const detail: ResolverEventDetail = {
      element: this.element,
      field: this.field,
      endpoint: this.endpoint,
      request,
      response,
      error: resolverError,
      fromCache: false,
      durationMs: now() - startedAt,
      attempt: attempt + 1,
    };

    this.config.logger?.error?.("resolver:error", {
      field: this.field.name,
      url: request.url,
      status: resolverError.status,
      durationMs: detail.durationMs,
      attempt: detail.attempt,
      message: resolverError.message,
    });

    this.config.onError?.(this.createContext(request, false), resolverError);
    this.dispatchEvent(this.element, "error", detail);
  }
}

async function safeJson(response: Response): Promise<unknown> {
  const text = await response.text();
  if (!text) {
    return null;
  }
  try {
    return JSON.parse(text);
  } catch (error) {
    throw new ResolverError("Resolver received invalid JSON", response.status, {
      error,
      body: text,
    });
  }
}

function getByPath(source: any, path: string | undefined): unknown {
  if (!path) {
    return undefined;
  }
  const segments = path.split(".").filter(Boolean);
  let current = source;
  for (const segment of segments) {
    if (current == null) {
      return undefined;
    }
    current = current[segment];
  }
  return current;
}

function shouldRetryStatus(status?: number): boolean {
  if (typeof status !== "number") {
    return false;
  }
  return status >= 500;
}
