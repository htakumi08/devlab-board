import { QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { beforeEach, describe, expect, test, vi } from "vitest";

import { apiFetch } from "../../lib/api";
import { createAppQueryClient } from "../../lib/queryClient";
import { FinanceAccountDetail } from "./FinanceAccountDetail";
import { FinanceAccounts } from "./FinanceAccounts";

vi.mock("../../lib/api", () => ({
  apiFetch: vi.fn(),
}));

const mockedApiFetch = vi.mocked(apiFetch);

const account = {
  id: "370fdd2d-aeb9-492d-a981-c40923531411",
  name: "Synthetic Checking",
  accountType: "checking",
  mask: "1234",
  currency: "JPY",
  currentAmountMinor: "9007199254740993",
  availableAmountMinor: "165000",
  status: "active",
  balanceAsOf: "2026-08-11T01:05:00Z",
};

const transaction = {
  id: "024c4ea1-e906-4f2a-a32b-f70f95762f76",
  accountId: account.id,
  accountName: account.name,
  name: "Synthetic Grocery",
  merchant: "Sample Market",
  amountMinor: "4200",
  currency: "JPY",
  direction: "debit" as const,
  status: "posted" as const,
  category: { code: "groceries", label: "食料品" },
  occurredAt: "2026-08-11T01:00:00Z",
};

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function renderFinanceRoutes(initialPath = "/finance-lab/accounts", onSessionExpired = vi.fn()) {
  const queryClient = createAppQueryClient();
  const onLogout = vi.fn(async () => undefined);
  const result = render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[initialPath]}>
        <Routes>
          <Route
            path="/finance-lab/accounts"
            element={
              <FinanceAccounts onLogout={onLogout} onSessionExpired={onSessionExpired} />
            }
          />
          <Route
            path="/finance-lab/accounts/:accountId"
            element={
              <FinanceAccountDetail onLogout={onLogout} onSessionExpired={onSessionExpired} />
            }
          />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
  return { ...result, onLogout, onSessionExpired, queryClient };
}

