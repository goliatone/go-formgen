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
    { value: "culture", label: "Culture Stories", tenantId: "archive", categoryId: "culture" },
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
    { id: "lead-3", full_name: "Sara Lindholm", authorId: "5", tenantId: "lumen" },
    { id: "lead-4", full_name: "Nico Estrada", authorId: "6", tenantId: "lumen" },
  ],
  contributors: [
    { id: "c1", full_name: "Leah Proof", tenantId: "garden", statuses: ["draft", "in_review"] },
    { id: "c2", full_name: "Glen Fact", tenantId: "garden", statuses: ["in_review", "scheduled"] },
    { id: "c6", full_name: "Ivy Publish", tenantId: "garden", statuses: ["scheduled", "published"] },
    { id: "c3", full_name: "Ravi Producer", tenantId: "archive", statuses: ["scheduled", "published"] },
    { id: "c4", full_name: "Mila Copy", tenantId: "archive", statuses: ["draft", "scheduled"] },
    { id: "c7", full_name: "Noor Review", tenantId: "archive", statuses: ["in_review"] },
    { id: "c5", full_name: "Avery Editor", tenantId: "lumen", statuses: ["draft", "in_review", "scheduled"] },
    { id: "c8", full_name: "Leo Launch", tenantId: "lumen", statuses: ["published"] },
  ],
  articles: [
    { id: "a1", title: "Garden irrigation best practices", tenantId: "garden", categoryId: "product" },
    { id: "a2", title: "Ops playbook for remote teams", tenantId: "archive", categoryId: "ops" },
    { id: "a7", title: "Culture spotlights volume 1", tenantId: "archive", categoryId: "culture" },
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
      const method = (init?.method ?? "GET").toUpperCase();
      if (method === "POST") {
        const tenantId = url.searchParams.get("tenant_id") ?? "garden";
        const categoryId = url.searchParams.get("category_id") ?? undefined;
        const rawBody = typeof init?.body === "string" ? init.body : "";
        let payload: any = null;
        try {
          payload = rawBody ? JSON.parse(rawBody) : {};
        } catch (_err) {
          payload = {};
        }

        const label = String(payload?.label ?? payload?.name ?? payload?.query ?? "").trim();
        if (!label) {
          return new Response(JSON.stringify({ error: "label is required" }), {
            status: 400,
            headers: RESPONSE_HEADERS,
          });
        }

        const base = label
          .toLowerCase()
          .replace(/[^a-z0-9]+/g, "-")
          .replace(/(^-|-$)/g, "");

        let value = base || `tag-${Date.now()}`;
        let suffix = 1;
        while (mockData.tags.some((tag) => tag.value === value)) {
          suffix += 1;
          value = `${base || "tag"}-${suffix}`;
        }

        const record: TagRecord = { value, label, tenantId };
        if (categoryId) {
          record.categoryId = categoryId;
        }
        mockData.tags.unshift(record);

        return new Response(JSON.stringify({ value, label }), {
          status: 201,
          headers: RESPONSE_HEADERS,
        });
      }

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

    if (url.pathname.includes("/api/uploads")) {
      const now = Date.now();
      const imageUrl = `https://placehold.co/1200x800.png?text=Uploaded+${now}`;
      const thumbUrl = `https://placehold.co/600x400.png?text=Preview+${now}`;
      const responseBody = {
        url: imageUrl,
        name: `upload-${now}.png`,
        originalName: `upload-${now}.png`,
        size: 512000,
        contentType: "image/png",
        thumbnail: thumbUrl,
      };
      return new Response(JSON.stringify(responseBody), {
        status: 200,
        headers: RESPONSE_HEADERS,
      });
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
