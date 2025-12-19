import type { ResolverError } from "./errors";

export type RelationshipKind = "belongsTo" | "hasOne" | "hasMany";
export type RelationshipCardinality = "one" | "many";

/**
 * Option describes the canonical `{value,label}` tuple shared across renderers.
 */
export interface Option {
  value: string;
  label: string;
  /** Icon name for registry lookup (not raw HTML for security). */
  icon?: string;
  /** URL to avatar image for display in options and chips. */
  avatar?: string;
  /** Subtitle/description text displayed below the label. */
  description?: string;
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
  /**
   * When true, search-based relationship widgets may offer an **inline create**
   * action for the current query. The inline create flow is triggered when the
   * user's search query doesn't match any existing options, allowing lightweight
   * creation (e.g., tags) directly within the dropdown.
   *
   * This is intentionally opt-in per field (usually via `data-endpoint-allow-create="true"`).
   *
   * **Note**: This is distinct from `createAction`, which provides a dedicated
   * "Create …" button that delegates to a host-defined UI (modal/panel/redirect)
   * for more complex creation flows. Both features may be enabled simultaneously.
   *
   * @see createAction for delegated create flows (modal/panel)
   */
  allowCreate?: boolean;
  /**
   * When true, a dedicated "Create …" action is rendered in the dropdown
   * (footer for chips, row for typeahead). Unlike `allowCreate` (inline create),
   * this action delegates creation to a host-defined UI (modal/panel/redirect)
   * and is always visible regardless of search query or existing matches.
   *
   * The action triggers either:
   * - `GlobalConfig.onCreateAction` hook (if provided), or
   * - `formgen:relationship:create-action` DOM event (if hook not provided)
   *
   * This is intentionally opt-in per field (via `data-endpoint-create-action="true"`).
   *
   * @see allowCreate for inline (query-based) creation
   * @see GlobalConfig.onCreateAction for programmatic handling
   */
  createAction?: boolean;
  /**
   * Custom label for the create action button. If omitted, a default label
   * is derived from the field label (e.g., "Create Author…" or "Create new…").
   *
   * Set via `data-endpoint-create-action-label="Create Author"`.
   */
  createActionLabel?: string;
  /**
   * Optional identifier the host can use to route to the correct modal/flow
   * when the create action is triggered. Passed through in the event payload
   * and hook detail.
   *
   * Set via `data-endpoint-create-action-id="author"`.
   */
  createActionId?: string;
  /**
   * How returned options from the create action are applied to the selection.
   * - `"append"` (default for multi-select): adds to existing selection
   * - `"replace"`: clears existing selection before applying new options
   *
   * Set via `data-endpoint-create-action-select="append|replace"`.
   */
  createActionSelect?: "append" | "replace";
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

/**
 * Detail payload passed to the `onCreateAction` hook when the user activates
 * a create action in a relationship widget (typeahead or chips).
 */
export interface CreateActionDetail {
  /** Current search query (may be empty in default mode). */
  query: string;
  /** Optional identifier for routing to the correct modal/flow. */
  actionId?: string;
  /** Which renderer triggered the action. */
  mode: "typeahead" | "chips";
  /** How returned options should be applied to selection. */
  selectBehavior: "append" | "replace";
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
  /**
   * Optional hook for **inline creation** of a new option record (e.g. POST /tags)
   * when a relationship field allows user-defined values via `allowCreate`.
   *
   * When provided, widgets may call this via `ResolverRegistry.create(...)`.
   * Returning an `Option` is required so the widget can select it immediately.
   *
   * **Note**: This is distinct from `onCreateAction`, which handles delegated
   * creation flows (modal/panel). Both hooks may be provided simultaneously.
   *
   * @see onCreateAction for delegated create flows (modal/panel)
   */
  createOption?: (context: ResolverContext, query: string) => Option | Promise<Option>;
  /**
   * Optional hook for **delegated creation** triggered by the "Create …" action
   * in relationship widgets. Unlike `createOption` (inline creation), this hook
   * delegates to a host-defined UI (modal/panel/redirect).
   *
   * When provided, this hook takes precedence over the DOM event
   * (`formgen:relationship:create-action`). If not provided, the event is
   * dispatched instead.
   *
   * **Return values:**
   * - `Option`: Single created option (typeahead always, chips single create)
   * - `Option[]`: Multiple created options (chips only, batch creation)
   * - `void`: Host will apply selection manually via registry/DOM
   * - `Promise<...>`: Async versions of the above
   *
   * Returned options are applied according to `detail.selectBehavior`:
   * - `"replace"` (default for typeahead): replaces existing selection
   * - `"append"` (default for chips): adds to existing selection
   *
   * @see createOption for inline (query-based) creation
   * @see CreateActionDetail for the detail payload structure
   */
  onCreateAction?: (
    context: ResolverContext,
    detail: CreateActionDetail
  ) => Option | Option[] | void | Promise<Option | Option[] | void>;
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
