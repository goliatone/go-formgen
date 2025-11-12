import type { ResolverError } from "./errors";

export type RelationshipKind = "belongsTo" | "hasOne" | "hasMany";
export type RelationshipCardinality = "one" | "many";

/**
 * Option describes the canonical `{value,label}` tuple shared across renderers.
 */
export interface Option {
  value: string;
  label: string;
  meta?: unknown;
  /** Raw value prior to mapping, useful for custom renderers */
  raw?: unknown;
}

/**
 * EndpointMapping remaps value/label paths from API responses.
 */
export interface EndpointMapping {
  value?: string;
  label?: string;
  meta?: string;
}

export type AuthStrategy = "header" | "cookie" | "custom" | undefined;

/**
 * EndpointAuth describes how runtime resolvers should supply authentication
 * tokens when fetching relationship options.
 */
export interface EndpointAuth {
  strategy?: AuthStrategy;
  header?: string;
  source?: string;
  prefix?: string;
}

/**
 * EndpointConfig mirrors the x-endpoint extension contract documented in
 * JS_TDD.md §5.1. Optional keys are omitted when not provided.
 */
export interface EndpointConfig {
  url?: string;
  method?: string;
  labelField?: string;
  valueField?: string;
  resultsPath?: string;
  params?: Record<string, string>;
  dynamicParams?: Record<string, string>;
  mapping?: EndpointMapping;
  auth?: EndpointAuth;
  mode?: string;
  searchParam?: string;
  submitAs?: string;
}

/**
 * EndpointOverride allows callers to inject endpoint metadata when an OpenAPI
 * schema lacks x-endpoint annotations for a relationship field.
 */
export interface EndpointOverride {
  operationId: string;
  fieldPath: string;
  endpoint: EndpointConfig;
}

export interface FieldValidationRule {
  kind: string;
  params?: Record<string, string>;
}

export interface ValidationError {
  code: string;
  message: string;
  rule?: FieldValidationRule;
  value?: string | string[] | null;
}

export interface ValidationResult {
  valid: boolean;
  messages: string[];
  errors: ValidationError[];
}

/**
 * FieldConfig aggregates the relationship metadata harvested from the DOM.
 */
export interface FieldConfig {
  name?: string;
  label?: string;
  relationship?: RelationshipKind;
  cardinality?: RelationshipCardinality;
  current?: string | string[] | null;
  refreshOn?: string[];
  refreshMode?: "auto" | "manual";
  mode?: "default" | "search";
  throttleMs?: number;
  debounceMs?: number;
  searchParam?: string;
  renderer?: string;
  cacheKey?: string;
  submitAs?: "default" | "json";
  icon?: string;
  iconSource?: string;
  iconRaw?: string;
  required?: boolean;
  validations?: FieldValidationRule[];
}

export interface ResolverRequest {
  url: string;
  init: RequestInit;
  cacheKey?: string;
}

export interface Logger {
  debug?: (...args: unknown[]) => void;
  info?: (...args: unknown[]) => void;
  warn?: (...args: unknown[]) => void;
  error?: (...args: unknown[]) => void;
}

export interface ResolverContext {
  element: HTMLElement;
  field: FieldConfig;
  endpoint: EndpointConfig;
  request: ResolverRequest;
  fromCache: boolean;
  config: ResolvedGlobalConfig;
}

export interface FetchResult {
  response: Response | null;
  payload: unknown;
  options: Option[];
  fromCache: boolean;
}

export interface RendererContext {
  element: HTMLElement;
  field: FieldConfig;
  options: Option[];
  fromCache: boolean;
  config: ResolvedGlobalConfig;
}

export interface CacheSetContext {
  ttlMs?: number;
  field: FieldConfig;
  endpoint: EndpointConfig;
}

export interface CacheAdapter {
  get(key: string): Option[] | Promise<Option[] | undefined> | undefined;
  set(key: string, value: Option[], context: CacheSetContext): void | Promise<void>;
  delete?(key: string): void | Promise<void>;
  clear?(): void | Promise<void>;
}

