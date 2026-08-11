import { beforeEach, describe, expect, test, vi } from "vitest";

import { apiFetch } from "../../lib/api";
import { FinanceApiError, getFinanceSummary } from "./financeApi";

vi.mock("../../lib/api", () => ({
  apiFetch: vi.fn(),
}));

const mockedApiFetch = vi.mocked(apiFetch);

describe("Finance API boundary", () => {
  beforeEach(() => {
    mockedApiFetch.mockReset();
  });

  // テスト内容: Finance APIのHTTP statusと公開error codeを例外として保持することを確認する。
  // 必要な理由: UIが認証・権限・一時障害をstatusごとに正しく分岐するため。
  test("preserves the response status and error code", async () => {
    mockedApiFetch.mockResolvedValue(
      new Response(
        JSON.stringify({
          error: { code: "finance_summary_forbidden", message: "Permission denied." },
        }),
        { status: 403, headers: { "Content-Type": "application/json" } },
      ),
    );

    await expect(getFinanceSummary()).rejects.toMatchObject<FinanceApiError>({
      status: 403,
      code: "finance_summary_forbidden",
    });
  });
});
