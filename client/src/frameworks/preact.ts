import { useEffect, useState } from "preact/hooks";
import type { Option } from "../config";
import type { ResolverRegistry } from "../registry";
import type { ResolverEventDetail } from "../resolver";
import { initRelationships } from "../index";

interface HookState {
  options: Option[];
  loading: boolean;
  error: Error | null;
}

const SUCCESS_EVENT = "formgen:relationship:success";
const ERROR_EVENT = "formgen:relationship:error";
const LOADING_EVENT = "formgen:relationship:loading";

/**
 * useRelationshipOptions integrates the resolver registry with Preact
 * components, mapping lifecycle events to component state.
 */
export function useRelationshipOptions(element: HTMLElement | null) {
  const [registry, setRegistry] = useState<ResolverRegistry | null>(null);
  const [state, setState] = useState<HookState>({
    options: [],
    loading: false,
    error: null,
  });

  useEffect(() => {
    let cancelled = false;
    initRelationships()
      .then((instance) => {
        if (!cancelled) {
          setRegistry(instance);
        }
      })
      .catch((error) => {
        if (!cancelled) {
          setState((prev) => ({ ...prev, error: error as Error, loading: false }));
        }
      });

    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    if (!element || !registry) {
      return;
    }

    const handleLoading = (event: Event) => {
      const detail = (event as CustomEvent<ResolverEventDetail>).detail;
      if (detail.element !== element) {
        return;
      }
      setState((prev) => ({ ...prev, loading: true, error: null }));
    };

    const handleSuccess = (event: Event) => {
      const detail = (event as CustomEvent<ResolverEventDetail>).detail;
      if (detail.element !== element) {
        return;
      }
      setState({ options: detail.options ?? [], loading: false, error: null });
    };

    const handleError = (event: Event) => {
      const detail = (event as CustomEvent<ResolverEventDetail>).detail;
      if (detail.element !== element) {
        return;
      }
      setState((prev) => ({ ...prev, loading: false, error: detail.error ?? null }));
    };

    element.addEventListener(LOADING_EVENT, handleLoading as EventListener);
    element.addEventListener(SUCCESS_EVENT, handleSuccess as EventListener);
    element.addEventListener(ERROR_EVENT, handleError as EventListener);

    registry.resolve(element).catch((error) => {
      setState((prev) => ({ ...prev, error: error as Error, loading: false }));
    });

    return () => {
      element.removeEventListener(LOADING_EVENT, handleLoading as EventListener);
      element.removeEventListener(SUCCESS_EVENT, handleSuccess as EventListener);
      element.removeEventListener(ERROR_EVENT, handleError as EventListener);
    };
  }, [element, registry]);

  return {
    options: state.options,
    loading: state.loading,
    error: state.error,
    refresh: async () => {
      if (!element || !registry) {
        return;
      }
      setState((prev) => ({ ...prev, loading: true }));
      try {
        await registry.resolve(element);
      } finally {
        setState((prev) => ({ ...prev, loading: false }));
      }
    },
  };
}
