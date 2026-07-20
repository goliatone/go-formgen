export type CancelableInvoker<T extends (...args: any[]) => void> = T & {
  cancel: () => void;
};

function cancelNestedInvoker(callback: (...args: any[]) => void): void {
  const cancel = (callback as { cancel?: () => void }).cancel;
  if (typeof cancel === "function") {
    cancel();
  }
}

export function createThrottledInvoker<T extends (...args: any[]) => void>(
  callback: T,
  interval: number
): CancelableInvoker<T> {
  if (!interval || interval <= 0) {
    const immediate = ((...args: Parameters<T>) => callback(...args)) as CancelableInvoker<T>;
    immediate.cancel = () => cancelNestedInvoker(callback);
    return immediate;
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

  const cancelable = throttled as CancelableInvoker<T>;
  cancelable.cancel = () => {
    if (trailingTimer !== undefined) {
      clearTimeout(trailingTimer);
      trailingTimer = undefined;
    }
    trailingArgs = undefined;
    cancelNestedInvoker(callback);
  };
  return cancelable;
}

export function createDebouncedInvoker<T extends (...args: any[]) => void>(
  callback: T,
  interval: number
): CancelableInvoker<T> {
  if (!interval || interval <= 0) {
    const immediate = ((...args: Parameters<T>) => callback(...args)) as CancelableInvoker<T>;
    immediate.cancel = () => cancelNestedInvoker(callback);
    return immediate;
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

  const cancelable = debounced as CancelableInvoker<T>;
  cancelable.cancel = () => {
    if (timer !== undefined) {
      clearTimeout(timer);
      timer = undefined;
    }
    cancelNestedInvoker(callback);
  };
  return cancelable;
}
