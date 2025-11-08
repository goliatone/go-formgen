type JsonValue = string | number | boolean | null | JsonValue[] | { [key: string]: JsonValue };

type OpenAPISchema = Record<string, JsonValue>;

interface UISchemaDocument {
  operations: Record<string, UISchemaOperation>;
}

interface UISchemaOperation {
  form: {
    title?: string;
    subtitle?: string;
    layout?: {
      gridColumns?: number;
      gutter?: string;
    };
    actions?: Array<{
      kind?: string;
      label: string;
      type?: string;
      href?: string;
    }>;
    metadata?: Record<string, JsonValue>;
    uiHints?: Record<string, JsonValue>;
  };
  sections: Array<{
    id: string;
    title?: string;
    description?: string;
    order?: number;
    fieldset?: boolean;
  }>;
  fields: Record<string, UISchemaField>;
}

interface UISchemaField {
  section: string;
  order: number;
  grid?: {
    span?: number;
    start?: number;
  };
  label?: string;
  helpText?: string;
  placeholder?: string;
  required?: boolean;
  component?: string;
  renderer?: string;
  rows?: number;
  itemLabel?: string;
  nested?: string[];
  refresh?: {
    mode?: "auto" | "manual";
    on?: string[];
    triggers?: string[];
    targets?: string[];
  };
}

interface RelationshipSpec {
  type?: string;
  cardinality?: "one" | "many";
  target?: string;
  foreignKey?: string;
}

interface EndpointSpec {
  url?: string;
  method?: string;
  labelField?: string;
  valueField?: string;
  resultsPath?: string;
  mode?: string;
  searchParam?: string;
  submitAs?: string;
  params?: Record<string, string>;
  dynamicParams?: Record<string, string>;
  mapping?: Record<string, string>;
  auth?: Record<string, string>;
  throttleMs?: number;
  debounceMs?: number;
  refreshMode?: "auto" | "manual";
  refreshOn?: string[];
}

export interface ScenarioField {
  key: string;
  section: string;
  order: number;
  component: string;
  grid?: {
    span?: number;
    start?: number;
  };
  label?: string;
  placeholder?: string;
  helpText?: string;
  required?: boolean;
  rows?: number;
  enumValues?: string[];
  relationship?: RelationshipSpec;
  endpoint?: EndpointSpec;
  refresh?: {
    mode?: "auto" | "manual";
    on?: string[];
    triggers?: string[];
    targets?: string[];
  };
  nestedKeys?: string[];
  itemLabel?: string;
  schema?: OpenAPISchema;
}

export interface ScenarioSection {
  id: string;
  title?: string;
  description?: string;
  order: number;
  fieldset?: boolean;
  fields: ScenarioField[];
}

export interface SandboxScenario {
  form: UISchemaOperation["form"];
  sections: ScenarioSection[];
  fieldMap: Record<string, ScenarioField>;
}

type SchemaMap = Map<string, OpenAPISchema>;

const UI_SCHEMA_PATH = new URL("./ui-schema.json", import.meta.url).href;
const OPENAPI_PATH = new URL("./schema.json", import.meta.url).href;

export async function loadSandboxScenario(
  operationId = "createArticle",
): Promise<SandboxScenario> {
  const [schemaDoc, uiDoc] = await Promise.all([
    fetchJson<OpenAPISchema>(OPENAPI_PATH),
    fetchJson<UISchemaDocument>(UI_SCHEMA_PATH),
  ]);

  const operation = uiDoc.operations?.[operationId];
  if (!operation) {
    throw new Error(`UI schema for operation ${operationId} not found.`);
  }

  const requestSchema = locateRequestSchema(schemaDoc, operationId);
  const schemaMap: SchemaMap = new Map();
  const requiredPaths = new Set<string>();

  flattenSchema(
    schemaDoc,
    requestSchema,
    "",
    schemaMap,
  );
  collectRequiredPaths(schemaDoc, requestSchema, "", requiredPaths);

  const fieldEntries = Object.entries(operation.fields);
  const fieldMap: Record<string, ScenarioField> = {};

  for (const [key, config] of fieldEntries) {
    const schemaNode = schemaMap.get(key);
    const required = config.required ?? requiredPaths.has(key);
    fieldMap[key] = buildScenarioField(
      key,
      config,
      schemaNode,
      required,
    );
  }

  const sections = operation.sections
    .slice()
    .sort((a, b) => (a.order ?? 0) - (b.order ?? 0))
    .map<ScenarioSection>((section) => {
      const sectionFields = fieldEntries
        .filter(([, field]) => field.section === section.id)
        .sort(([, a], [, b]) => a.order - b.order)
        .map(([fieldKey]) => fieldMap[fieldKey])
        .filter(Boolean);
      return {
        id: section.id,
        title: section.title,
        description: section.description,
        order: section.order ?? 0,
        fieldset: section.fieldset,
        fields: sectionFields,
      };
    });

  return {
    form: operation.form,
    sections,
    fieldMap,
  };
}

