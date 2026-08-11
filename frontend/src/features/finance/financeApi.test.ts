import { beforeEach, describe, expect, test, vi } from "vitest";

import { apiFetch } from "../../lib/api";
import {
  FinanceApiError,
  financeQueryKeys,
  getFinanceAccount,
  getFinanceAccounts,
  getFinanceCategories,
  getFinanceSummary,
  getFinanceTransactions,
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
    expect(financeQueryKeys.categories()).toEqual(["finance", "categories", "list"]);
    const firstPage = {
      accountId: null,
      dateFrom: null,
      dateTo: null,
      category: null,
      direction: null,
      status: null,
      sort: "newest",
      cursor: null,
    };
    expect(financeQueryKeys.transactions(firstPage)).toEqual([
      "finance",
      "transactions",
      "list",
      firstPage,
    ]);
    const nextPage = { ...firstPage, cursor: "cursor/value" };
    expect(financeQueryKeys.transactions(nextPage)).toEqual([
      "finance",
      "transactions",
      "list",
      nextPage,
    ]);
    expect(financeQueryKeys.transactions({ ...firstPage, status: "posted" })).not.toEqual(
      financeQueryKeys.transactions(firstPage),
    );
  });

  // テスト内容: category masterを専用endpointからAbortSignal付きで取得することを確認する。
  // 必要な理由: DBを正とするカテゴリー候補をfrontendへ固定値として重複させないため。
  test("requests the Finance category list with an abort signal", async () => {
    const controller = new AbortController();
    const categories = [{ code: "groceries", label: "食料品" }];
    mockedApiFetch.mockResolvedValue(
      new Response(JSON.stringify({ categories }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    await expect(getFinanceCategories(controller.signal)).resolves.toEqual(categories);
    expect(mockedApiFetch).toHaveBeenCalledWith("/api/finance/categories", {
      signal: controller.signal,
    });
  });

  // テスト内容: 取引履歴の先頭ページを既定25件とAbortSignal付きで取得することを確認する。
  // 必要な理由: API既定値への暗黙依存とlogout後に取得が継続する回帰を防ぐため。
  test("requests the first Transactions page with the fixed page size", async () => {
    const controller = new AbortController();
    mockedApiFetch.mockResolvedValue(
      new Response(JSON.stringify({ transactions: [], nextCursor: null }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    await expect(getFinanceTransactions({
      accountId: null,
      dateFrom: null,
      dateTo: null,
      category: null,
      direction: null,
      status: null,
      sort: "newest",
      cursor: null,
    }, controller.signal)).resolves.toEqual({
      transactions: [],
      nextCursor: null,
    });
    expect(mockedApiFetch).toHaveBeenCalledWith("/api/finance/transactions?limit=25", {
      signal: controller.signal,
    });
  });

  // テスト内容: APIが返したopaque cursorを解釈せず安全にURL encodeして次ページを取得することを確認する。
  // 必要な理由: cursor内の予約文字でquery境界が壊れたりclientがcursorへ依存したりしないため。
  test("requests the next Transactions page with an encoded opaque cursor", async () => {
    mockedApiFetch.mockResolvedValue(
      new Response(JSON.stringify({ transactions: [], nextCursor: null }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    await getFinanceTransactions({
      accountId: "account/value",
      dateFrom: "2026-08-01",
      dateTo: "2026-08-11",
      category: "uncategorized",
      direction: "debit",
      status: "posted",
      sort: "oldest",
      cursor: "cursor/value + next",
    });

    expect(mockedApiFetch).toHaveBeenCalledWith(
      "/api/finance/transactions?account_id=account%2Fvalue&date_from=2026-08-01&date_to=2026-08-11&category=uncategorized&direction=debit&status=posted&sort=oldest&cursor=cursor%2Fvalue+%2B+next&limit=25",
      { signal: undefined },
    );
  });
});
