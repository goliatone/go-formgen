type OptionRecord = { value: string; label: string };

type AuthorRecord = {
  id: string;
  full_name: string;
  tenantId: string;
};

type CategoryRecord = {
  id: string;
  name: string;
  tenantId: string;
};

type TagRecord = {
  value: string;
  label: string;
  tenantId: string;
  categoryId?: string;
};

type ManagerRecord = {
  id: string;
  full_name: string;
  authorId?: string;
  tenantId: string;
};

type ContributorRecord = {
  id: string;
  full_name: string;
  tenantId: string;
  statuses: string[];
};

type ArticleRecord = {
  id: string;
  title: string;
  tenantId: string;
  categoryId: string;
};

type MockDataset = {
  authors: AuthorRecord[];
  categories: CategoryRecord[];
  tags: TagRecord[];
  managers: ManagerRecord[];
  contributors: ContributorRecord[];
  articles: ArticleRecord[];
};

const mockData: MockDataset = {
  authors: [
    { id: "1", full_name: "Alice Smith", tenantId: "garden" },
    { id: "2", full_name: "Bob Johnson", tenantId: "garden" },
    { id: "3", full_name: "Carol Williams", tenantId: "archive" },
    { id: "4", full_name: "Zoe Alvarez", tenantId: "archive" },
    { id: "5", full_name: "Mason Fletcher", tenantId: "lumen" },
    { id: "6", full_name: "Priya Shah", tenantId: "lumen" },
  ],
  categories: [
    { id: "news", name: "News", tenantId: "garden" },
    { id: "product", name: "Product", tenantId: "garden" },
    { id: "culture", name: "Culture", tenantId: "archive" },
    { id: "ops", name: "Operations", tenantId: "archive" },
    { id: "design", name: "Design", tenantId: "lumen" },
    { id: "growth", name: "Growth", tenantId: "lumen" },
  ],
  tags: [
    { value: "design", label: "Product Design", tenantId: "lumen", categoryId: "design" },
    { value: "growth", label: "Growth", tenantId: "lumen", categoryId: "growth" },
    { value: "ml", label: "Machine Learning", tenantId: "garden", categoryId: "product" },
    { value: "ops", label: "Operations", tenantId: "archive", categoryId: "ops" },
    { value: "security", label: "Security", tenantId: "archive", categoryId: "ops" },
    { value: "ai", label: "AI Strategy", tenantId: "garden", categoryId: "product" },
    { value: "devrel", label: "DevRel", tenantId: "garden", categoryId: "news" },
    { value: "javascript", label: "JavaScript", tenantId: "garden" },
    { value: "typescript", label: "TypeScript", tenantId: "garden" },
    { value: "react", label: "React", tenantId: "lumen" },
    { value: "preact", label: "Preact", tenantId: "lumen" },
    { value: "vue", label: "Vue", tenantId: "archive" },
  ],
  managers: [
    { id: "m1", full_name: "Sarah Manager", authorId: "1", tenantId: "garden" },
    { id: "m2", full_name: "Tom Director", authorId: "2", tenantId: "garden" },
    { id: "lead-1", full_name: "James Okafor", authorId: "3", tenantId: "archive" },
    { id: "lead-2", full_name: "Helena Park", authorId: "4", tenantId: "archive" },
    { id: "lead-3", full_name: "Sara Lindholm", tenantId: "lumen" },
  ],
  contributors: [
    { id: "c1", full_name: "Leah Proof", tenantId: "garden", statuses: ["draft", "in_review"] },
    { id: "c2", full_name: "Glen Fact", tenantId: "garden", statuses: ["in_review"] },
    { id: "c3", full_name: "Ravi Producer", tenantId: "archive", statuses: ["scheduled", "published"] },
    { id: "c4", full_name: "Mila Copy", tenantId: "archive", statuses: ["draft", "scheduled"] },
    { id: "c5", full_name: "Avery Editor", tenantId: "lumen", statuses: ["draft", "in_review", "scheduled"] },
  ],
  articles: [
    { id: "a1", title: "Garden irrigation best practices", tenantId: "garden", categoryId: "product" },
    { id: "a2", title: "Ops playbook for remote teams", tenantId: "archive", categoryId: "ops" },
    { id: "a3", title: "Design systems in 2024", tenantId: "lumen", categoryId: "design" },
    { id: "a4", title: "Scaling customer success", tenantId: "garden", categoryId: "news" },
    { id: "a5", title: "Security considerations for AI", tenantId: "archive", categoryId: "ops" },
    { id: "a6", title: "Migration to Preact signals", tenantId: "lumen", categoryId: "growth" },
  ],
};

const RESPONSE_HEADERS = { "Content-Type": "application/json" };

function toLowerIncludes(value: string, search: string): boolean {
  return value.toLowerCase().includes(search.toLowerCase());
}

