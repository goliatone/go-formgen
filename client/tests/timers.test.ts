import { afterEach, describe, expect, it, vi } from "vitest";
import { createDebouncedInvoker, createThrottledInvoker } from "../src/timers";

describe("cancelable runtime timers", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it("cancels pending debounced work", () => {
    vi.useFakeTimers();
    const callback = vi.fn();
    const invoke = createDebouncedInvoker(callback, 25);

    invoke("stale");
    invoke.cancel();
    vi.advanceTimersByTime(30);

    expect(callback).not.toHaveBeenCalled();
  });

  it("cancels nested debounce work through a throttled invoker", () => {
    vi.useFakeTimers();
    const callback = vi.fn();
    const debounced = createDebouncedInvoker(callback, 25);
    const invoke = createThrottledInvoker(debounced, 10);

    invoke("stale");
    invoke.cancel();
    vi.advanceTimersByTime(30);

    expect(callback).not.toHaveBeenCalled();
  });
});
