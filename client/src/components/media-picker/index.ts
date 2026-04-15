import type { ComponentContext, ComponentFactory } from "../registry";
import { clearFieldError, renderFieldError } from "../../errors";

type MediaValueMode = "url" | "id";

interface MediaItem {
  id?: string;
  name?: string;
  url?: string;
  thumbnail?: string;
  type?: string;
  mime_type?: string;
  size?: number;
  status?: string;
  workflow_status?: string;
  workflow_error?: string;
  metadata?: Record<string, unknown>;
  created_at?: string;
}

interface MediaPage {
  items?: unknown[];
}

interface MediaOperationCapabilities {
  list?: boolean;
  get?: boolean;
  resolve?: boolean;
  upload?: boolean;
  presign?: boolean;
  confirm?: boolean;
}

interface MediaUploadCapabilities {
  direct_upload?: boolean;
  presign?: boolean;
  max_size?: number;
  accepted_kinds?: string[];
  accepted_mime_types?: string[];
}

interface MediaPickerCapabilities {
  value_modes?: MediaValueMode[];
  default_value_mode?: MediaValueMode;
}

interface MediaCapabilities {
  operations?: MediaOperationCapabilities;
  upload?: MediaUploadCapabilities;
  picker?: MediaPickerCapabilities;
}

interface MediaPickerConfig {
  variant?: string;
  multiple?: boolean;
  valueMode?: MediaValueMode;
  libraryPath?: string;
  itemEndpoint?: string;
  resolveEndpoint?: string;
  uploadEndpoint?: string;
  presignEndpoint?: string;
  confirmEndpoint?: string;
  capabilitiesEndpoint?: string;
  maxSize?: number;
  acceptedKinds?: string[] | string;
  accept?: string[] | string;
  headers?: Record<string, string>;
}

interface NormalizedMediaPickerConfig {
  variant: string;
  multiple: boolean;
  valueMode: MediaValueMode;
  libraryPath: string;
  itemEndpoint: string;
  resolveEndpoint: string;
  uploadEndpoint: string;
  presignEndpoint: string;
  confirmEndpoint: string;
  capabilitiesEndpoint: string;
  maxSize?: number;
  acceptedKinds: string[];
  accept: string[];
  headers: Record<string, string>;
}

interface PresignResponse {
  upload_url?: string;
  method?: string;
  headers?: Record<string, string>;
  fields?: Record<string, string>;
  upload_id?: string;
}

type UploadMode = "none" | "multipart" | "presign";

const DEFAULT_MAX_SIZE = 25 * 1024 * 1024;

export const mediaPickerFactory: ComponentFactory = ({ element, config }: ComponentContext) => {
  const input = element.querySelector<HTMLInputElement>("input, textarea, select");
  if (!input) {
    console.warn("formgen:media-picker - unable to find input element");
    return;
  }
  const picker = new MediaPicker(element, input, normalizeConfig(config as Record<string, unknown> | undefined));
  return () => picker.destroy();
};

class MediaPicker {
  private readonly element: HTMLElement;
  private readonly input: HTMLInputElement;
  private readonly config: NormalizedMediaPickerConfig;
  private readonly form: HTMLFormElement | null;
  private readonly originalName: string;
  private readonly hiddenContainer: HTMLElement;
  private readonly shell: HTMLElement;
  private readonly selectionRoot: HTMLElement;
  private readonly emptyState: HTMLElement;
  private readonly browseButton: HTMLButtonElement;
  private readonly uploadButton: HTMLButtonElement;
  private readonly clearButton: HTMLButtonElement;
  private readonly uploadInput: HTMLInputElement;
  private readonly statusMessage: HTMLElement;
  private readonly overlay: HTMLElement;
  private readonly modalSearch: HTMLInputElement;
  private readonly modalResults: HTMLElement;
  private readonly modalPreview: HTMLElement;
  private readonly modalName: HTMLElement;
  private readonly modalMeta: HTMLElement;
  private readonly modalError: HTMLElement;
  private readonly modalApply: HTMLButtonElement;
  private readonly modalClear: HTMLButtonElement;
  private readonly modalUpload: HTMLButtonElement;
  private readonly modalUploadInput: HTMLInputElement;
  private readonly modalClose: HTMLButtonElement;
  private selections: MediaItem[] = [];
  private candidateSelections: MediaItem[] = [];
  private focusedItem: MediaItem | null = null;
  private capabilities: MediaCapabilities = {};
  private effectiveValueMode: MediaValueMode;
  private uploadMode: UploadMode = "none";
  private libraryItems: MediaItem[] = [];
  private searchToken = 0;
  private destroyed = false;

