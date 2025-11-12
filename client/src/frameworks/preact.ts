import { useEffect, useState } from "preact/hooks";
import type { Option, ValidationResult } from "../config";
import type { ResolverRegistry } from "../registry";
import type { ResolverEventDetail } from "../resolver";
import { initRelationships } from "../index";

interface HookState {
  options: Option[];
  loading: boolean;
  error: Error | null;
  validation: ValidationResult | null;
}

const SUCCESS_EVENT = "formgen:relationship:success";
const ERROR_EVENT = "formgen:relationship:error";
const LOADING_EVENT = "formgen:relationship:loading";
const VALIDATION_EVENT = "formgen:relationship:validation";

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
    validation: null,
  });

  useEffect(() => {
    if (!element) return;

    let cancelled = false;
    let currentRegistry: ResolverRegistry | null = null;

    const setupElement = async () => {
      const reg = await initRelationships();
      if (cancelled) return;

      currentRegistry = reg;
      setRegistry(reg);

      // Check if element is already resolved
      const dataState = element.getAttribute("data-state");
      if (dataState === "ready" && element instanceof HTMLSelectElement) {
        const options: Option[] = Array.from(element.options)
          .filter((opt) => opt.value !== "")
          .map((opt) => ({
            value: opt.value,
            label: opt.textContent || opt.value,
          }));
        setState({ options, loading: false, error: null });
      } else if (dataState === "error") {
        setState((prev) => ({ ...prev, loading: false, error: new Error("Failed to load options") }));
      } else if (dataState !== "loading") {
        setState((prev) => ({ ...prev, loading: true }));
        try {
          await reg.resolve(element);
        } catch (error) {
          if (!cancelled) {
            setState((prev) => ({ ...prev, error: error as Error, loading: false }));
          }
        }
      }
    };

    setupElement().catch((error) => {
      if (!cancelled) {
        setState((prev) => ({ ...prev, error: error as Error, loading: false }));
      }
    });

    return () => {
      cancelled = true;
    };
  }, [element]);

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
      setState((prev) => ({
        ...prev,
        options: detail.options ?? [],
        loading: false,
        error: null,
      }));
    };

    const handleError = (event: Event) => {
      const detail = (event as CustomEvent<ResolverEventDetail>).detail;
      if (detail.element !== element) {
        return;
      }
      setState((prev) => ({ ...prev, loading: false, error: detail.error ?? null }));
    };

    const handleValidation = (event: Event) => {
      const detail = (event as CustomEvent<ResolverEventDetail>).detail;
      if (detail.element !== element) {
        return;
      }
      setState((prev) => ({ ...prev, validation: detail.validation ?? null }));
    };

    element.addEventListener(LOADING_EVENT, handleLoading as EventListener);
    element.addEventListener(SUCCESS_EVENT, handleSuccess as EventListener);
    element.addEventListener(ERROR_EVENT, handleError as EventListener);
    element.addEventListener(VALIDATION_EVENT, handleValidation as EventListener);

    return () => {
      element.removeEventListener(LOADING_EVENT, handleLoading as EventListener);
      element.removeEventListener(SUCCESS_EVENT, handleSuccess as EventListener);
      element.removeEventListener(ERROR_EVENT, handleError as EventListener);
      element.removeEventListener(VALIDATION_EVENT, handleValidation as EventListener);
    };
  }, [element, registry]);

  return {
    options: state.options,
    loading: state.loading,
    error: state.error,
    validation: state.validation,
    refresh: async () => {
      if (!element || !registry) {
        return;
      }
      await registry.resolve(element);
    },
  };
}
