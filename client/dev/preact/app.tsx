import { render } from "preact";
import { useEffect, useRef } from "preact/hooks";
import { useRelationshipOptions } from "../../src/frameworks/preact";

// Tailwind classes matching vanilla renderer
const classes = {
  form: 'grid gap-6 p-6 bg-white rounded-lg border border-gray-200',
  field: 'grid gap-2',
  label: 'text-sm font-medium text-gray-900',
  input: 'w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500',
  select: 'w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500',
  textarea: 'w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500',
  actions: 'flex gap-2',
  button: 'px-4 py-2 text-sm font-medium rounded-md bg-blue-600 text-white hover:bg-blue-700',
  buttonSecondary: 'px-4 py-2 text-sm font-medium rounded-md bg-gray-100 text-gray-700 hover:bg-gray-200',
  help: 'text-sm text-gray-500',
};

// Mock API
function installMockApi() {
  const mockData = {
    authors: [
      { id: "1", full_name: "Alice Smith" },
      { id: "2", full_name: "Bob Johnson" },
      { id: "3", full_name: "Carol Williams" },
    ],
    categories: [
      { id: "tech", name: "Technology" },
      { id: "design", name: "Design" },
      { id: "business", name: "Business" },
    ],
    tags: [
      { value: "javascript", label: "JavaScript" },
      { value: "typescript", label: "TypeScript" },
      { value: "react", label: "React" },
      { value: "preact", label: "Preact" },
      { value: "vue", label: "Vue" },
    ],
    managers: [
      { id: "m1", full_name: "Sarah Manager" },
      { id: "m2", full_name: "Tom Director" },
    ],
  };

  globalThis.fetch = async (input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url;
    const searchParams = new URLSearchParams(new URL(url, 'http://localhost').search);
    const searchValue = searchParams.get('q') || searchParams.get('search') || '';

    let data: any[] = [];
    if (url.includes('/api/authors')) {
      data = mockData.authors.filter(a =>
        !searchValue || a.full_name.toLowerCase().includes(searchValue.toLowerCase())
      ).map(a => ({ value: a.id, label: a.full_name }));
    } else if (url.includes('/api/categories')) {
      data = mockData.categories.filter(c =>
        !searchValue || c.name.toLowerCase().includes(searchValue.toLowerCase())
      ).map(c => ({ value: c.id, label: c.name }));
    } else if (url.includes('/api/tags')) {
      data = mockData.tags.filter(t =>
        !searchValue || t.label.toLowerCase().includes(searchValue.toLowerCase())
      );
    } else if (url.includes('/api/managers')) {
      data = mockData.managers.filter(m =>
        !searchValue || m.full_name.toLowerCase().includes(searchValue.toLowerCase())
      ).map(m => ({ value: m.id, label: m.full_name }));
    }

    return new Response(JSON.stringify(data), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    });
  };
}

installMockApi();

// Preact Component for Select Field
function SelectField({ fieldId, name, label, required, endpoint }: {
  fieldId: string;
  name: string;
  label: string;
  required?: boolean;
  endpoint: {
    url: string;
    renderer?: string;
    mode?: string;
    searchParam?: string;
  };
}) {
  const selectRef = useRef<HTMLSelectElement>(null);
  const { options, loading, error } = useRelationshipOptions(selectRef.current);

  return (
    <div className={classes.field} data-formgen-auto-init="true">
      <label htmlFor={fieldId} className={classes.label}>{label}{required && ' *'}</label>
      <select
        ref={selectRef}
        id={fieldId}
        name={name}
        className={classes.select}
        required={required}
        data-endpoint-url={endpoint.url}
        data-endpoint-method="GET"
        data-endpoint-renderer={endpoint.renderer || 'typeahead'}
        data-endpoint-mode={endpoint.mode}
        data-endpoint-search-param={endpoint.searchParam}
        data-endpoint-field-label={label}
        data-relationship-cardinality="one"
      >
        <option value="">Select {label}</option>
      </select>
      {loading && <small className={classes.help}>Loading...</small>}
      {error && <small className={classes.help} style={{ color: '#dc2626' }}>Error: {error.message}</small>}
      {!loading && !error && <small className={classes.help}>{options.length} options loaded</small>}
    </div>
  );
}

