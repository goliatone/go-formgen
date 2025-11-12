import type { ComponentContext, ComponentFactory } from "../registry";
import {
  addElementClasses,
  classesToString,
  getThemeClasses,
  type FileUploaderClassMap,
} from "../../theme/classes";
import { renderFieldError, clearFieldError } from "../../errors";

type FileUploaderVariant = "input" | "image" | "dropzone";

export interface UploadedFile {
  name: string;
  originalName: string;
  size: number;
  contentType: string;
  url: string;
  thumbnail?: string;
}

export interface FileSerializerHookContext {
  files: UploadedFile[];
  input: HTMLInputElement;
  fieldName: string;
  form: HTMLFormElement | null;
}

export type FileSerializerHook = (context: FileSerializerHookContext) => void;

export interface FileUploaderConfig {
  variant?: FileUploaderVariant;
  maxSize?: number;
  allowedTypes?: string[];
  multiple?: boolean;
  uploadEndpoint?: string;
  uploadMethod?: string;
  autoUpload?: boolean;
  preview?: boolean;
  headers?: Record<string, string>;
  serialize?: FileSerializerHook;
}

type FileStatus = "pending" | "uploading" | "uploaded" | "error";

interface FileEntry {
  id: string;
  file: File;
  status: FileStatus;
  progress: number;
  uploaded?: UploadedFile;
  error?: string;
  previewUrl?: string;
}

const DEFAULT_METHOD = "POST";
const DEFAULT_MAX_SIZE = 25 * 1024 * 1024; // 25MB

export const fileUploaderFactory: ComponentFactory = ({ element, config }: ComponentContext) => {
  const input = element.querySelector<HTMLInputElement>("input, textarea, select");
  if (!input) {
    console.warn("formgen:file-uploader – unable to find input element");
    return;
  }
  const uploader = new FileUploader(element, input, normalizeConfig(config as Record<string, unknown> | undefined));
  return () => uploader.destroy();
};

class FileUploader {
  private readonly element: HTMLElement;
  private readonly input: HTMLInputElement;
  private readonly config: Required<FileUploaderConfig>;
  private readonly theme: FileUploaderClassMap;
  private readonly form: HTMLFormElement | null;
  private readonly originalName: string;
  private readonly fileInput: HTMLInputElement;
  private readonly widget: HTMLElement;
  private readonly fileList: HTMLElement;
  private readonly hiddenContainer: HTMLElement;
  private readonly statusMessage: HTMLElement;
  private readonly previewImage?: HTMLImageElement;
  private readonly dropzone?: HTMLElement;
  private entries: FileEntry[] = [];
  private destroying = false;
  private submitting = false;

  constructor(element: HTMLElement, input: HTMLInputElement, config: Required<FileUploaderConfig>) {
    this.element = element;
    this.input = input;
    this.config = config;
    this.theme = getThemeClasses().fileUploader;
    this.form = input.form;
    this.originalName = input.name;

    this.input.type = "hidden";
    this.input.value = "";

    this.hiddenContainer = document.createElement("div");
    this.hiddenContainer.dataset.fgUploaderHidden = "true";
    this.hiddenContainer.style.display = "none";
    this.input.after(this.hiddenContainer);

    this.widget = document.createElement("div");
    addElementClasses(this.widget, this.theme.wrapper);

    this.fileInput = document.createElement("input");
    this.fileInput.type = "file";
    this.fileInput.multiple = this.config.multiple;
    if (this.config.allowedTypes.length > 0) {
      this.fileInput.accept = this.config.allowedTypes.join(",");
    }
    this.fileInput.className = "sr-only";
    this.widget.appendChild(this.fileInput);

    const control = this.createControl();
    this.widget.appendChild(control);

    this.previewImage = this.config.variant === "image" ? this.createPreview() : undefined;
    if (this.previewImage) {
      this.widget.appendChild(this.previewImage);
    }

    this.fileList = document.createElement("div");
    addElementClasses(this.fileList, this.theme.fileList);
    this.widget.appendChild(this.fileList);

    this.statusMessage = document.createElement("p");
    addElementClasses(this.statusMessage, this.theme.status ?? []);
    this.statusMessage.setAttribute("aria-live", "polite");
    this.widget.appendChild(this.statusMessage);

    element.appendChild(this.widget);

    this.fileInput.addEventListener("change", this.handleFileInputChange);
    this.form?.addEventListener("reset", this.handleFormReset);
    if (!this.config.autoUpload) {
      this.form?.addEventListener("submit", this.handleFormSubmit);
    }

    if (!this.config.uploadEndpoint) {
      this.showError("Upload endpoint is not configured.");
    }
  }

