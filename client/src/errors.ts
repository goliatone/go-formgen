export class ResolverError extends Error {
  readonly status?: number;
  readonly detail?: unknown;

  constructor(message: string, status?: number, detail?: unknown) {
    super(message);
    this.name = "ResolverError";
    this.status = status;
    this.detail = detail;
  }
}

export class ResolverAbortError extends Error {
  constructor() {
    super("Resolver request aborted");
    this.name = "ResolverAbortError";
  }
}