// Preact Component for Multi-Select Field
function MultiSelectField({ fieldId, name, label, required, endpoint }: {
  fieldId: string;
  name: string;
  label: string;
  required?: boolean;
  endpoint: {
    url: string;
    renderer?: string;
    mode?: string;
    searchParam?: string;
  };
}) {
  const selectRef = useRef<HTMLSelectElement>(null);
  const { options, loading, error } = useRelationshipOptions(selectRef.current);

  return (
    <div className={classes.field} data-formgen-auto-init="true">
      <label htmlFor={fieldId} className={classes.label}>{label}{required && ' *'}</label>
      <select
        ref={selectRef}
        id={fieldId}
        name={name}
        className={classes.select}
        multiple
        required={required}
        data-endpoint-url={endpoint.url}
        data-endpoint-method="GET"
        data-endpoint-renderer={endpoint.renderer || 'chips'}
        data-endpoint-mode={endpoint.mode}
        data-endpoint-search-param={endpoint.searchParam}
        data-endpoint-field-label={label}
        data-relationship-cardinality="many"
      >
      </select>
      {loading && <small className={classes.help}>Loading...</small>}
      {error && <small className={classes.help} style={{ color: '#dc2626' }}>Error: {error.message}</small>}
      {!loading && !error && <small className={classes.help}>{options.length} options loaded</small>}
    </div>
  );
}

// Main App Component
function App() {
  useEffect(() => {
    // Setup view dropdown
    const viewSelect = document.getElementById('view-select') as HTMLSelectElement;
    if (viewSelect) {
      viewSelect.value = 'preact';

      const handleViewChange = (e: Event) => {
        const newView = (e.target as HTMLSelectElement).value;
        if (newView === 'vanilla') {
          window.location.href = '/';
        }
      };

      viewSelect.addEventListener('change', handleViewChange);

      return () => {
        viewSelect.removeEventListener('change', handleViewChange);
      };
    }
  }, []);

  return (
    <form className={classes.form} data-formgen-auto-init="true">
      <h2 style={{ margin: 0, fontSize: '1.25rem', fontWeight: 600 }}>Create Article (Preact)</h2>

      <div className={classes.field}>
        <label htmlFor="article-title" className={classes.label}>Title *</label>
        <input
          type="text"
          id="article-title"
          name="article[title]"
          className={classes.input}
          placeholder="Enter article title"
          required
        />
      </div>

      <div className={classes.field}>
        <label htmlFor="article-content" className={classes.label}>Content</label>
        <textarea
          id="article-content"
          name="article[content]"
          className={classes.textarea}
          rows={4}
          placeholder="Enter article content"
        />
      </div>

      <SelectField
        fieldId="article-author"
        name="article[author_id]"
        label="Author"
        required
        endpoint={{
          url: '/api/authors',
          renderer: 'typeahead',
          mode: 'search',
          searchParam: 'q',
        }}
      />

      <SelectField
        fieldId="article-category"
        name="article[category_id]"
        label="Category"
        endpoint={{
          url: '/api/categories',
          renderer: 'typeahead',
        }}
      />

      <MultiSelectField
        fieldId="article-tags"
        name="article[tags][]"
        label="Tags"
        endpoint={{
          url: '/api/tags',
          renderer: 'chips',
          mode: 'search',
          searchParam: 'q',
        }}
      />

      <SelectField
        fieldId="article-manager"
        name="article[manager_id]"
        label="Manager"
        endpoint={{
          url: '/api/managers',
          renderer: 'typeahead',
          mode: 'search',
          searchParam: 'q',
        }}
      />

      <div className={classes.actions}>
        <button type="submit" className={classes.button}>Submit</button>
        <button type="reset" className={classes.buttonSecondary}>Reset</button>
      </div>
    </form>
  );
}

// Render the app
const appElement = document.getElementById('app');
if (appElement) {
  render(<App />, appElement);
}
