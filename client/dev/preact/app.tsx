import "../../src/theme/index.css";
import { render } from "preact";
import type { JSX } from "preact";
import { useEffect, useMemo, useRef, useState } from "preact/hooks";
import { useRelationshipOptions } from "../../src/frameworks/preact";
import { initBehaviors } from "../../src/behaviors";
import { installMockApi } from "../mock-api";
import {
  loadSandboxScenario,
  type ScenarioField,
  type SandboxScenario,
} from "../scenario-loader";

installMockApi();

const classes = {
  form: "max-w-5xl mx-auto space-y-6 p-6 bg-white rounded-xl border border-gray-200 dark:bg-slate-900 dark:border-gray-700",
  field: "space-y-2",
  label: "block text-sm font-medium text-gray-800 mb-2 dark:text-white",
  input: "py-3 px-4 block w-full border-gray-200 rounded-lg text-sm focus:border-blue-500 focus:ring-blue-500 disabled:opacity-50 disabled:pointer-events-none dark:bg-slate-900 dark:border-gray-700 dark:text-gray-400 dark:focus:ring-gray-600",
  select: "py-3 px-4 pe-9 block w-full border-gray-200 rounded-lg text-sm focus:border-blue-500 focus:ring-blue-500 disabled:opacity-50 disabled:pointer-events-none dark:bg-slate-900 dark:border-gray-700 dark:text-gray-400 dark:focus:ring-gray-600",
  textarea: "py-3 px-4 block w-full border-gray-200 rounded-lg text-sm focus:border-blue-500 focus:ring-blue-500 disabled:opacity-50 disabled:pointer-events-none dark:bg-slate-900 dark:border-gray-700 dark:text-gray-400 dark:focus:ring-gray-600",
  actions: "flex gap-x-2 pt-4 border-t border-gray-200 dark:border-gray-700",
  button: "py-3 px-4 inline-flex justify-center items-center gap-x-2 text-sm font-medium rounded-lg border border-transparent bg-blue-600 text-white hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-600 focus:ring-offset-2 disabled:opacity-50 disabled:pointer-events-none",
  buttonSecondary: "py-3 px-4 inline-flex justify-center items-center gap-x-2 text-sm font-medium rounded-lg border border-gray-200 bg-white text-gray-800 shadow-sm hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-gray-400 focus:ring-offset-2 dark:bg-slate-900 dark:border-gray-700 dark:text-white dark:hover:bg-gray-800 disabled:opacity-50 disabled:pointer-events-none",
  help: "text-xs text-gray-500 dark:text-gray-400",
};

function fieldKeyToId(key: string): string {
  return `fg-${key.replace(/[^a-zA-Z0-9]+/g, "-")}`;
}

function fieldKeyToName(key: string, isMultiple: boolean): string {
  const parts = key.split(".");
  if (parts.length === 0) {
    return key;
  }
  const [root, ...rest] = parts;
  let name = root;
  rest.forEach((part) => {
    if (part.endsWith("[]")) {
      const trimmed = part.slice(0, -2);
      name += `[${trimmed}][]`;
    } else {
      name += `[${part}]`;
    }
  });
  if (isMultiple) {
    name += name.endsWith("[]") ? "" : "[]";
  }
  return name;
}

function toDataAttributeKey(key: string): string {
  return key
    .replace(/[^a-zA-Z0-9]+/g, "-")
    .replace(/-{2,}/g, "-")
    .replace(/^-|-$/g, "")
    .toLowerCase();
}

function mergeRefreshTargets(field: ScenarioField): string[] | undefined {
  const fromEndpoint = field.endpoint?.refreshOn ?? [];
  const fromConfig = field.refresh?.on ?? [];
  const merged = Array.from(new Set([...fromEndpoint, ...fromConfig]));
  return merged.length > 0 ? merged : undefined;
}