  constructor(element: HTMLElement, input: HTMLInputElement, config: NormalizedMediaPickerConfig) {
    this.element = element;
    this.input = input;
    this.config = config;
    this.form = input.form;
    this.originalName = input.name;
    this.effectiveValueMode = config.valueMode;

    const hydratedValues = this.collectHydratedValues();

    this.input.type = "hidden";

    this.hiddenContainer = document.createElement("div");
    this.hiddenContainer.dataset.mediaPickerHidden = "true";
    this.hiddenContainer.style.display = "none";
    this.input.after(this.hiddenContainer);

    this.shell = this.resolveShell();
    this.selectionRoot = document.createElement("div");
    this.selectionRoot.dataset.mediaPickerSelections = "true";
    this.selectionRoot.className = "space-y-3";

    this.emptyState = document.createElement("p");
    this.emptyState.dataset.mediaPickerEmpty = "true";
    this.emptyState.className =
      "rounded-xl border border-dashed border-gray-300 bg-gray-50 px-4 py-6 text-sm text-gray-500";
    this.emptyState.textContent = this.config.multiple ? "No media selected." : "No media selected.";

    const actions = document.createElement("div");
    actions.className = "flex flex-wrap gap-3";

    this.browseButton = createButton("Browse Library", "secondary");
    this.browseButton.dataset.mediaPickerBrowse = "true";
    this.browseButton.addEventListener("click", this.handleBrowse);

    this.uploadButton = createButton(this.config.multiple ? "Upload Files" : "Upload File", "secondary");
    this.uploadButton.dataset.mediaPickerUpload = "true";
    this.uploadButton.addEventListener("click", this.handleUploadTrigger);

    this.clearButton = createButton("Clear", "secondary");
    this.clearButton.dataset.mediaPickerClear = "true";
    this.clearButton.addEventListener("click", this.handleClear);

    this.uploadInput = document.createElement("input");
    this.uploadInput.type = "file";
    this.uploadInput.className = "sr-only";
    this.uploadInput.multiple = this.config.multiple;
    this.uploadInput.addEventListener("change", this.handleInlineUpload);

    this.statusMessage = document.createElement("p");
    this.statusMessage.dataset.mediaPickerStatus = "true";
    this.statusMessage.className = "text-sm text-gray-500";
    this.statusMessage.setAttribute("aria-live", "polite");

    actions.append(this.browseButton, this.uploadButton, this.clearButton, this.uploadInput);
    this.shell.replaceChildren(this.selectionRoot, this.emptyState, actions, this.statusMessage);

    const modal = buildModal();
    this.overlay = modal.overlay;
    this.modalSearch = modal.search;
    this.modalResults = modal.results;
    this.modalPreview = modal.preview;
    this.modalName = modal.name;
    this.modalMeta = modal.meta;
    this.modalError = modal.error;
    this.modalApply = modal.apply;
    this.modalClear = modal.clear;
    this.modalUpload = modal.upload;
    this.modalUploadInput = modal.uploadInput;
    this.modalClose = modal.close;

    this.modalClose.addEventListener("click", this.closeModal);
    this.modalApply.addEventListener("click", this.handleApplySelection);
    this.modalClear.addEventListener("click", this.handleModalClear);
    this.modalUpload.addEventListener("click", this.handleModalUploadTrigger);
    this.modalUploadInput.addEventListener("change", this.handleModalUpload);
    this.modalSearch.addEventListener("input", this.handleModalSearch);
    this.overlay.addEventListener("click", this.handleOverlayClick);

    document.body.appendChild(this.overlay);

    this.renderSelections();
    void this.bootstrap(hydratedValues);
  }

  destroy(): void {
    this.destroyed = true;
    this.browseButton.removeEventListener("click", this.handleBrowse);
    this.uploadButton.removeEventListener("click", this.handleUploadTrigger);
    this.clearButton.removeEventListener("click", this.handleClear);
    this.uploadInput.removeEventListener("change", this.handleInlineUpload);
    this.modalClose.removeEventListener("click", this.closeModal);
    this.modalApply.removeEventListener("click", this.handleApplySelection);
    this.modalClear.removeEventListener("click", this.handleModalClear);
    this.modalUpload.removeEventListener("click", this.handleModalUploadTrigger);
    this.modalUploadInput.removeEventListener("change", this.handleModalUpload);
    this.modalSearch.removeEventListener("input", this.handleModalSearch);
    this.overlay.removeEventListener("click", this.handleOverlayClick);
    this.overlay.remove();
  }

  private async bootstrap(initialValues: string[]): Promise<void> {
    this.setStatus("Loading media configuration...");
    try {
      this.capabilities = await this.loadCapabilities();
      this.effectiveValueMode = resolveValueMode(this.config.valueMode, this.capabilities.picker);
      this.uploadMode = resolveUploadMode(this.config, this.capabilities);
      this.configureUploadInputs();
      this.updateCapabilityAffordances();

      if (initialValues.length > 0) {
        this.selections = (await Promise.all(initialValues.map((value) => this.resolveValue(value)))).filter(
          (item): item is MediaItem => item !== null,
        );
      }

      this.serializeSelections();
      this.renderSelections();
      this.setStatus("");
    } catch (error) {
      this.setStatus(error instanceof Error ? error.message : "Unable to load media picker.");
    }
  }

  private resolveShell(): HTMLElement {
    const existing = this.element.querySelector<HTMLElement>("[data-media-picker-root]");
    if (existing) {
      existing.className = "space-y-3";
      return existing;
    }
    const shell = document.createElement("div");
    shell.dataset.mediaPickerRoot = "true";
    shell.className = "space-y-3";
    this.element.appendChild(shell);
    return shell;
  }

  private configureUploadInputs(): void {
    const accepted = this.resolvedAcceptedTypes();
    const acceptValue = accepted.join(",");
    this.uploadInput.accept = acceptValue;
    this.modalUploadInput.accept = acceptValue;
  }

  private updateCapabilityAffordances(): void {
    const canUpload = this.uploadMode !== "none";
    this.uploadButton.hidden = !canUpload;
    this.modalUpload.hidden = !canUpload;
    this.clearButton.disabled = this.selections.length === 0;
  }

  private async loadCapabilities(): Promise<MediaCapabilities> {
    const fallback = defaultCapabilities(this.config);
    if (!this.config.capabilitiesEndpoint) {
      return fallback;
    }
    try {
      const payload = (await requestJSON(this.config.capabilitiesEndpoint, {
        headers: buildRequestHeaders(this.config.headers, this.element),
      })) as MediaCapabilities;
      return normalizeCapabilities(payload, fallback);
    } catch (error) {
      console.warn("formgen:media-picker - failed to load capabilities", error);
      return fallback;
    }
  }