export interface CacheConfig {
  strategy?: "none" | "memory" | "custom";
  ttlMs?: number;
  adapter?: CacheAdapter;
  keyFactory?: (context: ResolverContext) => string | undefined;
}

export interface ResolvedCacheConfig extends CacheConfig {
  strategy: "none" | "memory" | "custom";
  ttlMs: number;
  adapter?: CacheAdapter;
}

/**
 * Global configuration shared across resolver instances.
 */
export interface GlobalConfig {
  baseUrl?: string;
  buildHeaders?: (
    context: ResolverContext
  ) => Record<string, string> | Promise<Record<string, string>>;
  beforeFetch?: (context: ResolverContext) => void | Promise<void>;
  afterFetch?: (context: ResolverContext, result: FetchResult) => void | Promise<void>;
  transformOptions?: (
    context: ResolverContext,
    options: Option[]
  ) => Option[] | Promise<Option[]>;
  renderOption?: (context: RendererContext, option: Option) => HTMLOptionElement;
  onError?: (context: ResolverContext, error: ResolverError) => void;
  validateSelection?: (
    context: ResolverContext,
    value: string | string[] | null
  ) => ValidationResult | Promise<ValidationResult>;
  onValidationError?: (context: ResolverContext, error: ValidationError) => void;
  cache?: CacheConfig;
  logger?: Logger;
  searchThrottleMs?: number;
  searchDebounceMs?: number;
  retryAttempts?: number;
  retryDelayMs?: number;
}

export interface ResolvedGlobalConfig extends GlobalConfig {
  baseUrl: string;
  cache: ResolvedCacheConfig;
  logger?: Logger;
  searchThrottleMs: number;
  searchDebounceMs: number;
  retryAttempts: number;
  retryDelayMs: number;
}

const DEFAULT_CACHE: ResolvedCacheConfig = {
  strategy: "memory",
  ttlMs: 5 * 60 * 1000,
};

const DEFAULT_CONFIG: ResolvedGlobalConfig = {
  baseUrl: "",
  cache: DEFAULT_CACHE,
  logger: undefined,
  searchThrottleMs: 250,
  searchDebounceMs: 250,
  retryAttempts: 1,
  retryDelayMs: 300,
};

let activeConfig: ResolvedGlobalConfig = DEFAULT_CONFIG;

export function resolveGlobalConfig(config: GlobalConfig = {}): ResolvedGlobalConfig {
  const merged: ResolvedGlobalConfig = {
    ...DEFAULT_CONFIG,
    ...config,
    cache: {
      ...DEFAULT_CACHE,
      ...(config.cache ?? {}),
    },
  };

  if (!merged.cache.strategy) {
    merged.cache.strategy = "memory";
  }
  if (typeof merged.cache.ttlMs !== "number" || Number.isNaN(merged.cache.ttlMs)) {
    merged.cache.ttlMs = DEFAULT_CACHE.ttlMs;
  }

  if (typeof merged.searchThrottleMs !== "number" || merged.searchThrottleMs < 0) {
    merged.searchThrottleMs = DEFAULT_CONFIG.searchThrottleMs;
  }
  if (typeof merged.searchDebounceMs !== "number" || merged.searchDebounceMs < 0) {
    merged.searchDebounceMs = DEFAULT_CONFIG.searchDebounceMs;
  }
  if (typeof merged.retryAttempts !== "number" || merged.retryAttempts < 0) {
    merged.retryAttempts = DEFAULT_CONFIG.retryAttempts;
  }
  if (typeof merged.retryDelayMs !== "number" || merged.retryDelayMs < 0) {
    merged.retryDelayMs = DEFAULT_CONFIG.retryDelayMs;
  }

  return merged;
}

export function setGlobalConfig(config: GlobalConfig = {}): ResolvedGlobalConfig {
  activeConfig = resolveGlobalConfig(config);
  return activeConfig;
}

export function getGlobalConfig(): ResolvedGlobalConfig {
  return activeConfig;
}

export const defaultConfig: ResolvedGlobalConfig = DEFAULT_CONFIG;