function buildRelationshipAttributes(field: ScenarioField, isMultiple: boolean): Record<string, string> {
  const attrs: Record<string, string> = {};
  const endpoint = field.endpoint;
  const relationship = field.relationship;

  if (endpoint?.url) attrs["data-endpoint-url"] = endpoint.url;
  if (endpoint?.method) attrs["data-endpoint-method"] = endpoint.method.toUpperCase();
  if (endpoint?.labelField) attrs["data-endpoint-label-field"] = endpoint.labelField;
  if (endpoint?.valueField) attrs["data-endpoint-value-field"] = endpoint.valueField;
  if (endpoint?.resultsPath) attrs["data-endpoint-results-path"] = endpoint.resultsPath;
  if (endpoint?.mode) attrs["data-endpoint-mode"] = endpoint.mode;
  if (endpoint?.searchParam) attrs["data-endpoint-search-param"] = endpoint.searchParam;
  if (endpoint?.submitAs) attrs["data-endpoint-submit-as"] = endpoint.submitAs;
  if (endpoint?.throttleMs) attrs["data-endpoint-throttle"] = String(endpoint.throttleMs);
  if (endpoint?.debounceMs) attrs["data-endpoint-debounce"] = String(endpoint.debounceMs);
  if (field.label) attrs["data-endpoint-field-label"] = field.label;

  const refreshMode = field.refresh?.mode ?? endpoint?.refreshMode;
  if (refreshMode) {
    attrs["data-endpoint-refresh"] = refreshMode;
  }

  const refreshOn = mergeRefreshTargets(field);
  if (refreshOn && refreshOn.length > 0) {
    attrs["data-endpoint-refresh-on"] = refreshOn.join(",");
  }

  if (endpoint?.params) {
    for (const [key, value] of Object.entries(endpoint.params)) {
      attrs[`data-endpoint-params-${toDataAttributeKey(key)}`] = value;
    }
  }

  if (endpoint?.dynamicParams) {
    for (const [key, value] of Object.entries(endpoint.dynamicParams)) {
      attrs[`data-endpoint-dynamic-params-${toDataAttributeKey(key)}`] = value;
    }
  }

  if (endpoint?.mapping) {
    if (endpoint.mapping.value) {
      attrs["data-endpoint-mapping-value"] = endpoint.mapping.value;
    }
    if (endpoint.mapping.label) {
      attrs["data-endpoint-mapping-label"] = endpoint.mapping.label;
    }
  }

  if (endpoint?.auth) {
    if (endpoint.auth.strategy) {
      attrs["data-endpoint-auth-strategy"] = endpoint.auth.strategy;
    }
    if (endpoint.auth.header) {
      attrs["data-endpoint-auth-header"] = endpoint.auth.header;
    }
    if (endpoint.auth.source) {
      attrs["data-endpoint-auth-source"] = endpoint.auth.source;
    }
  }

  if (relationship?.type) {
    attrs["data-relationship-type"] = relationship.type;
  }
  if (relationship?.target) {
    attrs["data-relationship-target"] = relationship.target;
  }
  if (relationship?.foreignKey) {
    attrs["data-relationship-foreign-key"] = relationship.foreignKey;
  }
  const cardinality = relationship?.cardinality ?? (isMultiple ? "many" : "one");
  attrs["data-relationship-cardinality"] = cardinality;

  return attrs;
}