describe("Finance Accounts", () => {
  beforeEach(() => {
    mockedApiFetch.mockReset();
  });

  // テスト内容: Accounts一覧の初回取得中にloading状態を通知することを確認する。
  // 必要な理由: 空一覧と通信待ちを利用者や支援技術が区別できるようにするため。
  test("announces the Accounts loading state", async () => {
    mockedApiFetch.mockImplementation(() => new Promise<Response>(() => undefined));

    renderFinanceRoutes();

    expect(await screen.findByRole("status")).toHaveTextContent("口座一覧を読み込んでいます");
  });

  // テスト内容: Accounts APIが空配列の場合にempty stateを表示することを確認する。
  // 必要な理由: データがない正常状態を取得失敗として扱わないため。
  test("renders the Accounts empty state", async () => {
    mockedApiFetch.mockResolvedValue(jsonResponse({ accounts: [] }));

    renderFinanceRoutes();

    expect(await screen.findByRole("heading", { name: "口座がありません" })).toBeInTheDocument();
  });

  // テスト内容: mask・status・口座種別・精度を保った残高を一覧へ表示しkeyboardで詳細へ移動できることを確認する。
  // 必要な理由: 完全番号を出さず、主要な口座参照フローをpointerなしで完結させるため。
  test("renders masked Accounts and opens a detail with the keyboard", async () => {
    mockedApiFetch.mockImplementation(async (path) => {
      if (path === "/api/finance/accounts") {
        return jsonResponse({ accounts: [account] });
      }
      if (path === `/api/finance/accounts/${account.id}`) {
        return jsonResponse({ account, recentTransactions: [] });
      }
      throw new Error(`Unexpected API path: ${path}`);
    });
    const user = userEvent.setup();
    renderFinanceRoutes();

    expect(await screen.findByText("•••• 1234")).toBeInTheDocument();
    expect(screen.getByText("Checking")).toBeInTheDocument();
    expect(screen.getByText("Active")).toBeInTheDocument();
    expect(screen.getByText("￥9,007,199,254,740,993")).toBeInTheDocument();
    const detailLink = screen.getByRole("link", { name: "Synthetic Checkingの詳細を見る" });
    detailLink.focus();
    expect(detailLink).toHaveFocus();
    await user.keyboard("{Enter}");

    expect(await screen.findByRole("heading", { name: "Synthetic Checking" })).toBeInTheDocument();
  });

  // テスト内容: Accounts APIの500後に明示的な再試行でempty stateへ回復することを確認する。
  // 必要な理由: 一時的なサーバー障害から画面再読込なしで復旧できるようにするため。
  test("retries the Accounts list after a server error", async () => {
    let requests = 0;
    mockedApiFetch.mockImplementation(async () => {
      requests += 1;
      return requests === 1
        ? jsonResponse({ error: { code: "finance_accounts_unavailable" } }, 500)
        : jsonResponse({ accounts: [] });
    });
    const user = userEvent.setup();
    renderFinanceRoutes();

    await user.click(await screen.findByRole("button", { name: "再試行" }));

    expect(await screen.findByRole("heading", { name: "口座がありません" })).toBeInTheDocument();
    expect(requests).toBe(2);
  });

  // テスト内容: Accounts APIの403でpermission stateを表示し、再試行を提示しないことを確認する。
  // 必要な理由: 権限不足を一時障害として誤案内しないため。
  test("renders permission denied for the Accounts list", async () => {
    mockedApiFetch.mockResolvedValue(
      jsonResponse({ error: { code: "finance_accounts_forbidden" } }, 403),
    );

    renderFinanceRoutes();

    expect(await screen.findByRole("alert")).toHaveTextContent("口座一覧を表示する権限がありません");
    expect(screen.queryByRole("button", { name: "再試行" })).not.toBeInTheDocument();
  });

  // テスト内容: Accounts APIの401を既存session失効callbackへ渡すことを確認する。
  // 必要な理由: Finance画面内に期限切れsessionや機微cacheを残さないため。
  test("reports an unauthorized Accounts request", async () => {
    mockedApiFetch.mockResolvedValue(jsonResponse({ error: { code: "unauthorized" } }, 401));
    const onSessionExpired = vi.fn();

    renderFinanceRoutes("/finance-lab/accounts", onSessionExpired);

    await vi.waitFor(() => expect(onSessionExpired).toHaveBeenCalledTimes(1));
    expect(screen.queryByRole("button", { name: "再試行" })).not.toBeInTheDocument();
  });

  // テスト内容: Account Detailの初回取得中にloading状態を通知することを確認する。
  // 必要な理由: 直接URLで開いた場合も空画面を表示しないため。
  test("announces the Account Detail loading state", async () => {
    mockedApiFetch.mockImplementation(() => new Promise<Response>(() => undefined));

    renderFinanceRoutes(`/finance-lab/accounts/${account.id}`);

    expect(await screen.findByRole("status")).toHaveTextContent("口座詳細を読み込んでいます");
  });

  // テスト内容: Account Detailにmask、残高、基準日時、取引empty stateを表示することを確認する。
  // 必要な理由: 詳細APIだけで口座情報を復元し、完全番号を画面へ出さないため。
  test("renders an Account Detail with an empty transaction state", async () => {
    mockedApiFetch.mockResolvedValue(jsonResponse({ account, recentTransactions: [] }));

    renderFinanceRoutes(`/finance-lab/accounts/${account.id}`);

    expect(await screen.findByRole("heading", { name: "Synthetic Checking" })).toBeInTheDocument();
    expect(screen.getByText("•••• 1234")).toBeInTheDocument();
    expect(screen.getByText("￥9,007,199,254,740,993")).toBeInTheDocument();
    expect(screen.getByText("最近の取引はありません。")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "口座一覧へ戻る" })).toHaveAttribute(
      "href",
      "/finance-lab/accounts",
    );
  });

  // テスト内容: Account Detailのnetwork失敗後に再試行し、口座別取引を表示できることを確認する。
  // 必要な理由: HTTP responseを持たない通信障害から取引表示まで復旧できるようにするため。
  test("retries an Account Detail after a network error", async () => {
    let requests = 0;
    mockedApiFetch.mockImplementation(async () => {
      requests += 1;
      if (requests === 1) {
        throw new TypeError("Failed to fetch");
      }
      return jsonResponse({ account, recentTransactions: [transaction] });
    });
    const user = userEvent.setup();
    renderFinanceRoutes(`/finance-lab/accounts/${account.id}`);

    await user.click(await screen.findByRole("button", { name: "再試行" }));

    expect(await screen.findByText("Synthetic Grocery")).toBeInTheDocument();
    expect(screen.getByText("-￥4,200")).toBeInTheDocument();
    expect(requests).toBe(2);
  });

  // テスト内容: Account Detailの403でpermission stateを表示し、再試行を提示しないことを確認する。
  // 必要な理由: roleによる禁止を通信障害やresource不存在と混同しないため。
  test("renders permission denied for an Account Detail", async () => {
    mockedApiFetch.mockResolvedValue(
      jsonResponse({ error: { code: "finance_account_forbidden" } }, 403),
    );

    renderFinanceRoutes(`/finance-lab/accounts/${account.id}`);

    expect(await screen.findByRole("alert")).toHaveTextContent("口座詳細を表示する権限がありません");
    expect(screen.queryByRole("button", { name: "再試行" })).not.toBeInTheDocument();
  });

  // テスト内容: Account Detailの401を既存session失効callbackへ渡すことを確認する。
  // 必要な理由: 詳細直接URLでも認証境界とcache破棄を迂回させないため。
  test("reports an unauthorized Account Detail request", async () => {
    mockedApiFetch.mockResolvedValue(jsonResponse({ error: { code: "unauthorized" } }, 401));
    const onSessionExpired = vi.fn();

    renderFinanceRoutes(`/finance-lab/accounts/${account.id}`, onSessionExpired);

    await vi.waitFor(() => expect(onSessionExpired).toHaveBeenCalledTimes(1));
    expect(screen.queryByRole("button", { name: "再試行" })).not.toBeInTheDocument();
  });

  // テスト内容: Account Detailの404で所有者境界を漏らさないnot-found stateを表示することを確認する。
  // 必要な理由: 不存在と他ユーザー所有を画面から区別できないようにするため。
  test("renders not found for a hidden Account Detail", async () => {
    mockedApiFetch.mockResolvedValue(
      jsonResponse({ error: { code: "finance_account_not_found" } }, 404),
    );

    renderFinanceRoutes(`/finance-lab/accounts/${account.id}`);

    expect(await screen.findByRole("heading", { name: "口座が見つかりません" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "口座一覧へ戻る" })).toBeInTheDocument();
  });
});