function applySearch(records: OptionRecord[], searchValue: string): OptionRecord[] {
  const query = searchValue.trim().toLowerCase();
  if (!query) {
    return records;
  }
  return records.filter(
    (item) =>
      toLowerIncludes(item.value, query) || toLowerIncludes(item.label, query),
  );
}

function applyLimit(records: OptionRecord[], url: URL): OptionRecord[] {
  const limitParam = url.searchParams.get("limit") ?? url.searchParams.get("per_page");
  const limit = limitParam ? Number(limitParam) : undefined;
  if (limit && !Number.isNaN(limit) && limit > 0) {
    return records.slice(0, limit);
  }
  return records;
}

function respondWithOptions(records: OptionRecord[], url: URL): Response {
  const limited = applyLimit(records, url);
  return new Response(JSON.stringify(limited), {
    status: 200,
    headers: RESPONSE_HEADERS,
  });
}

export function installMockApi(): void {
  const flag = "__formgenMockApiInstalled";
  const globalFlags = globalThis as typeof globalThis & Record<string, unknown>;
  if (globalFlags[flag]) {
    return;
  }

  const realFetch =
    typeof globalThis.fetch === "function"
      ? globalThis.fetch.bind(globalThis)
      : undefined;

  globalThis.fetch = async (
    input: RequestInfo | URL,
    init?: RequestInit,
  ): Promise<Response> => {
    const href =
      typeof input === "string"
        ? input
        : input instanceof URL
          ? input.href
          : input.url;
    const url = new URL(href, "http://localhost");
    const searchValue =
      url.searchParams.get("q") ??
      url.searchParams.get("search") ??
      "";

    if (url.pathname.includes("/api/authors")) {
      const tenantId = url.searchParams.get("tenant_id");
      const records = mockData.authors
        .filter((author) => (tenantId ? author.tenantId === tenantId : true))
        .map<OptionRecord>((author) => ({
          value: author.id,
          label: author.full_name,
        }));
      return respondWithOptions(applySearch(records, searchValue), url);
    }

    if (url.pathname.includes("/api/categories")) {
      const tenantId = url.searchParams.get("tenant_id");
      const records = mockData.categories
        .filter((category) => (tenantId ? category.tenantId === tenantId : true))
        .map<OptionRecord>((category) => ({
          value: category.id,
          label: category.name,
        }));
      return respondWithOptions(applySearch(records, searchValue), url);
    }

    if (url.pathname.includes("/api/tags")) {
      const tenantId = url.searchParams.get("tenant_id");
      const categoryId = url.searchParams.get("category_id");
      const records = mockData.tags
        .filter((tag) => (tenantId ? tag.tenantId === tenantId : true))
        .filter((tag) => (categoryId ? tag.categoryId === categoryId || !tag.categoryId : true))
        .map<OptionRecord>((tag) => ({ value: tag.value, label: tag.label }));
      return respondWithOptions(applySearch(records, searchValue), url);
    }

    if (url.pathname.includes("/api/managers")) {
      const authorId = url.searchParams.get("author_id");
      const tenantId = url.searchParams.get("tenant_id");
      const records = mockData.managers
        .filter((manager) => (tenantId ? manager.tenantId === tenantId : true))
        .filter((manager) => (authorId ? manager.authorId === authorId : true))
        .map<OptionRecord>((manager) => ({
          value: manager.id,
          label: manager.full_name,
        }));
      return respondWithOptions(applySearch(records, searchValue), url);
    }

    if (url.pathname.includes("/api/contributors")) {
      const tenantId = url.searchParams.get("tenant_id");
      const status = url.searchParams.get("status");
      const records = mockData.contributors
        .filter((record) => (tenantId ? record.tenantId === tenantId : true))
        .filter((record) => (status ? record.statuses.includes(status) : true))
        .map<OptionRecord>((contributor) => ({
          value: contributor.id,
          label: contributor.full_name,
        }));
      return respondWithOptions(applySearch(records, searchValue), url);
    }

    if (url.pathname.includes("/api/articles")) {
      const tenantId = url.searchParams.get("tenant_id");
      const categoryId = url.searchParams.get("category_id");
      const records = mockData.articles
        .filter((article) => (tenantId ? article.tenantId === tenantId : true))
        .filter((article) => (categoryId ? article.categoryId === categoryId : true))
        .map<OptionRecord>((article) => ({ value: article.id, label: article.title }));
      return respondWithOptions(applySearch(records, searchValue), url);
    }

    if (url.pathname.includes("/api/relationships")) {
      return new Response(JSON.stringify([]), {
        status: 200,
        headers: RESPONSE_HEADERS,
      });
    }

    if (realFetch) {
      return realFetch(input as RequestInfo, init);
    }

    throw new Error("No mock handler matched and fetch fallback is unavailable.");
  };

  globalFlags[flag] = true;
}