function RelationshipSelect({ field }: { field: ScenarioField }) {
  const selectRef = useRef<HTMLSelectElement>(null);
  const isMultiple = field.relationship?.cardinality === "many" || field.schema?.type === "array";
  const { options, loading, error } = useRelationshipOptions(selectRef.current);

  const relationshipAttrs = useMemo(() => buildRelationshipAttributes(field, isMultiple), [field, isMultiple]);
  const id = fieldKeyToId(field.key);
  const name = fieldKeyToName(field.key, isMultiple);
  const placeholder = field.placeholder ?? (isMultiple ? "Select options" : `Select ${field.label ?? "option"}`);

  return (
    <div className={classes.field} data-formgen-auto-init="true">
      {field.label && (
        <label htmlFor={id} className={classes.label}>
          {field.label}
          {field.required && " *"}
        </label>
      )}
      <select
        ref={selectRef}
        id={id}
        name={name}
        className={classes.select}
        required={field.required}
        multiple={isMultiple}
        {...relationshipAttrs}
      >
        {!isMultiple && <option value="">{placeholder}</option>}
      </select>
      {field.helpText && <small className={classes.help}>{field.helpText}</small>}
      {loading && <small className={classes.help}>Loading…</small>}
      {error && (
        <small className={classes.help} style={{ color: "#dc2626" }}>
          Error: {error.message}
        </small>
      )}
      {!loading && !error && (
        <small className={classes.help}>{options.length} options loaded</small>
      )}
      {field.refresh?.mode === "manual" && field.refresh.triggers && (
        <div className="flex gap-2 pt-2">
          {field.refresh.triggers.map((trigger) => (
            <button
              key={trigger}
              type="button"
              className={classes.buttonSecondary}
              data-endpoint-refresh-trigger="true"
              data-endpoint-refresh-target={name}
              data-trigger-id={trigger}
            >
              Refresh {field.label ?? "options"}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

function renderStandardField(field: ScenarioField) {
  const id = fieldKeyToId(field.key);
  const name = fieldKeyToName(field.key, field.schema?.type === "array");

  switch (field.component) {
    case "checkbox":
      return (
        <div className={`${classes.field} flex items-center gap-3`}>
          <input type="checkbox" id={id} name={name} className="shrink-0 h-4 w-4 text-blue-600 border-gray-300 rounded" />
          <label htmlFor={id} className={classes.label} style={{ marginBottom: 0 }}>
            {field.label}
          </label>
          {field.helpText && <small className={classes.help}>{field.helpText}</small>}
        </div>
      );
    case "textarea":
      return (
        <div className={classes.field}>
          {field.label && (
            <label htmlFor={id} className={classes.label}>
              {field.label}
              {field.required && " *"}
            </label>
          )}
          <textarea
            id={id}
            name={name}
            className={classes.textarea}
            rows={field.rows ?? 4}
            placeholder={field.placeholder}
            required={field.required}
          />
          {field.helpText && <small className={classes.help}>{field.helpText}</small>}
        </div>
      );
    case "select":
      return (
        <div className={classes.field}>
          {field.label && (
            <label htmlFor={id} className={classes.label}>
              {field.label}
              {field.required && " *"}
            </label>
          )}
          <select
            id={id}
            name={name}
            className={classes.select}
            required={field.required}
            defaultValue=""
          >
            <option value="" disabled>
              {field.placeholder ?? `Select ${field.label ?? "option"}`}
            </option>
            {field.enumValues?.map((value) => (
              <option key={value} value={value}>
                {value.replace(/_/g, " ")}
              </option>
            ))}
          </select>
          {field.helpText && <small className={classes.help}>{field.helpText}</small>}
        </div>
      );
    case "number":
      return (
        <div className={classes.field}>
          {field.label && (
            <label htmlFor={id} className={classes.label}>
              {field.label}
              {field.required && " *"}
            </label>
          )}
          <input
            type="number"
            id={id}
            name={name}
            className={classes.input}
            placeholder={field.placeholder}
            required={field.required}
          />
          {field.helpText && <small className={classes.help}>{field.helpText}</small>}
        </div>
      );
    case "datetime":
      return (
        <div className={classes.field}>
          {field.label && (
            <label htmlFor={id} className={classes.label}>
              {field.label}
              {field.required && " *"}
            </label>
          )}
          <input
            type="datetime-local"
            id={id}
            name={name}
            className={classes.input}
            required={field.required}
          />
          {field.helpText && <small className={classes.help}>{field.helpText}</small>}
        </div>
      );
    default:
      return (
        <div className={classes.field}>
          {field.label && (
            <label htmlFor={id} className={classes.label}>
              {field.label}
              {field.required && " *"}
            </label>
          )}
          <input
            type="text"
            id={id}
            name={name}
            className={classes.input}
            placeholder={field.placeholder}
            required={field.required}
          />
          {field.helpText && <small className={classes.help}>{field.helpText}</small>}
        </div>
      );
  }
}

function ArrayField({ field, scenario }: { field: ScenarioField; scenario: SandboxScenario }) {
  const nested = field.nestedKeys?.map((key) => scenario.fieldMap[key]).filter(Boolean) ?? [];
  return (
    <div className="space-y-3" data-component="array">
      {field.label && <h3 className="text-sm font-semibold text-gray-800 dark:text-white">{field.label}</h3>}
      {field.helpText && <p className={classes.help}>{field.helpText}</p>}
      <div className="space-y-4" data-relationship-collection="many">
        <div className="grid gap-4" style={{ gridTemplateColumns: "repeat(12, minmax(0, 1fr))" }}>
          {nested.map((nestedField) => {
            const span = nestedField.grid?.span ?? 12;
            const style: JSX.CSSProperties = {
              gridColumn: `span ${span} / span ${span}`,
            };
            if (nestedField.grid?.start) {
              style.gridColumnStart = String(nestedField.grid.start);
            }
            return (
              <div key={nestedField.key} style={style}>
                {renderField(nestedField, scenario)}
              </div>
            );
          })}
        </div>
        <button type="button" className={classes.buttonSecondary} data-relationship-action="add">
          Add {field.itemLabel ?? "item"}
        </button>
      </div>
    </div>
  );
}

function renderField(field: ScenarioField, scenario: SandboxScenario): JSX.Element {
  if (field.component === "array") {
    return <ArrayField field={field} scenario={scenario} />;
  }

  if (field.component === "relationship" || field.relationship) {
    return <RelationshipSelect field={field} />;
  }

  return renderStandardField(field);
}

function Section({ section, scenario, gridColumns }: {
  section: SandboxScenario["sections"][number];
  scenario: SandboxScenario;
  gridColumns: number;
}) {
  const gridStyle: JSX.CSSProperties = {
    gridTemplateColumns: `repeat(${gridColumns}, minmax(0, 1fr))`,
  };

  return (
    <section className="space-y-4" data-section-id={section.id}>
      {(section.title || section.description) && (
        <header className="space-y-1">
          {section.title && <h2 className="text-lg font-semibold text-gray-900 dark:text-white">{section.title}</h2>}
          {section.description && <p className="text-sm text-gray-600 dark:text-gray-400">{section.description}</p>}
        </header>
      )}
      <div className="grid gap-6" style={gridStyle}>
        {section.fields.map((field) => {
          const span = field.grid?.span ?? gridColumns;
          const style: JSX.CSSProperties = {
            gridColumn: `span ${span} / span ${span}`,
          };
          if (field.grid?.start) {
            style.gridColumnStart = String(field.grid.start);
          }
          return (
            <div key={field.key} style={style}>
              {renderField(field, scenario)}
            </div>
          );
        })}
      </div>
    </section>
  );
}

function App() {
  const [scenario, setScenario] = useState<SandboxScenario | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const viewSelect = document.getElementById("view-select") as HTMLSelectElement | null;
    if (viewSelect) {
      viewSelect.value = "preact";
      const handler = (event: Event) => {
        const value = (event.target as HTMLSelectElement).value;
        if (value === "vanilla") {
          window.location.href = "/";
        }
      };
      viewSelect.addEventListener("change", handler);
      return () => viewSelect.removeEventListener("change", handler);
    }
    return undefined;
  }, []);

  useEffect(() => {
    let cancelled = false;
    loadSandboxScenario()
      .then((data) => {
        if (!cancelled) {
          setScenario(data);
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : String(err));
        }
      });
    return () => {
      cancelled = true;
    };
  }, []);

  if (error) {
    return (
      <div className="max-w-3xl mx-auto text-red-600">
        Failed to load sandbox scenario: {error}
      </div>
    );
  }

  useEffect(() => {
    if (!scenario) {
      return;
    }
    const result = initBehaviors();
    return () => result.dispose();
  }, [scenario]);

  if (!scenario) {
    return <div className="max-w-3xl mx-auto text-sm text-gray-600">Loading sandbox…</div>;
  }

  const layout = scenario.form.layout ?? {};
  const gridColumns = layout.gridColumns ?? 12;

  return (
    <form className={classes.form} data-formgen-auto-init="true">
      {scenario.form.title && <h1 className="text-2xl font-bold text-gray-900 dark:text-white">{scenario.form.title}</h1>}
      {scenario.form.subtitle && <p className="text-sm text-gray-600 dark:text-gray-400">{scenario.form.subtitle}</p>}

      {scenario.sections.map((section) => (
        <Section
          key={section.id}
          section={section}
          scenario={scenario}
          gridColumns={gridColumns}
        />
      ))}

      <div className={classes.actions}>
        {scenario.form.actions?.map((action, index) => {
          if (action.href) {
            return (
              <a
                key={`${action.label}-${index}`}
                href={action.href}
                className={classes.buttonSecondary}
              >
                {action.label}
              </a>
            );
          }
          const kind = action.kind === "secondary" ? classes.buttonSecondary : classes.button;
          return (
            <button
              key={`${action.label}-${index}`}
              type={action.type ?? "submit"}
              className={kind}
            >
              {action.label}
            </button>
          );
        }) ?? (
          <button type="submit" className={classes.button}>
            Submit
          </button>
        )}
      </div>
    </form>
  );
}

const appElement = document.getElementById("app");
if (appElement) {
  render(<App />, appElement);
}
