import { useAuth } from "./store";
import type { ApiError } from "./types";

// In dev mode with Vite proxy, API_BASE is "" (same-origin, proxy forwards /v1 -> :8080).
// In Trae sandbox preview or production, we detect if the proxy is unavailable
// and fall back to direct backend access on port 8080.
const API_BASE = (import.meta as any).env?.DEV
  ? (window.location.port === "5173" ? "" : `http://${window.location.hostname}:8080`)
  : "";

export class HttpError extends Error {
  code: string;
  status: number;
  retryable: boolean;
  constructor(status: number, code: string, message: string, retryable: boolean) {
    super(message);
    this.status = status;
    this.code = code;
    this.retryable = retryable;
  }
}

async function request<T>(
  method: string,
  path: string,
  body?: unknown
): Promise<T> {
  const token = useAuth.getState().token;
  const headers: Record<string, string> = {};
  if (body !== undefined) headers["Content-Type"] = "application/json";
  if (token) headers["Authorization"] = `Bearer ${token}`;

  const resp = await fetch(`${API_BASE}${path}`, {
    method,
    headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });

  if (resp.status === 204) return undefined as T;

  const text = await resp.text();
  if (resp.status >= 400) {
    let code = "UNKNOWN";
    let message = "request failed";
    let retryable = false;
    try {
      const err = JSON.parse(text) as ApiError;
      code = err.error?.code ?? code;
      message = err.error?.message ?? message;
      retryable = err.error?.retryable ?? false;
    } catch {
      // non-JSON error
    }
    throw new HttpError(resp.status, code, message, retryable);
  }

  if (!text) return undefined as T;
  return JSON.parse(text) as T;
}

export const api = {
  get: <T>(path: string) => request<T>("GET", path),
  post: <T>(path: string, body?: unknown) => request<T>("POST", path, body),
};

// Health check (unauthenticated).
export async function healthCheck(): Promise<{ status: string }> {
  const resp = await fetch(`${API_BASE}/healthz`);
  return resp.json();
}
