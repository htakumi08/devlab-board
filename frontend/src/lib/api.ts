const apiBaseUrl = import.meta.env.VITE_API_BASE_URL ?? "http://localhost:30102";

// 共通設定を付けてバックエンドAPIへリクエストする。
export async function apiFetch(path: string, init: RequestInit = {}) {
  return fetch(`${apiBaseUrl}${path}`, {
    ...init,
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      ...(init.headers ?? {}),
    },
  });
}
