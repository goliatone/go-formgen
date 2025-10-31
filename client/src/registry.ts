import {
  type CacheAdapter,
  type CacheConfig,
  type EndpointConfig,
  type FieldConfig,
  type GlobalConfig,
  type Option,
  type RendererContext,
  type ResolvedGlobalConfig,
  getGlobalConfig,
  resolveGlobalConfig,
  setGlobalConfig,
} from "./config";
import { MemoryCache } from "./state";
import {
  Resolver,
  type ResolverEventDetail,
  type ResolverEventName,
  type ResolverOptions,
  type Renderer,
  type CustomResolver,
} from "./resolver";

const DEFAULT_RENDERER_KEY = "default";

class MemoryCacheAdapter implements CacheAdapter {
  private readonly cache = new MemoryCache<string, Option[]>();

  get(key: string): Option[] | undefined {
    return this.cache.get(key);
  }

  set(key: string, value: Option[], context: { ttlMs?: number }): void {
    this.cache.set(key, value, context.ttlMs);
  }

  delete(key: string): void {
    this.cache.delete(key);
  }

  clear(): void {
    this.cache.clear();
  }
}

function defaultRenderer(context: RendererContext): void {
  const { element, options, field } = context;

  if (element instanceof HTMLSelectElement) {
    const previousSelection = new Set(Array.from(element.selectedOptions).map((opt) => opt.value));
    const finalSelection = computeTargetSelection(field, previousSelection);
    element.innerHTML = "";
    options.forEach((option) => {
      const node = document.createElement("option");
      node.value = option.value;
      node.textContent = option.label;
      if (finalSelection.has(option.value)) {
        node.selected = true;
      }
      element.appendChild(node);
    });
    return;
  }

  if (element instanceof HTMLDataListElement) {
    element.innerHTML = "";
    options.forEach((option) => {
      const node = document.createElement("option");
      node.value = option.value;
      node.label = option.label;
      element.appendChild(node);
    });
    return;
  }

  element.textContent = options.map((option) => option.label).join(", ");
}

function computeTargetSelection(field: FieldConfig, previous: Set<string>): Set<string> {
  if (field.current == null) {
    return previous;
  }
  const selection = new Set<string>();
  if (Array.isArray(field.current)) {
    field.current.forEach((value) => selection.add(String(value)));
  } else {
    selection.add(String(field.current));
  }
  return selection;
}

function mutateDataState(element: HTMLElement, state: string): void {
  element.setAttribute("data-state", state);
}

export interface RegistrationOptions {
  field: FieldConfig;
  endpoint: EndpointConfig;
}

export class ResolverRegistry {
  private readonly resolvers = new Map<HTMLElement, Resolver>();
  private readonly renderers = new Map<string, Renderer>();
  private readonly customResolvers: CustomResolver[] = [];
  private readonly config: ResolvedGlobalConfig;
  private readonly cacheAdapter?: CacheAdapter;
  private readonly cacheConfig: CacheConfig;

  constructor(config?: GlobalConfig) {
    this.config = config ? resolveGlobalConfig(config) : getGlobalConfig();
    if (config) {
      setGlobalConfig(this.config);
    }
    this.cacheConfig = this.config.cache ?? {};
    this.cacheAdapter = this.resolveCacheAdapter(this.cacheConfig);
    this.registerRenderer(DEFAULT_RENDERER_KEY, defaultRenderer);
  }

  private resolveCacheAdapter(cacheConfig: CacheConfig): CacheAdapter | undefined {
    if (cacheConfig.strategy === "none") {
      return undefined;
    }
    if (cacheConfig.strategy === "custom") {
      return cacheConfig.adapter;
    }
    return new MemoryCacheAdapter();
  }

  getConfig(): ResolvedGlobalConfig {
    return this.config;
  }

  register(
    element: HTMLElement,
    options: RegistrationOptions
  ): Resolver;
  register(name: string, resolver: CustomResolver): void;
  register(
    arg1: HTMLElement | string,
    arg2: RegistrationOptions | CustomResolver
  ): Resolver | void {
    if (arg1 instanceof HTMLElement) {
      return this.attachField(arg1, arg2 as RegistrationOptions);
    }
    this.registerResolver(arg1, arg2 as CustomResolver);
  }

  private attachField(
    element: HTMLElement,
    { field, endpoint }: RegistrationOptions
  ): Resolver {
    const resolver = new Resolver({
      element,
      field,
      endpoint,
      config: this.config,
      cacheAdapter: this.cacheAdapter,
      cacheConfig: this.cacheConfig,
      dispatchEvent: this.dispatchEvent,
      renderers: this.renderers,
      defaultRenderer: DEFAULT_RENDERER_KEY,
      customResolvers: this.customResolvers,
    });

    this.resolvers.set(element, resolver);
    return resolver;
  }

  get(element: HTMLElement): Resolver | undefined {
    return this.resolvers.get(element);
  }

  async resolve(element: HTMLElement): Promise<void> {
    const resolver = this.resolvers.get(element);
    if (!resolver) {
      return;
    }
    await resolver.resolve();
  }

  registerRenderer(name: string, renderer: Renderer): void {
    this.renderers.set(name, renderer);
  }

  registerResolver(name: string, resolver: CustomResolver): void {
    this.customResolvers.push({ ...resolver, name });
  }

  private dispatchEvent = (element: HTMLElement, name: ResolverEventName, detail: ResolverEventDetail) => {
    if (name === "loading") {
      mutateDataState(element, "loading");
    } else if (name === "success") {
      mutateDataState(element, "ready");
    } else if (name === "error") {
      mutateDataState(element, "error");
    }

    try {
      const event = new CustomEvent(`formgen:relationship:${name}`, {
        bubbles: true,
        detail,
      });
      element.dispatchEvent(event);
    } catch (err) {
      if (this.config.logger?.warn) {
        this.config.logger.warn("Failed to dispatch resolver event", err);
      }
    }
  };
}