async function fetchJson<T>(path: string): Promise<T> {
  const response = await fetch(path, { cache: "no-store" });
  if (!response.ok) {
    throw new Error(`Failed to load ${path}: ${response.status}`);
  }
  return response.json() as Promise<T>;
}

function locateRequestSchema(doc: OpenAPISchema, operationId: string): OpenAPISchema {
  const paths = doc.paths as Record<string, JsonValue>;
  if (!paths) {
    throw new Error("OpenAPI document missing paths.");
  }

  for (const [, methods] of Object.entries(paths)) {
    if (typeof methods !== "object" || methods == null) {
      continue;
    }
    for (const [, operation] of Object.entries(methods as Record<string, JsonValue>)) {
      if (
        typeof operation === "object" &&
        operation != null &&
        (operation as Record<string, JsonValue>).operationId === operationId
      ) {
        const requestBody = (operation as Record<string, JsonValue>).requestBody as Record<string, JsonValue>;
        const content = requestBody?.content as Record<string, JsonValue>;
        const jsonSchema = content?.["application/json"] as Record<string, JsonValue>;
        const schema = jsonSchema?.schema as OpenAPISchema;
        if (!schema) {
          throw new Error(`Operation ${operationId} missing request schema.`);
        }
        return schema;
      }
    }
  }

  throw new Error(`Operation ${operationId} not found in OpenAPI document.`);
}

function flattenSchema(
  doc: OpenAPISchema,
  schema: OpenAPISchema,
  path: string,
  map: SchemaMap,
): void {
  const resolved = dereferenceSchema(doc, schema);
  if (path) {
    map.set(path, resolved);
  }

  if (isObject(resolved.properties)) {
    const properties = resolved.properties as Record<string, JsonValue>;
    for (const [key, value] of Object.entries(properties)) {
      if (!isObject(value)) {
        continue;
      }
      const nextPath = path ? `${path}.${key}` : key;
      flattenSchema(doc, value as OpenAPISchema, nextPath, map);
    }
  }

  if (resolved.items && typeof resolved.items === "object") {
    const itemSchema = resolved.items as OpenAPISchema;
    const itemPath = path ? `${path}[]` : "[]";
    flattenSchema(doc, itemSchema, itemPath, map);
  }
}

function collectRequiredPaths(
  doc: OpenAPISchema,
  schema: OpenAPISchema,
  path: string,
  required: Set<string>,
): void {
  const resolved = dereferenceSchema(doc, schema);
  if (Array.isArray(resolved.required)) {
    for (const key of resolved.required) {
      const next = path ? `${path}.${String(key)}` : String(key);
      required.add(next);
    }
  }

  if (isObject(resolved.properties)) {
    const properties = resolved.properties as Record<string, JsonValue>;
    for (const [key, value] of Object.entries(properties)) {
      if (!isObject(value)) {
        continue;
      }
      const nextPath = path ? `${path}.${key}` : key;
      collectRequiredPaths(doc, value as OpenAPISchema, nextPath, required);
    }
  }

  if (resolved.items && typeof resolved.items === "object") {
    collectRequiredPaths(doc, resolved.items as OpenAPISchema, `${path}[]`, required);
  }
}

function buildScenarioField(
  key: string,
  config: UISchemaField,
  schemaNode: OpenAPISchema | undefined,
  required: boolean,
): ScenarioField {
  const component = config.component ?? inferComponent(schemaNode);
  const label = config.label ?? humanizeKey(key);
  const enumValues = Array.isArray(schemaNode?.enum)
    ? (schemaNode?.enum as JsonValue[]).map((value) => String(value))
    : undefined;

  const relationship = extractRelationship(schemaNode);
  const endpoint = extractEndpoint(schemaNode);

  return {
    key,
    section: config.section,
    order: config.order,
    component,
    grid: config.grid,
    label,
    placeholder: config.placeholder,
    helpText: config.helpText ?? (typeof schemaNode?.description === "string" ? schemaNode?.description : undefined),
    required,
    rows: config.rows,
    enumValues,
    relationship,
    endpoint,
    refresh: config.refresh,
    nestedKeys: config.nested,
    itemLabel: config.itemLabel,
    schema: schemaNode,
  };
}

function inferComponent(schemaNode: OpenAPISchema | undefined): string {
  if (!schemaNode) {
    return "input";
  }
  const type = schemaNode.type;
  if (type === "boolean") {
    return "checkbox";
  }
  if (type === "integer" || type === "number") {
    return "number";
  }
  if (type === "array") {
    return "array";
  }
  if (type === "string" && schemaNode.format === "date-time") {
    return "datetime";
  }
  if (type === "string" && (schemaNode.maxLength ?? 0) > 180) {
    return "textarea";
  }
  return "input";
}

