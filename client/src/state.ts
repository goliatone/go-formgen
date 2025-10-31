interface CacheEntry<TValue> {
  value: TValue;
  expiresAt?: number;
}

/**
 * MemoryCache implements a lightweight TTL-aware cache suitable for the
 * resolver runtime. Eviction happens lazily on access to keep the
 * implementation simple while covering the documented requirements from
 * JS_TDD.md §7.1.
 */
export class MemoryCache<TKey, TValue> {
  private readonly store = new Map<TKey, CacheEntry<TValue>>();

  get(key: TKey): TValue | undefined {
    const entry = this.store.get(key);
    if (!entry) {
      return undefined;
    }
    if (entry.expiresAt && entry.expiresAt <= Date.now()) {
      this.store.delete(key);
      return undefined;
    }
    return entry.value;
  }

  set(key: TKey, value: TValue, ttlMs?: number): void {
    const expiresAt = typeof ttlMs === "number" && ttlMs > 0 ? Date.now() + ttlMs : undefined;
    this.store.set(key, { value, expiresAt });
  }

  delete(key: TKey): void {
    this.store.delete(key);
  }

  clear(): void {
    this.store.clear();
  }
}