  private collectHydratedValues(): string[] {
    const normalize = (value: string) => value.trim();

    if (!this.config.multiple) {
      const value = normalize(this.input.value ?? "");
      return value ? [value] : [];
    }

    const logicalName = this.originalName.endsWith("[]") ? this.originalName.slice(0, -2) : this.originalName;
    const multipleName = `${logicalName}[]`;
    const selector = [
      `input[name="${cssEscapeAttributeValue(this.originalName)}"]`,
      `input[name="${cssEscapeAttributeValue(multipleName)}"]`,
    ].join(", ");

    const candidates = Array.from(this.element.querySelectorAll<HTMLInputElement>(selector));
    const values = candidates.map((candidate) => normalize(candidate.value ?? "")).filter(Boolean);

    for (const candidate of candidates) {
      if (candidate !== this.input) {
        candidate.remove();
      }
    }
    return values;
  }

  private async resolveValue(value: string): Promise<MediaItem | null> {
    const trimmed = value.trim();
    if (!trimmed) {
      return null;
    }

    try {
      if (this.effectiveValueMode === "id") {
        if (this.config.itemEndpoint) {
          const endpoint = replacePathToken(this.config.itemEndpoint, "id", trimmed);
          return normalizeMediaItem(await requestJSON(endpoint, { headers: buildRequestHeaders(this.config.headers, this.element) }));
        }
        if (this.config.resolveEndpoint) {
          return normalizeMediaItem(
            await requestJSON(this.config.resolveEndpoint, {
              method: "POST",
              headers: jsonHeaders(this.config.headers, this.element),
              body: JSON.stringify({ id: trimmed }),
            }),
          );
        }
      } else if (this.config.resolveEndpoint) {
        return normalizeMediaItem(
          await requestJSON(this.config.resolveEndpoint, {
            method: "POST",
            headers: jsonHeaders(this.config.headers, this.element),
            body: JSON.stringify({ url: trimmed }),
          }),
        );
      }
    } catch (error) {
      console.warn("formgen:media-picker - hydrate failed", error);
    }

    return fallbackMediaItem(trimmed, this.effectiveValueMode);
  }

  private renderSelections(): void {
    this.selectionRoot.innerHTML = "";
    this.emptyState.hidden = this.selections.length > 0;
    this.clearButton.disabled = this.selections.length === 0;

    if (this.selections.length === 0) {
      return;
    }

    const items = this.config.multiple ? this.selections : this.selections.slice(0, 1);
    for (const item of items) {
      this.selectionRoot.appendChild(this.renderSelectionCard(item));
    }
  }

  private renderSelectionCard(item: MediaItem): HTMLElement {
    const card = document.createElement("div");
    card.dataset.mediaPickerSelection = "true";
    card.className = "flex items-center gap-3 rounded-xl border border-gray-200 bg-white p-3 shadow-sm";

    const thumb = document.createElement("div");
    thumb.className =
      "flex h-16 w-16 shrink-0 items-center justify-center overflow-hidden rounded-lg border border-gray-200 bg-gray-50";
    thumb.appendChild(renderMediaPreview(item, "h-full w-full object-cover"));

    const text = document.createElement("div");
    text.className = "min-w-0 flex-1";

    const name = document.createElement("div");
    name.className = "truncate text-sm font-medium text-gray-900";
    name.textContent = item.name || item.url || item.id || "Media";

    const meta = document.createElement("div");
    meta.className = "mt-1 text-xs text-gray-500";
    meta.textContent = describeMedia(item, this.effectiveValueMode);

    text.append(name, meta);

    const remove = document.createElement("button");
    remove.type = "button";
    remove.dataset.mediaPickerRemove = serializeSelectionValue(item, this.effectiveValueMode);
    remove.className =
      "inline-flex h-9 w-9 items-center justify-center rounded-md border border-slate-200 bg-white text-slate-500 shadow-sm transition hover:text-rose-500 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2";
    remove.textContent = "×";
    remove.addEventListener("click", () => this.removeSelection(item));

    card.append(thumb, text, remove);
    return card;
  }

  private removeSelection(item: MediaItem): void {
    const target = serializeSelectionValue(item, this.effectiveValueMode);
    this.selections = this.selections.filter(
      (entry) => serializeSelectionValue(entry, this.effectiveValueMode) !== target,
    );
    this.serializeSelections();
    this.renderSelections();
    clearFieldError(this.element);
  }

  private serializeSelections(): void {
    this.hiddenContainer.innerHTML = "";

    const values = this.selections
      .map((item) => serializeSelectionValue(item, this.effectiveValueMode))
      .filter(Boolean);

    if (values.length === 0) {
      this.input.name = this.originalName;
      this.input.value = "";
      return;
    }

    if (!this.config.multiple) {
      this.input.name = this.originalName;
      this.input.value = values[0] ?? "";
      return;
    }

    const baseName = this.originalName.endsWith("[]") ? this.originalName : `${this.originalName}[]`;
    this.input.name = baseName;
    this.input.value = values[0] ?? "";
    for (let index = 1; index < values.length; index += 1) {
      const hidden = document.createElement("input");
      hidden.type = "hidden";
      hidden.name = baseName;
      hidden.value = values[index] ?? "";
      this.hiddenContainer.appendChild(hidden);
    }
  }

  private setStatus(message: string): void {
    this.statusMessage.textContent = message;
  }

  private showError(message: string): void {
    this.setStatus(message);
    renderFieldError(this.element, message);
  }

  private openModal = async () => {
    this.candidateSelections = this.selections.slice();
    this.focusedItem = this.candidateSelections[0] ?? null;
    this.modalError.textContent = "";
    this.modalError.hidden = true;
    this.modalSearch.value = "";
    this.overlay.hidden = false;
    this.overlay.classList.add("flex");
    await this.loadLibrary("");
    this.renderModal();
  };

