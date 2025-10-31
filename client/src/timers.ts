export function createThrottledInvoker<T extends (...args: any[]) => void>(
  callback: T,
  interval: number
): T {
  if (!interval || interval <= 0) {
    return callback as T;
  }

  let lastCall = 0;
  let trailingTimer: number | undefined;
  let trailingArgs: Parameters<T> | undefined;

  const invoke = (...args: Parameters<T>) => {
    lastCall = Date.now();
    callback(...args);
  };

  const throttled = (...args: Parameters<T>) => {
    const now = Date.now();
    const remaining = interval - (now - lastCall);

    if (remaining <= 0) {
      if (trailingTimer !== undefined) {
        clearTimeout(trailingTimer);
        trailingTimer = undefined;
      }
      invoke(...args);
      return;
    }

    trailingArgs = args;
    if (trailingTimer === undefined) {
      trailingTimer = setTimeout(() => {
        trailingTimer = undefined;
        if (trailingArgs) {
          invoke(...trailingArgs);
          trailingArgs = undefined;
        }
      }, remaining) as unknown as number;
    }
  };

  return throttled as T;
}

export function createDebouncedInvoker<T extends (...args: any[]) => void>(
  callback: T,
  interval: number
): T {
  if (!interval || interval <= 0) {
    return callback as T;
  }

  let timer: number | undefined;
  const debounced = (...args: Parameters<T>) => {
    if (timer !== undefined) {
      clearTimeout(timer);
    }
    timer = setTimeout(() => {
      timer = undefined;
      callback(...args);
    }, interval) as unknown as number;
  };

  return debounced as T;
}
