import type { MFAChallengeStartResponse, MFAChallengeVerifyResponse, MFADisableResponse, MFARecoveryCodesResponse, MFAStatusResponse, TOTPEnrollmentResponse } from "@edugrade/sdk";
import { apiClient } from "./client";

export type MFAOperation = MFAChallengeStartResponse["operation"];
export type MFAMethod = MFAChallengeStartResponse["methods"][number];

// Deliberately not a query-cache API: provisioning secrets, proofs and recovery
// codes must not survive in shared caches, storage or idempotency receipts.
async function request<T>(path: string, init: RequestInit, signal?: AbortSignal) {
  const transport = new AbortController();
  const abort = () => transport.abort();
  if (signal?.aborted) abort();
  else signal?.addEventListener("abort", abort, { once: true });
  // An abort is not a rollback. The flow must reconcile state, never retry a
  // secret-bearing mutation just because this bounded browser wait timed out.
  const timeout = setTimeout(abort, 30_000);
  try {
    return await apiClient.request<T>(path, { ...init, cache: "no-store", signal: transport.signal });
  } finally {
    clearTimeout(timeout);
    signal?.removeEventListener("abort", abort);
  }
}

function post<T>(path: string, body: object, signal?: AbortSignal) {
  return request<T>(path, { method: "POST", body: JSON.stringify(body) }, signal);
}

export const mfaAPI = {
  status: (signal?: AbortSignal) => request<MFAStatusResponse>("/api/v1/auth/mfa", {}, signal),
  enroll: (password: string, signal?: AbortSignal) => post<TOTPEnrollmentResponse>("/api/v1/auth/mfa/totp/enroll", { password }, signal),
  confirm: (enrollmentID: string, code: string, signal?: AbortSignal) => post<MFARecoveryCodesResponse>("/api/v1/auth/mfa/totp/confirm", { enrollment_id: enrollmentID, code }, signal),
  start: (operation: MFAOperation, password: string, signal?: AbortSignal) => post<MFAChallengeStartResponse>("/api/v1/auth/step-up/start", { operation, password }, signal),
  verify: (challengeID: string, method: MFAMethod, code: string, signal?: AbortSignal) => post<MFAChallengeVerifyResponse>("/api/v1/auth/step-up/verify", { challenge_id: challengeID, method, code }, signal),
  disable: (challengeID: string, signal?: AbortSignal) => post<MFADisableResponse>("/api/v1/auth/mfa/totp/disable", { challenge_id: challengeID }, signal),
  rotate: (challengeID: string, signal?: AbortSignal) => post<MFARecoveryCodesResponse>("/api/v1/auth/mfa/recovery-codes/rotate", { challenge_id: challengeID }, signal)
};