  private closeModal = () => {
    this.overlay.hidden = true;
    this.overlay.classList.remove("flex");
    this.modalUploadInput.value = "";
  };

  private handleBrowse = () => {
    void this.openModal();
  };

  private handleUploadTrigger = () => {
    if (this.uploadMode === "none") {
      return;
    }
    this.uploadInput.click();
  };

  private handleInlineUpload = async () => {
    const files = this.uploadInput.files;
    this.uploadInput.value = "";
    if (!files || files.length === 0) {
      return;
    }
    try {
      await this.uploadFiles(files);
    } catch (error) {
      this.showError(error instanceof Error ? error.message : "Upload failed.");
    }
  };

  private handleClear = () => {
    this.selections = [];
    this.serializeSelections();
    this.renderSelections();
    this.setStatus("");
    clearFieldError(this.element);
  };

  private handleOverlayClick = (event: Event) => {
    if (event.target === this.overlay) {
      this.closeModal();
    }
  };

  private handleApplySelection = () => {
    this.selections = dedupeSelections(this.candidateSelections, this.effectiveValueMode);
    if (!this.config.multiple && this.selections.length > 1) {
      this.selections = this.selections.slice(0, 1);
    }
    this.serializeSelections();
    this.renderSelections();
    this.setStatus("");
    clearFieldError(this.element);
    this.closeModal();
  };

  private handleModalClear = () => {
    this.candidateSelections = [];
    this.focusedItem = null;
    this.renderModal();
  };

  private handleModalUploadTrigger = () => {
    if (this.uploadMode === "none") {
      return;
    }
    this.modalUploadInput.click();
  };

  private handleModalUpload = async () => {
    const files = this.modalUploadInput.files;
    this.modalUploadInput.value = "";
    if (!files || files.length === 0) {
      return;
    }
    try {
      const uploaded = await this.uploadFiles(files);
      if (!this.config.multiple && uploaded.length > 0) {
        this.focusedItem = uploaded[0] ?? null;
      }
      await this.loadLibrary(this.modalSearch.value);
      this.renderModal();
    } catch (error) {
      this.modalError.textContent = error instanceof Error ? error.message : "Upload failed.";
      this.modalError.hidden = false;
    }
  };

  private handleModalSearch = () => {
    void this.loadLibrary(this.modalSearch.value).then(() => this.renderModal());
  };

  private async uploadFiles(list: FileList): Promise<MediaItem[]> {
    if (this.uploadMode === "none") {
      throw new Error("Uploads are not available for this picker.");
    }
    const files = Array.from(list);
    const accepted = this.config.multiple ? files : files.slice(0, 1);
    const uploaded: MediaItem[] = [];
    for (const file of accepted) {
      const validation = this.validateFile(file);
      if (validation) {
        throw new Error(validation);
      }
      const item = await this.uploadFile(file);
      uploaded.push(item);
    }

    if (this.config.multiple) {
      this.selections = dedupeSelections([...this.selections, ...uploaded], this.effectiveValueMode);
    } else {
      this.selections = uploaded.slice(0, 1);
    }
    this.serializeSelections();
    this.renderSelections();
    clearFieldError(this.element);
    return uploaded;
  }

  private validateFile(file: File): string | null {
    const maxSize = this.capabilities.upload?.max_size ?? this.config.maxSize ?? DEFAULT_MAX_SIZE;
    if (maxSize > 0 && file.size > maxSize) {
      return `File "${file.name}" exceeds maximum size of ${formatBytes(maxSize)}.`;
    }

    const acceptedTypes = this.resolvedAcceptedTypes();
    if (acceptedTypes.length > 0 && !acceptedTypes.some((entry) => matchesType(file, entry))) {
      return `File "${file.name}" is not an allowed type.`;
    }

    const acceptedKinds = this.resolvedAcceptedKinds();
    if (acceptedKinds.length > 0 && !acceptedKinds.some((kind) => matchesKind(file, kind))) {
      return `File "${file.name}" is not an allowed media kind.`;
    }
    return null;
  }

  private resolvedAcceptedTypes(): string[] {
    const configAccept = this.config.accept;
    const capsAccept = this.capabilities.upload?.accepted_mime_types ?? [];
    return configAccept.length > 0 ? configAccept : capsAccept.filter(Boolean);
  }

  private resolvedAcceptedKinds(): string[] {
    const capsKinds = this.capabilities.upload?.accepted_kinds ?? [];
    return this.config.acceptedKinds.length > 0 ? this.config.acceptedKinds : capsKinds.filter(Boolean);
  }

  private async uploadFile(file: File): Promise<MediaItem> {
    this.setStatus(`Uploading ${file.name}...`);

    if (this.uploadMode === "presign") {
      const presign = normalizePresignResponse(
        (await requestJSON(this.config.presignEndpoint, {
          method: "POST",
          headers: jsonHeaders(this.config.headers, this.element),
          body: JSON.stringify({
            name: file.name,
            file_name: file.name,
            content_type: file.type || "application/octet-stream",
            size: file.size,
          }),
        })) as PresignResponse,
      );
      await performPresignedUpload(file, presign);
      const confirmed = await requestJSON(this.config.confirmEndpoint, {
        method: "POST",
        headers: jsonHeaders(this.config.headers, this.element),
        body: JSON.stringify({
          upload_id: presign.upload_id,
          name: file.name,
          file_name: file.name,
          content_type: file.type || "application/octet-stream",
          size: file.size,
        }),
      });
      this.setStatus("");
      return normalizeMediaItem(confirmed);
    }

    const formData = new FormData();
    formData.append("file", file, file.name);
    const uploaded = await requestJSON(this.config.uploadEndpoint, {
      method: "POST",
      headers: buildRequestHeaders(this.config.headers, this.element),
      body: formData,
    });
    this.setStatus("");
    return normalizeMediaItem(uploaded);
  }