  destroy(): void {
    this.destroying = true;
    this.fileInput.removeEventListener("change", this.handleFileInputChange);
    this.form?.removeEventListener("reset", this.handleFormReset);
    this.form?.removeEventListener("submit", this.handleFormSubmit);
    this.clearEntries();
  }

  private createControl(): HTMLElement {
    if (this.config.variant === "dropzone") {
      const dropzone = document.createElement("div");
      addElementClasses(dropzone, this.theme.dropzone);
      dropzone.textContent = "Drag & drop files or click to browse";
      dropzone.role = "button";
      dropzone.tabIndex = 0;
      dropzone.addEventListener("click", this.triggerFileDialog);
      dropzone.addEventListener("keydown", (event) => {
        if (event.key === "Enter" || event.key === " ") {
          event.preventDefault();
          this.triggerFileDialog();
        }
      });
      const prevent = (event: DragEvent) => {
        event.preventDefault();
        event.stopPropagation();
      };
      dropzone.addEventListener("dragover", prevent);
      dropzone.addEventListener("dragenter", prevent);
      dropzone.addEventListener("drop", (event) => {
        prevent(event);
        if (event.dataTransfer?.files) {
          this.handleSelectedFiles(event.dataTransfer.files);
        }
      });
      this.dropzone = dropzone;
      return dropzone;
    }

    const button = document.createElement("button");
    button.type = "button";
    addElementClasses(button, this.theme.button);
    button.textContent = this.config.variant === "image" ? "Choose image" : "Choose files";
    button.addEventListener("click", this.triggerFileDialog);
    return button;
  }

  private createPreview(): HTMLImageElement {
    const img = document.createElement("img");
    addElementClasses(img, this.theme.preview);
    img.alt = "Selected image preview";
    img.hidden = true;
    return img;
  }

  private triggerFileDialog = () => {
    this.fileInput.click();
  };

  private handleFileInputChange = () => {
    if (this.fileInput.files) {
      this.handleSelectedFiles(this.fileInput.files);
      this.fileInput.value = "";
    }
  };

  private handleSelectedFiles(list: FileList): void {
    const files = Array.from(list);
    if (!files.length) {
      return;
    }
    if (!this.config.multiple) {
      this.clearEntries();
    }
    for (const file of files) {
      const error = this.validateFile(file);
      if (error) {
        this.showError(error);
        continue;
      }
      this.addEntry(file);
    }
  }

  private validateFile(file: File): string | null {
    if (file.size > this.config.maxSize) {
      return `File "${file.name}" exceeds maximum size of ${formatBytes(this.config.maxSize)}.`;
    }
    if (this.config.allowedTypes.length > 0) {
      const ok = this.config.allowedTypes.some((type) => matchesType(file, type));
      if (!ok) {
        return `File "${file.name}" is not an allowed type.`;
      }
    }
    return null;
  }

  private addEntry(file: File): void {
    const entry: FileEntry = {
      id: `${Date.now()}-${Math.random().toString(36).slice(2)}`,
      file,
      status: this.config.autoUpload ? "uploading" : "pending",
      progress: 0,
      previewUrl: this.config.preview && file.type.startsWith("image/") ? URL.createObjectURL(file) : undefined,
    };
    this.entries.push(entry);
    this.refreshUI();
    if (this.config.autoUpload) {
      void this.uploadEntry(entry);
    }
  }

  private async uploadEntry(entry: FileEntry): Promise<void> {
    entry.status = "uploading";
    entry.progress = 0;
    this.refreshUI();
    try {
      const uploaded = await this.sendUpload(entry.file);
      entry.uploaded = uploaded;
      entry.status = "uploaded";
      entry.progress = 100;
      entry.error = undefined;
      this.serializeFiles();
      clearFieldError(this.element);
    } catch (error) {
      entry.status = "error";
      entry.error = error instanceof Error ? error.message : "Upload failed";
      entry.progress = 0;
      this.showError(entry.error);
    } finally {
      this.refreshUI();
    }
  }

