import type { BehaviorFactory } from "./types";
import { findNearestInput } from "./utils";

interface AutoResizeConfig {
  minRows?: number;
  maxRows?: number;
}

export const autoResize: BehaviorFactory = ({ element, config }) => {
  const target = findNearestInput(element);
  if (!(target instanceof HTMLTextAreaElement)) {
    console.warn("[formgen:behaviors] autoResize requires a textarea target.");
    return;
  }

  const options = normaliseConfig(config);
  const rowsConfig = normaliseBounds(options);

  const resize = () => {
    const computed = window.getComputedStyle(target);
    const lineHeight = resolveLineHeightPx(computed);
    if (!lineHeight) {
      return;
    }

    const paddingTop = parseFloat(computed.paddingTop || "0") || 0;
    const paddingBottom = parseFloat(computed.paddingBottom || "0") || 0;
    const borderTop = parseFloat(computed.borderTopWidth || "0") || 0;
    const borderBottom = parseFloat(computed.borderBottomWidth || "0") || 0;
    const chrome = paddingTop + paddingBottom + borderTop + borderBottom;

    target.style.height = "auto";

    const minRows = rowsConfig.minRows ?? target.rows;
    const maxRows = rowsConfig.maxRows;
    const minHeight = minRows ? lineHeight * minRows + chrome : undefined;
    const maxHeight = maxRows ? lineHeight * maxRows + chrome : undefined;

    let nextHeight = target.scrollHeight;
    if (minHeight !== undefined && nextHeight < minHeight) {
      nextHeight = minHeight;
    }
    if (maxHeight !== undefined && nextHeight > maxHeight) {
      nextHeight = maxHeight;
    }

    target.style.height = `${Math.ceil(nextHeight)}px`;

    if (rowsConfig.minRows !== undefined) {
      target.rows = rowsConfig.minRows;
    }
  };

  const handleInput = () => resize();

  target.addEventListener("input", handleInput);
  resize();

  return () => {
    target.removeEventListener("input", handleInput);
  };
};

function normaliseConfig(config: unknown): AutoResizeConfig {
  if (!config || typeof config !== "object") {
    return {};
  }
  const record = config as Record<string, unknown>;
  return {
    minRows: coercePositiveInt(record.minRows),
    maxRows: coercePositiveInt(record.maxRows),
  };
}

function coercePositiveInt(value: unknown): number | undefined {
  const parsed =
    typeof value === "number"
      ? value
      : typeof value === "string"
        ? Number.parseInt(value, 10)
        : NaN;
  if (!Number.isFinite(parsed)) {
    return undefined;
  }
  const normalized = Math.floor(parsed);
  if (normalized <= 0) {
    return undefined;
  }
  return normalized;
}

function normaliseBounds(options: AutoResizeConfig): AutoResizeConfig {
  const minRows = options.minRows;
  const maxRows = options.maxRows;
  if (minRows !== undefined && maxRows !== undefined && maxRows < minRows) {
    return { minRows, maxRows: minRows };
  }
  return options;
}

function resolveLineHeightPx(computed: CSSStyleDeclaration): number | undefined {
  const raw = computed.lineHeight;
  if (raw && raw !== "normal") {
    const parsed = Number.parseFloat(raw);
    if (Number.isFinite(parsed) && parsed > 0) {
      return parsed;
    }
  }

  const fontSize = Number.parseFloat(computed.fontSize || "");
  if (Number.isFinite(fontSize) && fontSize > 0) {
    return fontSize * 1.2;
  }

  return undefined;
}