  private async loadLibrary(search: string): Promise<void> {
    if (!this.config.libraryPath) {
      this.libraryItems = [];
      return;
    }
    const token = ++this.searchToken;
    const url = new URL(this.config.libraryPath, window.location.origin);
    if (search.trim()) {
      url.searchParams.set("search", search.trim());
    }
    url.searchParams.set("limit", "48");
    const acceptedKinds = this.resolvedAcceptedKinds();
    if (acceptedKinds.length === 1) {
      url.searchParams.set("type", acceptedKinds[0] ?? "");
    }
    const payload = (await requestJSON(url.toString(), {
      headers: buildRequestHeaders(this.config.headers, this.element),
    })) as MediaPage | MediaItem[];
    if (token !== this.searchToken) {
      return;
    }
    const rawItems = Array.isArray(payload) ? payload : Array.isArray(payload?.items) ? payload.items : [];
    this.libraryItems = rawItems.map((item) => normalizeMediaItem(item)).filter((item) => this.matchesLibraryFilters(item));
  }

  private matchesLibraryFilters(item: MediaItem): boolean {
    const acceptedKinds = this.resolvedAcceptedKinds();
    if (acceptedKinds.length === 0) {
      return true;
    }
    if (!item.type) {
      return true;
    }
    return acceptedKinds.includes(item.type);
  }

  private renderModal(): void {
    this.modalResults.innerHTML = "";
    const items = this.libraryItems;

    if (items.length === 0) {
      const empty = document.createElement("p");
      empty.dataset.mediaPickerModalEmpty = "true";
      empty.className =
        "col-span-full rounded-xl border border-dashed border-gray-300 px-4 py-10 text-center text-sm text-gray-500";
      empty.textContent = "No media matched this filter.";
      this.modalResults.appendChild(empty);
    }

    for (const item of items) {
      const value = serializeSelectionValue(item, this.effectiveValueMode);
      const selected = this.candidateSelections.some(
        (entry) => serializeSelectionValue(entry, this.effectiveValueMode) === value,
      );
      const button = document.createElement("button");
      button.type = "button";
      button.dataset.mediaPickerOption = value;
      button.className = [
        "overflow-hidden rounded-xl border bg-white text-left shadow-sm transition",
        selected ? "border-blue-500 ring-2 ring-blue-500/30" : "border-gray-200 hover:border-gray-400",
      ].join(" ");
      button.addEventListener("click", () => {
        this.focusedItem = item;
        if (this.config.multiple) {
          this.toggleCandidateSelection(item);
        } else {
          this.candidateSelections = [item];
        }
        this.renderModal();
      });

      const thumb = document.createElement("div");
      thumb.className = "h-28 bg-gray-50";
      thumb.appendChild(renderMediaPreview(item, "h-full w-full object-cover"));

      const copy = document.createElement("div");
      copy.className = "space-y-1 p-3";
      const title = document.createElement("div");
      title.className = "truncate text-sm font-medium text-gray-900";
      title.textContent = item.name || item.url || item.id || "Media";
      const meta = document.createElement("div");
      meta.className = "truncate text-xs text-gray-500";
      meta.textContent = describeMedia(item, this.effectiveValueMode);
      copy.append(title, meta);

      button.append(thumb, copy);
      this.modalResults.appendChild(button);
    }

    const focus = this.focusedItem ?? this.candidateSelections[0] ?? null;
    this.modalPreview.replaceChildren(renderMediaPreview(focus, "h-full w-full object-cover"));
    this.modalName.textContent = focus?.name || focus?.url || focus?.id || "No media selected";
    this.modalMeta.textContent = focus ? describeMedia(focus, this.effectiveValueMode) : "";
    this.modalApply.textContent = this.config.multiple ? "Use Selection" : "Select";
    this.modalClear.disabled = this.candidateSelections.length === 0;
  }

  private toggleCandidateSelection(item: MediaItem): void {
    const value = serializeSelectionValue(item, this.effectiveValueMode);
    const index = this.candidateSelections.findIndex(
      (entry) => serializeSelectionValue(entry, this.effectiveValueMode) === value,
    );
    if (index >= 0) {
      this.candidateSelections = [
        ...this.candidateSelections.slice(0, index),
        ...this.candidateSelections.slice(index + 1),
      ];
      return;
    }
    this.candidateSelections = dedupeSelections([...this.candidateSelections, item], this.effectiveValueMode);
  }
}