function extractRelationship(schemaNode: OpenAPISchema | undefined): RelationshipSpec | undefined {
  if (!schemaNode || !isObject(schemaNode["x-relationships"])) {
    return undefined;
  }
  const rel = schemaNode["x-relationships"] as Record<string, JsonValue>;
  const spec: RelationshipSpec = {
    type: typeof rel.type === "string" ? rel.type : undefined,
    target: typeof rel.target === "string" ? rel.target : undefined,
    foreignKey: typeof rel.foreignKey === "string" ? rel.foreignKey : undefined,
  };
  if (typeof rel.cardinality === "string") {
    spec.cardinality = rel.cardinality === "many" ? "many" : "one";
  }
  return spec;
}

function extractEndpoint(schemaNode: OpenAPISchema | undefined): EndpointSpec | undefined {
  if (!schemaNode || !isObject(schemaNode["x-endpoint"])) {
    return undefined;
  }
  const endpoint = schemaNode["x-endpoint"] as Record<string, JsonValue>;
  const dynamicParams = toStringMap(endpoint.dynamicParams);
  const params = toStringMap(endpoint.params);
  const mapping = toStringMap(endpoint.mapping);
  const auth = toStringMap(endpoint.auth);

  const spec: EndpointSpec = {
    url: toString(endpoint.url),
    method: toString(endpoint.method),
    labelField: toString(endpoint.labelField),
    valueField: toString(endpoint.valueField),
    resultsPath: toString(endpoint.resultsPath),
    mode: toString(endpoint.mode),
    searchParam: toString(endpoint.searchParam),
    submitAs: toString(endpoint.submitAs),
    params: params ?? undefined,
    dynamicParams: dynamicParams ?? undefined,
    mapping: mapping ?? undefined,
    auth: auth ?? undefined,
  };

  const throttle = Number(endpoint.throttleMs ?? endpoint.throttle);
  if (!Number.isNaN(throttle) && throttle > 0) {
    spec.throttleMs = throttle;
  }
  const debounce = Number(endpoint.debounceMs ?? endpoint.debounce);
  if (!Number.isNaN(debounce) && debounce > 0) {
    spec.debounceMs = debounce;
  }

  if (dynamicParams) {
    const references = extractFieldReferences(dynamicParams);
    if (references.length > 0) {
      spec.refreshOn = references;
    }
  }

  const refresh = toString(endpoint.refresh ?? endpoint.refreshMode);
  if (refresh === "manual" || refresh === "auto") {
    spec.refreshMode = refresh;
  }

  return spec;
}

function extractFieldReferences(map: Record<string, string>): string[] {
  const refs = new Set<string>();
  Object.values(map).forEach((value) => {
    const matches = String(value).match(/\{\{field:([^}]+)\}\}/g);
    matches?.forEach((match) => {
      const field = match.replace(/\{\{field:([^}]+)\}\}/, "$1").trim();
      if (field) {
        refs.add(field);
      }
    });
  });
  return Array.from(refs);
}

function dereferenceSchema(doc: OpenAPISchema, schema: OpenAPISchema): OpenAPISchema {
  if (!schema) {
    return {};
  }
  if (typeof schema.$ref === "string") {
    const resolved = resolveRef(doc, schema.$ref);
    return dereferenceSchema(doc, resolved);
  }

  if (Array.isArray(schema.allOf) && schema.allOf.length > 0) {
    return schema.allOf.reduce<OpenAPISchema>(
      (acc, item) => ({
        ...acc,
        ...dereferenceSchema(doc, item as OpenAPISchema),
      }),
      { ...schema, allOf: undefined },
    );
  }

  return schema;
}

function resolveRef(doc: OpenAPISchema, ref: string): OpenAPISchema {
  if (!ref.startsWith("#/")) {
    throw new Error(`Unsupported $ref: ${ref}`);
  }
  const parts = ref.slice(2).split("/");
  let current: any = doc;
  for (const part of parts) {
    if (current == null) {
      break;
    }
    current = current[part];
  }
  if (!isObject(current)) {
    throw new Error(`Unable to resolve reference: ${ref}`);
  }
  return current as OpenAPISchema;
}

function humanizeKey(key: string): string {
  const cleaned = key.split(".").pop() ?? key;
  const withoutArray = cleaned.replace("\\[]", "").replace("[]", "");
  return withoutArray
    .replace(/_/g, " ")
    .replace(/\\b\\w/g, (char) => char.toUpperCase())
    .trim();
}

function toStringMap(value: JsonValue | undefined): Record<string, string> | null {
  if (!isObject(value)) {
    return null;
  }
  const record = value as Record<string, JsonValue>;
  const result: Record<string, string> = {};
  Object.entries(record).forEach(([key, val]) => {
    const str = toString(val);
    if (str) {
      result[key] = str;
    }
  });
  return Object.keys(result).length > 0 ? result : null;
}

function toString(value: JsonValue | undefined): string | undefined {
  if (value == null) {
    return undefined;
  }
  return String(value);
}

function isObject(value: unknown): value is Record<string, JsonValue> {
  return typeof value === "object" && value != null && !Array.isArray(value);
}
