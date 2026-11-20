const API_BASE = "/api";

export async function apiFetch(
  path: string,
  options: RequestInit = {}
): Promise<Response> {
  const token = localStorage.getItem("token");

  const headers: HeadersInit = {
    "Content-Type": "application/json",
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
    ...(options.headers || {}),
  };

  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers,
  });

  if (res.status === 401) {
    localStorage.removeItem("token");
    window.location.href = "/login";
  }

  // Ubah data otomatisasi (tambah/hapus/toggle) -> ringkasan di halaman Otomatisasi
  // memuat ulang sendiri, agar angkanya tak basi.
  const method = (options.method || "GET").toUpperCase();
  if (res.ok && method !== "GET" && AUTOMATION_PATH.test(path)) {
    window.dispatchEvent(new Event(AUTOMATION_CHANGED));
  }

  return res;
}

export const AUTOMATION_CHANGED = "automation:changed";
const AUTOMATION_PATH = /^\/(auto-reply|menu-bots|ai-reply|group-rules)(\/|\?|$)/;

export async function apiGet<T>(path: string): Promise<T> {
  const res = await apiFetch(path);
  if (!res.ok) throw new Error((await res.json()).message || "Request failed");
  return res.json();
}

export async function apiPost<T>(path: string, body?: unknown): Promise<T> {
  const res = await apiFetch(path, {
    method: "POST",
    body: body ? JSON.stringify(body) : undefined,
  });
  if (!res.ok) throw new Error((await res.json()).message || "Request failed");
  return res.json();
}

export async function apiPut<T>(path: string, body?: unknown): Promise<T> {
  const res = await apiFetch(path, {
    method: "PUT",
    body: body ? JSON.stringify(body) : undefined,
  });
  if (!res.ok) throw new Error((await res.json()).message || "Request failed");
  return res.json();
}

export async function apiDelete<T>(path: string): Promise<T> {
  const res = await apiFetch(path, { method: "DELETE" });
  if (!res.ok) throw new Error((await res.json()).message || "Request failed");
  return res.json();
}

export async function apiPatch<T>(path: string, body?: unknown): Promise<T> {
  const res = await apiFetch(path, {
    method: "PATCH",
    body: body ? JSON.stringify(body) : undefined,
  });
  if (!res.ok) throw new Error((await res.json()).message || "Request failed");
  return res.json();
}