function buildModal(): {
  overlay: HTMLElement;
  search: HTMLInputElement;
  results: HTMLElement;
  preview: HTMLElement;
  name: HTMLElement;
  meta: HTMLElement;
  error: HTMLElement;
  apply: HTMLButtonElement;
  clear: HTMLButtonElement;
  upload: HTMLButtonElement;
  uploadInput: HTMLInputElement;
  close: HTMLButtonElement;
} {
  const overlay = document.createElement("div");
  overlay.dataset.mediaPickerModal = "true";
  overlay.hidden = true;
  overlay.className = "fixed inset-0 z-[80] hidden items-center justify-center bg-black/50 p-6";

  const panel = document.createElement("div");
  panel.className =
    "grid h-[min(80vh,760px)] w-[min(1100px,95vw)] grid-cols-[minmax(0,1fr)_320px] overflow-hidden rounded-2xl bg-white shadow-2xl";

  const left = document.createElement("div");
  left.className = "flex min-h-0 flex-col";
  const toolbar = document.createElement("div");
  toolbar.className = "border-b border-gray-200 p-4";
  const search = document.createElement("input");
  search.dataset.mediaPickerModalSearch = "true";
  search.type = "search";
  search.placeholder = "Search media";
  search.className = "w-full rounded-lg border border-gray-300 px-4 py-3 text-sm";
  toolbar.appendChild(search);

  const results = document.createElement("div");
  results.dataset.mediaPickerModalResults = "true";
  results.className = "grid flex-1 grid-cols-[repeat(auto-fill,minmax(160px,1fr))] gap-3 overflow-y-auto p-4";
  left.append(toolbar, results);

  const right = document.createElement("div");
  right.className = "border-l border-gray-200 p-4";

  const preview = document.createElement("div");
  preview.dataset.mediaPickerModalPreview = "true";
  preview.className =
    "mb-4 flex min-h-[220px] items-center justify-center overflow-hidden rounded-2xl border border-gray-200 bg-gray-50";

  const name = document.createElement("h3");
  name.dataset.mediaPickerModalName = "true";
  name.className = "text-lg font-semibold text-gray-900";
  name.textContent = "No media selected";

  const meta = document.createElement("p");
  meta.dataset.mediaPickerModalMeta = "true";
  meta.className = "mt-1 text-sm text-gray-500";

  const actions = document.createElement("div");
  actions.className = "mt-4 flex flex-wrap gap-3";

  const apply = createButton("Select", "primary");
  apply.dataset.mediaPickerModalApply = "true";

  const clear = createButton("Clear", "secondary");
  clear.dataset.mediaPickerModalClear = "true";

  const upload = createButton("Upload", "secondary");
  upload.dataset.mediaPickerModalUpload = "true";

  const close = createButton("Close", "secondary");
  close.dataset.mediaPickerModalClose = "true";

  const uploadInput = document.createElement("input");
  uploadInput.dataset.mediaPickerModalUploadInput = "true";
  uploadInput.type = "file";
  uploadInput.className = "sr-only";

  const error = document.createElement("p");
  error.dataset.mediaPickerModalError = "true";
  error.hidden = true;
  error.className = "mt-4 text-sm text-red-600";

  actions.append(apply, clear, upload, close, uploadInput);
  right.append(preview, name, meta, actions, error);

  panel.append(left, right);
  overlay.appendChild(panel);

  return { overlay, search, results, preview, name, meta, error, apply, clear, upload, uploadInput, close };
}

function createButton(label: string, tone: "primary" | "secondary"): HTMLButtonElement {
  const button = document.createElement("button");
  button.type = "button";
  button.textContent = label;
  button.className =
    tone === "primary"
      ? "inline-flex items-center justify-center rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white transition hover:bg-blue-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2"
      : "inline-flex items-center justify-center rounded-lg border border-gray-200 bg-white px-4 py-2 text-sm font-medium text-gray-700 transition hover:border-gray-300 hover:bg-gray-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2";
  return button;
}

function normalizeConfig(raw?: Record<string, unknown>): NormalizedMediaPickerConfig {
  const config = (raw ?? {}) as MediaPickerConfig;
  const toBool = (value: unknown, fallback = false) => {
    if (typeof value === "boolean") {
      return value;
    }
    if (typeof value === "string") {
      return value.toLowerCase() === "true";
    }
    return fallback;
  };
  const toNumber = (value: unknown): number | undefined => {
    if (typeof value === "number" && Number.isFinite(value)) {
      return value;
    }
    if (typeof value === "string" && value.trim() !== "") {
      const parsed = Number(value);
      return Number.isFinite(parsed) ? parsed : undefined;
    }
    return undefined;
  };
  const headers: Record<string, string> = {};
  if (config.headers && typeof config.headers === "object") {
    Object.entries(config.headers).forEach(([key, value]) => {
      if (value == null) {
        return;
      }
      headers[key] = String(value);
    });
  }
  return {
    variant: typeof config.variant === "string" && config.variant.trim() ? config.variant.trim() : "media-picker",
    multiple: toBool(config.multiple, false),
    valueMode: config.valueMode === "id" ? "id" : "url",
    libraryPath: stringValue(config.libraryPath),
    itemEndpoint: stringValue(config.itemEndpoint),
    resolveEndpoint: stringValue(config.resolveEndpoint),
    uploadEndpoint: stringValue(config.uploadEndpoint),
    presignEndpoint: stringValue(config.presignEndpoint),
    confirmEndpoint: stringValue(config.confirmEndpoint),
    capabilitiesEndpoint: stringValue(config.capabilitiesEndpoint),
    maxSize: toNumber(config.maxSize),
    acceptedKinds: normalizeList(config.acceptedKinds),
    accept: normalizeList(config.accept),
    headers,
  };
}

function stringValue(value: unknown): string {
  return typeof value === "string" ? value.trim() : "";
}

function normalizeList(value: unknown): string[] {
  if (Array.isArray(value)) {
    return value.filter((entry): entry is string => typeof entry === "string").map((entry) => entry.trim()).filter(Boolean);
  }
  if (typeof value === "string") {
    return value
      .split(",")
      .map((entry) => entry.trim())
      .filter(Boolean);
  }
  return [];
}

function defaultCapabilities(config: NormalizedMediaPickerConfig): MediaCapabilities {
  const supportsID = Boolean(config.itemEndpoint || config.resolveEndpoint);
  return {
    operations: {
      list: Boolean(config.libraryPath),
      get: Boolean(config.itemEndpoint),
      resolve: Boolean(config.resolveEndpoint),
      upload: Boolean(config.uploadEndpoint),
      presign: Boolean(config.presignEndpoint),
      confirm: Boolean(config.confirmEndpoint),
    },
    upload: {
      direct_upload: Boolean(config.uploadEndpoint),
      presign: Boolean(config.presignEndpoint && config.confirmEndpoint),
      max_size: config.maxSize,
      accepted_kinds: config.acceptedKinds,
      accepted_mime_types: config.accept,
    },
    picker: {
      value_modes: supportsID ? ["url", "id"] : ["url"],
      default_value_mode: config.valueMode === "id" && supportsID ? "id" : "url",
    },
  };
}

