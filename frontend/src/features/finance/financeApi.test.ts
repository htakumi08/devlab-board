import { beforeEach, describe, expect, test, vi } from "vitest";

import { apiFetch } from "../../lib/api";
import {
  FinanceApiError,
  financeQueryKeys,
  getFinanceAccount,
  getFinanceAccounts,
  getFinanceSummary,
} from "./financeApi";

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

  // テスト内容: Accounts一覧取得がAbortSignal付きの固定endpointを利用することを確認する。
  // 必要な理由: logout時に進行中のFinance取得を中断し、機微データがcacheへ戻ることを防ぐため。
  test("requests the Accounts list with an abort signal", async () => {
    const controller = new AbortController();
    mockedApiFetch.mockResolvedValue(
      new Response(JSON.stringify({ accounts: [] }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    await expect(getFinanceAccounts(controller.signal)).resolves.toEqual([]);
    expect(mockedApiFetch).toHaveBeenCalledWith("/api/finance/accounts", {
      signal: controller.signal,
    });
  });

  // テスト内容: Account Detail取得で公開IDをpath segmentとしてencodeすることを確認する。
  // 必要な理由: client入力を別pathへ解釈させず、公開IDだけをAPI境界へ渡すため。
  test("requests an Account Detail with an encoded public ID", async () => {
    const controller = new AbortController();
    const account = {
      id: "account/id",
      name: "Synthetic Checking",
      accountType: "checking",
      mask: "1234",
      currency: "JPY",
      currentAmountMinor: "1000",
      availableAmountMinor: null,
      status: "active",
      balanceAsOf: "2026-08-11T01:05:00Z",
    };
    mockedApiFetch.mockResolvedValue(
      new Response(JSON.stringify({ account, recentTransactions: [] }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    await expect(getFinanceAccount("account/id", controller.signal)).resolves.toEqual({
      account,
      recentTransactions: [],
    });
    expect(mockedApiFetch).toHaveBeenCalledWith("/api/finance/accounts/account%2Fid", {
      signal: controller.signal,
    });
  });

  // テスト内容: Finance query keyが共通prefixとaccount IDを含む安定した構造であることを確認する。
  // 必要な理由: route別cacheの混同を防ぎ、logout時にFinance cacheだけを一括破棄するため。
  test("builds scoped and stable Finance query keys", () => {
    expect(financeQueryKeys.all).toEqual(["finance"]);
    expect(financeQueryKeys.summary()).toEqual(["finance", "summary"]);
    expect(financeQueryKeys.accounts()).toEqual(["finance", "accounts", "list"]);
    expect(financeQueryKeys.account("account-1")).toEqual([
      "finance",
      "accounts",
      "detail",
      "account-1",
    ]);
  });
});
