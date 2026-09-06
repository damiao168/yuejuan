import { ApiClientError, apiClientErrorFromResponse, type DesktopApiClient } from "./client";

export interface CaptureUploadInitInput {
  sha256: string;
  size: number;
  mime: string;
  exam: string;
  batch: string;
  idempotency_key: string;
  filename?: string;
}

export interface CaptureUploadInitResponse {
  remote_upload_id: string;
  chunk_size: number;
  confirmed_offset: number;
  already_exists: boolean;
  status: "uploading" | "finalizing" | "completed" | "failed";
  file_asset_id?: string;
  capture_file_id?: string;
  error_code?: string;
}

export interface CaptureUploadCompleteResponse {
  remote_upload_id: string;
  confirmed_offset: number;
  status: "uploading" | "finalizing" | "completed" | "failed";
  file_asset_id?: string;
  capture_file_id?: string;
  error_code?: string;
}

export async function sha256ForFile(file: Blob) {
  const digest = await crypto.subtle.digest("SHA-256", await file.arrayBuffer());
  return Array.from(new Uint8Array(digest), (byte) => byte.toString(16).padStart(2, "0")).join("");
}

export function initCaptureUpload(client: DesktopApiClient, input: CaptureUploadInitInput) {
  return client.request<CaptureUploadInitResponse>("/api/v1/capture/uploads:init", {
    method: "POST",
    body: JSON.stringify(input)
  });
}

// The server advances confirmed_offset atomically.  Retrying the same chunk
// is safe; callers must persist the returned offset before scheduling another
// chunk so a process crash cannot produce duplicate answer-sheet pages.
export async function putCaptureUploadChunk(client: DesktopApiClient, uploadId: string, offset: number, chunk: Blob) {
  const bytes = new Uint8Array(await chunk.arrayBuffer());
  const hash = await sha256ForBytes(bytes);
  const headers = new Headers({
    Accept: "application/json",
    "Content-Type": "application/octet-stream",
    "Upload-Offset": String(offset),
    "X-Chunk-SHA256": hash
  });
  const authorization = client.authorizationHeader();
  if (authorization) {
    headers.set("Authorization", authorization);
  }
  const response = await fetch(client.url(`/api/v1/capture/uploads/${encodeURIComponent(uploadId)}/chunks`), {
    method: "PUT",
    headers,
    body: bytes
  });
  if (!response.ok) {
    throw await apiClientErrorFromResponse(response);
  }
  return (await response.json()) as Pick<CaptureUploadInitResponse, "remote_upload_id" | "confirmed_offset" | "status">;
}

export function completeCaptureUpload(client: DesktopApiClient, uploadId: string, sha256: string) {
  return client.request<CaptureUploadCompleteResponse>(`/api/v1/capture/uploads/${encodeURIComponent(uploadId)}/complete`, {
    method: "POST",
    body: JSON.stringify({ sha256 })
  });
}

export interface ResumeCaptureUploadInput extends Omit<CaptureUploadInitInput, "sha256" | "size" | "mime" | "filename"> {
  file: File | CaptureUploadSource;
}

export interface CaptureUploadProgress {
  remoteUploadId: string;
  confirmedOffset: number;
  totalBytes: number;
  status: CaptureUploadInitResponse["status"];
  fileAssetId?: string;
  captureFileId?: string;
}

export interface CaptureUploadSource {
  name: string;
  type: string;
  size: number;
  sha256: string;
  slice(start: number, end: number): Blob | Promise<Blob>;
}

export async function resumeCaptureUpload(
  client: DesktopApiClient,
  input: ResumeCaptureUploadInput,
  onProgress: (progress: CaptureUploadProgress) => Promise<void> | void
) {
  const sha256 = "sha256" in input.file ? input.file.sha256 : await sha256ForFile(input.file);
  const initialized = await initCaptureUpload(client, {
    sha256,
    size: input.file.size,
    mime: input.file.type || inferCaptureMime(input.file.name),
    exam: input.exam,
    batch: input.batch,
    idempotency_key: input.idempotency_key,
    filename: input.file.name
  });
  let confirmedOffset = initialized.confirmed_offset;
  await onProgress({
    remoteUploadId: initialized.remote_upload_id,
    confirmedOffset,
    totalBytes: input.file.size,
    status: initialized.status,
    fileAssetId: initialized.file_asset_id,
    captureFileId: initialized.capture_file_id
  });
  if (initialized.status === "completed") {
    return initialized;
  }
  if (initialized.status !== "uploading") {
    throw new ApiClientError(409, "capture_upload_not_resumable", `capture upload is ${initialized.status}`);
  }
  while (confirmedOffset < input.file.size) {
    const end = Math.min(confirmedOffset + initialized.chunk_size, input.file.size);
    const chunk = await input.file.slice(confirmedOffset, end);
    const chunkResult = await putCaptureUploadChunk(client, initialized.remote_upload_id, confirmedOffset, chunk);
    if (chunkResult.confirmed_offset <= confirmedOffset || chunkResult.confirmed_offset > input.file.size) {
      throw new ApiClientError(409, "capture_upload_invalid_offset", "server returned an invalid confirmed offset");
    }
    confirmedOffset = chunkResult.confirmed_offset;
    await onProgress({ remoteUploadId: initialized.remote_upload_id, confirmedOffset, totalBytes: input.file.size, status: chunkResult.status });
  }
  const completed = await completeCaptureUpload(client, initialized.remote_upload_id, sha256);
  await onProgress({
    remoteUploadId: completed.remote_upload_id,
    confirmedOffset: completed.confirmed_offset,
    totalBytes: input.file.size,
    status: completed.status,
    fileAssetId: completed.file_asset_id,
    captureFileId: completed.capture_file_id
  });
  return completed;
}

async function sha256ForBytes(bytes: Uint8Array) {
  const material = new ArrayBuffer(bytes.byteLength);
  new Uint8Array(material).set(bytes);
  const digest = await crypto.subtle.digest("SHA-256", material);
  return Array.from(new Uint8Array(digest), (byte) => byte.toString(16).padStart(2, "0")).join("");
}

function inferCaptureMime(name: string) {
  const extension = name.slice(name.lastIndexOf(".")).toLowerCase();
  if (extension === ".pdf") return "application/pdf";
  if (extension === ".png") return "image/png";
  if (extension === ".jpg" || extension === ".jpeg") return "image/jpeg";
  if (extension === ".tif" || extension === ".tiff") return "image/tiff";
  return "application/octet-stream";
}