function normalizeCapabilities(payload: MediaCapabilities, fallback: MediaCapabilities): MediaCapabilities {
  return {
    operations: {
      ...fallback.operations,
      ...(payload.operations ?? {}),
    },
    upload: {
      ...fallback.upload,
      ...(payload.upload ?? {}),
    },
    picker: {
      ...fallback.picker,
      ...(payload.picker ?? {}),
      value_modes: normalizeValueModes(payload.picker?.value_modes, fallback.picker?.value_modes),
    },
  };
}

function normalizeValueModes(value: MediaValueMode[] | undefined, fallback: MediaValueMode[] | undefined): MediaValueMode[] {
  const source = Array.isArray(value) && value.length > 0 ? value : fallback ?? ["url"];
  const out = source.filter((entry): entry is MediaValueMode => entry === "url" || entry === "id");
  return out.length > 0 ? Array.from(new Set(out)) : ["url"];
}

function resolveValueMode(requested: MediaValueMode, picker?: MediaPickerCapabilities): MediaValueMode {
  const supported = normalizeValueModes(picker?.value_modes, ["url"]);
  if (supported.includes(requested)) {
    return requested;
  }
  if (picker?.default_value_mode && supported.includes(picker.default_value_mode)) {
    return picker.default_value_mode;
  }
  return supported.includes("url") ? "url" : supported[0] ?? "url";
}

function resolveUploadMode(config: NormalizedMediaPickerConfig, capabilities: MediaCapabilities): UploadMode {
  const operations = capabilities.operations ?? {};
  const upload = capabilities.upload ?? {};
  if (upload.presign && operations.presign && operations.confirm && config.presignEndpoint && config.confirmEndpoint) {
    return "presign";
  }
  if (upload.direct_upload && operations.upload && config.uploadEndpoint) {
    return "multipart";
  }
  return "none";
}

function normalizeMediaItem(value: unknown): MediaItem {
  const item = (value && typeof value === "object" ? value : {}) as Record<string, unknown>;
  const metadata =
    item.metadata && typeof item.metadata === "object" ? (item.metadata as Record<string, unknown>) : undefined;
  return {
    id: stringValue(item.id),
    name: stringValue(item.name) || stringValue(item.filename),
    url: stringValue(item.url),
    thumbnail: stringValue(item.thumbnail) || stringValue(item.thumbnail_url) || stringValue(metadata?.thumbnail_url),
    type: stringValue(item.type) || stringValue(metadata?.type),
    mime_type: stringValue(item.mime_type) || stringValue(item.content_type) || stringValue(metadata?.mime_type),
    size: typeof item.size === "number" ? item.size : undefined,
    status: stringValue(item.status),
    workflow_status: stringValue(item.workflow_status),
    workflow_error: stringValue(item.workflow_error),
    metadata,
    created_at: stringValue(item.created_at),
  };
}

function fallbackMediaItem(value: string, mode: MediaValueMode): MediaItem {
  if (mode === "id") {
    return {
      id: value,
      name: value,
    };
  }
  return {
    url: value,
    name: guessFileName(value),
    thumbnail: looksLikeImageURL(value) ? value : "",
    type: looksLikeImageURL(value) ? "image" : "",
  };
}

function serializeSelectionValue(item: MediaItem, mode: MediaValueMode): string {
  return mode === "id" ? item.id?.trim() ?? "" : item.url?.trim() ?? "";
}

function dedupeSelections(items: MediaItem[], mode: MediaValueMode): MediaItem[] {
  const seen = new Set<string>();
  const out: MediaItem[] = [];
  for (const item of items) {
    const key = serializeSelectionValue(item, mode);
    if (!key || seen.has(key)) {
      continue;
    }
    seen.add(key);
    out.push(item);
  }
  return out;
}

function renderMediaPreview(item: MediaItem | null, imageClass: string): HTMLElement {
  if (item?.type === "image" || looksLikeImageURL(item?.thumbnail || item?.url || "")) {
    const img = document.createElement("img");
    img.className = imageClass;
    img.src = item?.thumbnail || item?.url || "";
    img.alt = item?.name || "Media preview";
    return img;
  }
  const fallback = document.createElement("div");
  fallback.className = "flex h-full w-full items-center justify-center bg-gray-100 text-sm text-gray-500";
  fallback.textContent = item?.type || "Media";
  return fallback;
}

function describeMedia(item: MediaItem, mode: MediaValueMode): string {
  const parts = [item.type, item.mime_type, mode === "id" ? item.id : item.url].filter(Boolean);
  return parts.join(" · ");
}

function replacePathToken(pathname: string, token: string, value: string): string {
  const escaped = encodeURIComponent(value);
  return pathname.replace(`:${token}`, escaped);
}

async function performPresignedUpload(file: File, presign: PresignResponse): Promise<void> {
  if (!presign.upload_url || !presign.upload_id) {
    throw new Error("Upload session initialization failed.");
  }
  const method = (presign.method || "PUT").toUpperCase();
  const directHeaders = { ...(presign.headers ?? {}) };

  if (method === "POST" && presign.fields && Object.keys(presign.fields).length > 0) {
    const formData = new FormData();
    for (const [key, value] of Object.entries(presign.fields)) {
      formData.append(key, value);
    }
    formData.append("file", file, file.name);
    const response = await fetch(presign.upload_url, {
      method,
      body: formData,
    });
    if (!response.ok) {
      throw new Error(`Upload failed (${response.status})`);
    }
    return;
  }

  const response = await fetch(presign.upload_url, {
    method,
    headers: directHeaders,
    body: file,
  });
  if (!response.ok) {
    throw new Error(`Upload failed (${response.status})`);
  }
}

