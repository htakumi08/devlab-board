import { apiFetch } from "../../lib/api";

export type UserAgentResponse = {
  userAgent: string;
};

// サーバーが受信したUser-AgentをAPIから取得する。
export async function fetchUserAgent(): Promise<UserAgentResponse> {
  const response = await apiFetch("/api/user-agent");
  if (!response.ok) {
    throw new Error(`User-Agent API returned ${response.status}`);
  }

  const payload: unknown = await response.json();
  if (!isUserAgentResponse(payload)) {
    throw new Error("User-Agent API returned an invalid response");
  }

  return payload;
}

function isUserAgentResponse(value: unknown): value is UserAgentResponse {
  if (typeof value !== "object" || value === null) {
    return false;
  }

  return "userAgent" in value && typeof value.userAgent === "string";
}