  private async sendUpload(file: File): Promise<UploadedFile> {
    if (!this.config.uploadEndpoint) {
      throw new Error("Upload endpoint is not configured.");
    }
    const headers = buildRequestHeaders(this.config.headers, this.element);
    const formData = new FormData();
    formData.append("file", file, file.name);
    const response = await fetch(this.config.uploadEndpoint, {
      method: this.config.uploadMethod ?? DEFAULT_METHOD,
      headers,
      body: formData,
    });
    if (!response.ok) {
      throw new Error(`Upload failed (${response.status})`);
    }
    const payload = (await response.json()) as Partial<UploadedFile>;
    if (!payload || typeof payload.url !== "string") {
      throw new Error("Upload response missing url.");
    }
    return {
      name: payload.name ?? payload.url,
      originalName: payload.originalName ?? file.name,
      size: payload.size ?? file.size,
      contentType: payload.contentType ?? file.type,
      url: payload.url,
      thumbnail: payload.thumbnail,
    };
  }

  private refreshUI(): void {
    this.renderFileList();
    this.updatePreview();
  }

  private renderFileList(): void {
    this.fileList.innerHTML = "";
    for (const entry of this.entries) {
      const item = document.createElement("div");
      addElementClasses(item, this.theme.fileItem);
      const meta = document.createElement("div");
      addElementClasses(meta, this.theme.fileMeta ?? []);
      meta.innerHTML = `<span class="${classesToString(this.theme.fileName ?? [])}">${entry.file.name}</span>
        <span class="${classesToString(this.theme.fileSize ?? [])}">${formatBytes(entry.file.size)}</span>`;
      item.appendChild(meta);

      const actions = document.createElement("div");
      addElementClasses(actions, this.theme.fileActions ?? []);
      const status = document.createElement("span");
      status.textContent = statusLabel(entry);
      actions.appendChild(status);

      const removeBtn = document.createElement("button");
      removeBtn.type = "button";
      removeBtn.textContent = "Remove";
      removeBtn.className = "text-xs text-red-500 hover:text-red-600";
      removeBtn.addEventListener("click", () => this.removeEntry(entry.id));
      actions.appendChild(removeBtn);

      item.appendChild(actions);

      const progress = document.createElement("div");
      addElementClasses(progress, this.theme.progress);
      const bar = document.createElement("div");
      bar.className = "bg-blue-500 h-full transition-all duration-200";
      bar.style.width = `${entry.progress}%`;
      progress.appendChild(bar);
      item.appendChild(progress);

      if (entry.error) {
        const error = document.createElement("p");
        addElementClasses(error, this.theme.error);
        error.textContent = entry.error;
        item.appendChild(error);
      }

      this.fileList.appendChild(item);
    }
  }

  private removeEntry(id: string): void {
    const index = this.entries.findIndex((entry) => entry.id === id);
    if (index === -1) {
      return;
    }
    const [removed] = this.entries.splice(index, 1);
    if (removed.previewUrl) {
      URL.revokeObjectURL(removed.previewUrl);
    }
    this.serializeFiles();
    this.refreshUI();
  }

  private updatePreview(): void {
    if (!this.previewImage) {
      return;
    }
    const target =
      this.entries.find((entry) => entry.uploaded?.thumbnail || entry.previewUrl) ?? this.entries[0];
    if (!target) {
      this.previewImage.hidden = true;
      this.previewImage.src = "";
      return;
    }
    const nextSource = target.uploaded?.thumbnail ?? target.uploaded?.url ?? target.previewUrl ?? "";
    if (!nextSource) {
      this.previewImage.hidden = true;
      return;
    }
    this.previewImage.hidden = false;
    this.previewImage.src = nextSource;
  }

  private handleFormReset = () => {
    this.clearEntries();
    this.serializeFiles();
    this.refreshUI();
    this.statusMessage.textContent = "";
    clearFieldError(this.element);
  };

  private handleFormSubmit = async (event: SubmitEvent) => {
    if (this.config.autoUpload || this.submitting) {
      return;
    }
    const pending = this.entries.filter((entry) => entry.status !== "uploaded");
    if (pending.length === 0) {
      return;
    }
    event.preventDefault();
    this.submitting = true;
    try {
      for (const entry of pending) {
        await this.uploadEntry(entry);
        if (entry.status !== "uploaded") {
          throw new Error("Unable to upload file.");
        }
      }
      this.serializeFiles();
      this.form?.submit();
    } catch (error) {
      this.showError(error instanceof Error ? error.message : "Upload failed");
    } finally {
      this.submitting = false;
    }
  };