function normalizePresignResponse(value: PresignResponse): PresignResponse {
  return {
    upload_url: value.upload_url?.trim(),
    method: value.method?.trim() || "PUT",
    headers:
      value.headers && typeof value.headers === "object"
        ? Object.fromEntries(Object.entries(value.headers).map(([key, entry]) => [key, String(entry)]))
        : undefined,
    fields:
      value.fields && typeof value.fields === "object"
        ? Object.fromEntries(Object.entries(value.fields).map(([key, entry]) => [key, String(entry)]))
        : undefined,
    upload_id: value.upload_id?.trim(),
  };
}

async function requestJSON(
  url: string,
  options: RequestInit & { headers?: Record<string, string> } = {},
): Promise<unknown> {
  const response = await fetch(url, {
    credentials: "same-origin",
    headers: { Accept: "application/json", ...(options.headers ?? {}) },
    ...options,
  });
  const contentType = String(response.headers.get("content-type") || "").toLowerCase();
  const expectsJSON = contentType.includes("application/json") || contentType.includes("+json");
  if (!expectsJSON) {
    if (!response.ok) {
      throw new Error(`Request failed (${response.status})`);
    }
    throw new Error("Unexpected server response: expected JSON.");
  }
  const payload = await response.json().catch(() => null);
  if (!response.ok) {
    const message = readErrorMessage(payload) || `Request failed (${response.status})`;
    throw new Error(message);
  }
  if (payload == null) {
    throw new Error("Unexpected server response: expected JSON.");
  }
  return payload;
}

function jsonHeaders(headers: Record<string, string>, element: HTMLElement): Record<string, string> {
  return {
    "Content-Type": "application/json",
    ...buildRequestHeaders(headers, element),
  };
}

function buildRequestHeaders(headers: Record<string, string>, element: HTMLElement): Record<string, string> {
  const resolved: Record<string, string> = {};
  for (const [key, value] of Object.entries(headers ?? {})) {
    const actual = resolveHeaderValue(value, element);
    if (actual) {
      resolved[key] = actual;
    }
  }
  return resolved;
}

function resolveHeaderValue(value: string, element: HTMLElement): string | undefined {
  if (!value) {
    return undefined;
  }
  if (value.startsWith("meta:")) {
    const meta = document.querySelector(`meta[name="${value.slice(5)}"]`);
    return meta?.getAttribute("content") ?? undefined;
  }
  if (value.startsWith("data:")) {
    const attr = value.slice(5);
    if (element.hasAttribute(attr)) {
      return element.getAttribute(attr) ?? undefined;
    }
    const normalized = dataAttributeToDatasetKey(attr);
    return (element.dataset as Record<string, string | undefined>)[normalized];
  }
  return value;
}

function dataAttributeToDatasetKey(attribute: string): string {
  return attribute
    .replace(/^data-/, "")
    .split(/[-_:]/)
    .filter(Boolean)
    .map((part, index) =>
      index === 0 ? part.toLowerCase() : part.charAt(0).toUpperCase() + part.slice(1).toLowerCase(),
    )
    .join("");
}

function readErrorMessage(value: unknown): string {
  if (!value) {
    return "";
  }
  if (typeof value === "string") {
    return value.trim();
  }
  if (Array.isArray(value)) {
    for (const entry of value) {
      const message = readErrorMessage(entry);
      if (message) {
        return message;
      }
    }
    return "";
  }
  if (typeof value !== "object") {
    return "";
  }
  const object = value as Record<string, unknown>;
  for (const key of ["error", "message", "detail", "reason", "description"]) {
    const message = readErrorMessage(object[key]);
    if (message) {
      return message;
    }
  }
  return "";
}

function matchesType(file: File, allowed: string): boolean {
  if (!allowed) {
    return true;
  }
  if (allowed.startsWith(".")) {
    return file.name.toLowerCase().endsWith(allowed.toLowerCase());
  }
  if (allowed.endsWith("/*")) {
    const base = allowed.split("/")[0];
    return file.type.startsWith(`${base}/`);
  }
  return file.type === allowed;
}

function matchesKind(file: File, kind: string): boolean {
  const normalized = kind.trim().toLowerCase();
  if (!normalized) {
    return true;
  }
  switch (normalized) {
    case "image":
      return file.type.startsWith("image/");
    case "audio":
      return file.type.startsWith("audio/");
    case "video":
      return file.type.startsWith("video/");
    case "document":
      return /pdf|msword|officedocument|text\//.test(file.type);
    default:
      return file.type.startsWith(`${normalized}/`);
  }
}

function cssEscapeAttributeValue(value: string): string {
  if (
    typeof globalThis !== "undefined" &&
    typeof (globalThis as unknown as { CSS?: { escape?: (input: string) => string } }).CSS?.escape === "function"
  ) {
    return (globalThis as unknown as { CSS: { escape: (input: string) => string } }).CSS.escape(value);
  }
  return value.replace(/["\\]/g, "\\$1");
}

function looksLikeImageURL(value: string): boolean {
  return /\.(avif|gif|jpe?g|png|svg|webp)$/i.test(value.split("?")[0] || "");
}

function guessFileName(url: string): string {
  const cleaned = url.split("#")[0].split("?")[0];
  const parts = cleaned.split("/").filter(Boolean);
  return parts[parts.length - 1] ?? url;
}

function formatBytes(size: number): string {
  if (!Number.isFinite(size)) {
    return "0 B";
  }
  if (size < 1024) {
    return `${size} B`;
  }
  if (size < 1024 * 1024) {
    return `${(size / 1024).toFixed(1)} KB`;
  }
  return `${(size / (1024 * 1024)).toFixed(1)} MB`;
}