  private clearEntries(): void {
    for (const entry of this.entries) {
      if (entry.previewUrl) {
        URL.revokeObjectURL(entry.previewUrl);
      }
    }
    this.entries = [];
    this.hiddenContainer.innerHTML = "";
    this.input.value = "";
    this.input.name = this.originalName;
  }

  private serializeFiles(): void {
    const uploaded = this.entries
      .filter((entry) => entry.uploaded)
      .map((entry) => entry.uploaded as UploadedFile);

    if (typeof this.config.serialize === "function") {
      try {
        this.config.serialize({
          files: uploaded,
          input: this.input,
          fieldName: this.input.name,
          form: this.form,
        });
      } catch (error) {
        console.warn("formgen:file-uploader – serialize hook failed", error);
      }
      return;
    }

    this.hiddenContainer.innerHTML = "";

    if (!uploaded.length) {
      this.input.name = this.originalName;
      this.input.value = "";
      return;
    }

    if (!this.config.multiple) {
      this.input.name = this.originalName;
      this.input.value = uploaded[0].url;
      return;
    }

    const baseName = this.originalName.endsWith("[]") ? this.originalName : `${this.originalName}[]`;
    this.input.name = baseName;
    this.input.value = uploaded[0].url;
    for (let index = 1; index < uploaded.length; index += 1) {
      const hidden = document.createElement("input");
      hidden.type = "hidden";
      hidden.name = baseName;
      hidden.value = uploaded[index].url;
      this.hiddenContainer.appendChild(hidden);
    }
  }

  private showError(message: string): void {
    this.statusMessage.textContent = message;
    renderFieldError(this.element, message);
  }
}

function buildRequestHeaders(
  headers: Record<string, string>,
  element: HTMLElement
): Record<string, string> {
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
      index === 0 ? part.toLowerCase() : part.charAt(0).toUpperCase() + part.slice(1).toLowerCase()
    )
    .join("");
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

function normalizeConfig(raw?: Record<string, unknown>): Required<FileUploaderConfig> {
  const config = (raw ?? {}) as FileUploaderConfig;
  const toBool = (value: unknown, fallback = false) => {
    if (typeof value === "boolean") {
      return value;
    }
    if (typeof value === "string") {
      return value.toLowerCase() === "true";
    }
    return fallback;
  };
  const toNumber = (value: unknown, fallback: number) => {
    if (typeof value === "number") {
      return value;
    }
    if (typeof value === "string" && value.trim() !== "") {
      const parsed = Number(value);
      return Number.isFinite(parsed) ? parsed : fallback;
    }
    return fallback;
  };
  const headers: Record<string, string> = {};
  if (config.headers && typeof config.headers === "object") {
    Object.entries(config.headers).forEach(([key, value]) => {
      if (value === undefined || value === null) {
        return;
      }
      headers[key] = String(value);
    });
  }
  return {
    variant: (config.variant as FileUploaderVariant) ?? "input",
    maxSize: toNumber(config.maxSize, DEFAULT_MAX_SIZE),
    allowedTypes: Array.isArray(config.allowedTypes)
      ? config.allowedTypes.filter((value): value is string => typeof value === "string")
      : typeof config.allowedTypes === "string"
      ? config.allowedTypes.split(",").map((value) => value.trim()).filter(Boolean)
      : [],
    multiple: toBool(config.multiple, false),
    uploadEndpoint: config.uploadEndpoint ?? "",
    uploadMethod: config.uploadMethod ?? DEFAULT_METHOD,
    autoUpload: config.autoUpload !== undefined ? toBool(config.autoUpload, true) : true,
    preview: config.preview !== undefined ? toBool(config.preview, config.variant === "image") : config.variant === "image",
    headers,
    serialize: config.serialize,
  };
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

function statusLabel(entry: FileEntry): string {
  switch (entry.status) {
    case "pending":
      return "Pending upload";
    case "uploading":
      return `Uploading… ${entry.progress}%`;
    case "uploaded":
      return "Uploaded";
    case "error":
      return "Error";
    default:
      return "Unknown";
  }
}
