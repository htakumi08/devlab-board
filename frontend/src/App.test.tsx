import { QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, test, vi } from "vitest";

import { App } from "./App";
import { financeQueryKeys } from "./features/finance/financeApi";
import { apiFetch } from "./lib/api";
import { createAppQueryClient } from "./lib/queryClient";

vi.mock("./lib/api", () => ({
  apiFetch: vi.fn(),
}));

const authenticatedUser = {
  id: "user-1",
  email: "finance@example.com",
  name: "Finance User",
  role: "user",
};

const mockedApiFetch = vi.mocked(apiFetch);

const financeAccount = {
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

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function mockAuthenticatedSession() {
  mockedApiFetch.mockImplementation(async (path) => {
    if (path === "/api/auth/me") {
      return jsonResponse({ user: authenticatedUser });
    }
    if (path === "/api/dashboard") {
      return jsonResponse({ cards: [] });
    }
    if (path === "/api/finance/summary") {
      return jsonResponse({
        summary: {
          accountCount: 0,
          balances: [],
          recentTransactions: [],
          asOf: null,
        },
      });
    }

    throw new Error(`Unexpected API path: ${path}`);
  });
}

function renderApp(initialPath: string, queryClient = createAppQueryClient()) {
  const result = render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[initialPath]}>
        <App />
      </MemoryRouter>
    </QueryClientProvider>,
  );
  return { ...result, queryClient };
}

describe("Finance Dashboard navigation", () => {
  beforeEach(() => {
    mockedApiFetch.mockReset();
  });

  // テスト内容: DevLabのサイドメニューからFinance専用画面へ移動できることを確認する。
  // 必要な理由: Finance機能の入口とレイアウト分離は、後続画面を追加する前提となるため。
  test("opens the Finance Dashboard from the DevLab sidebar", async () => {
    mockAuthenticatedSession();
    const user = userEvent.setup();
    renderApp("/");

    expect(
      await screen.findByRole("heading", { name: "開発状況ダッシュボード" }),
    ).toBeInTheDocument();

    await user.click(screen.getByRole("link", { name: "Finance Dashboard" }));

    expect(await screen.findByRole("heading", { name: "Finance Overview" })).toBeInTheDocument();
    expect(
      screen.getByRole("navigation", { name: "Finance navigation" }),
    ).toBeInTheDocument();
    expect(screen.queryByLabelText("Primary navigation")).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Home" })).toHaveAttribute("aria-current", "page");
    expect(screen.getByRole("heading", { name: "口座データがありません" })).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Financeサイドバーを閉じる" }));
    expect(screen.queryByRole("navigation", { name: "Finance navigation" })).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Financeサイドバーを開く" }));
    expect(screen.getByRole("navigation", { name: "Finance navigation" })).toBeInTheDocument();
  });

  // テスト内容: Finance画面からkeyboard操作でDevLabへ戻れることを確認する。
  // 必要な理由: 専用レイアウトが利用者を閉じ込めず、pointer以外でも移動可能にするため。
  test("returns to the DevLab Board with the keyboard-accessible back link", async () => {
    mockAuthenticatedSession();
    const user = userEvent.setup();
    renderApp("/finance-lab");

    expect(await screen.findByRole("heading", { name: "Finance Overview" })).toBeInTheDocument();

    const backLink = screen.getByRole("link", { name: "DevLab Boardへ戻る" });
    backLink.focus();
    expect(backLink).toHaveFocus();
    await user.keyboard("{Enter}");

    expect(
      await screen.findByRole("heading", { name: "開発状況ダッシュボード" }),
    ).toBeInTheDocument();
  });

  // テスト内容: 未実装のFinance配下URLがFinance Overviewへ戻ることを確認する。
  // 必要な理由: 直接URLや将来routeの入力でDevLab側へ誤って脱出する挙動を防ぐため。
  test("redirects an unknown Finance route to the Finance Overview", async () => {
    mockAuthenticatedSession();
    renderApp("/finance-lab/not-implemented");

    expect(await screen.findByRole("heading", { name: "Finance Overview" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Accounts" })).toBeInTheDocument();
    expect(screen.getByText("Transactions")).toHaveAttribute("aria-disabled", "true");
  });

  // テスト内容: keyboard操作でAccounts一覧へ移動し、maskと精度を保った残高を表示できることを確認する。
  // 必要な理由: 主要routeをpointerなしで利用でき、完全番号や金額の丸めを画面へ持ち込まないため。
  test("opens the Accounts list with the keyboard and renders a masked precise balance", async () => {
    mockedApiFetch.mockImplementation(async (path) => {
      if (path === "/api/auth/me") {
        return jsonResponse({ user: authenticatedUser });
      }
      if (path === "/api/dashboard") {
        return jsonResponse({ cards: [] });
      }
      if (path === "/api/finance/summary") {
        return jsonResponse({
          summary: { accountCount: 0, balances: [], recentTransactions: [], asOf: null },
        });
      }
      if (path === "/api/finance/accounts") {
        return jsonResponse({ accounts: [financeAccount] });
      }
      throw new Error(`Unexpected API path: ${path}`);
    });
    const user = userEvent.setup();
    renderApp("/finance-lab");

    const accountsLink = await screen.findByRole("link", { name: "Accounts" });
    accountsLink.focus();
    expect(accountsLink).toHaveFocus();
    await user.keyboard("{Enter}");

    expect(await screen.findByRole("heading", { name: "Accounts" })).toBeInTheDocument();
    expect(screen.getByText("•••• 1234")).toBeInTheDocument();
    expect(screen.getByText("￥9,007,199,254,740,993")).toBeInTheDocument();
    expect(accountsLink).toHaveAttribute("aria-current", "page");
  });

  // テスト内容: Account DetailのURLを直接開き、一覧取得を経由せず対象口座を表示することを確認する。
  // 必要な理由: 再読込・共有URL・ブラウザ履歴でも現在画面を維持する要件を守るため。
  test("opens an Account Detail directly by public account ID", async () => {
    mockedApiFetch.mockImplementation(async (path) => {
      if (path === "/api/auth/me") {
        return jsonResponse({ user: authenticatedUser });
      }
      if (path === "/api/dashboard") {
        return jsonResponse({ cards: [] });
      }
      if (path === `/api/finance/accounts/${financeAccount.id}`) {
        return jsonResponse({ account: financeAccount, recentTransactions: [] });
      }
      if (path === "/api/finance/summary") {
        return jsonResponse({
          summary: { accountCount: 0, balances: [], recentTransactions: [], asOf: null },
        });
      }
      throw new Error(`Unexpected API path: ${path}`);
    });
    renderApp(`/finance-lab/accounts/${financeAccount.id}`);

    expect(await screen.findByRole("heading", { name: "Synthetic Checking" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Accounts" })).toHaveAttribute("aria-current", "page");
    expect(
      mockedApiFetch.mock.calls.some(([path]) => path === "/api/finance/accounts"),
    ).toBe(false);
  });

  // テスト内容: Finance専用画面から既存sessionを終了できることを確認する。
  // 必要な理由: レイアウト分離後も共通の認証操作を失わないようにするため。
  test("logs out from the Finance Dashboard", async () => {
    mockAuthenticatedSession();
    mockedApiFetch.mockImplementation(async (path) => {
      if (path === "/api/auth/me") {
        return jsonResponse({ user: authenticatedUser });
      }
      if (path === "/api/dashboard") {
        return jsonResponse({ cards: [] });
      }
      if (path === "/api/finance/summary") {
        return jsonResponse({
          summary: {
            accountCount: 0,
            balances: [],
            recentTransactions: [],
            asOf: null,
          },
        });
      }
      if (path === "/api/auth/logout") {
        return jsonResponse({});
      }

      throw new Error(`Unexpected API path: ${path}`);
    });
    const user = userEvent.setup();
    renderApp("/finance-lab");

    await user.click(await screen.findByRole("button", { name: "Logout" }));

    expect(await screen.findByRole("heading", { name: "ログイン" })).toBeInTheDocument();
    expect(mockedApiFetch).toHaveBeenCalledWith("/api/auth/logout", { method: "POST" });
  });

  // テスト内容: logout API失敗時も進行中取得を中断し、Finance query cacheを破棄してログインへ戻ることを確認する。
  // 必要な理由: network障害時に残高や口座情報がsession終了後のmemoryへ残ることを防ぐため。
  test("clears Finance queries and aborts in-flight data when logout fails", async () => {
    let financeSignal: AbortSignal | null = null;
    mockedApiFetch.mockImplementation(async (path, init) => {
      if (path === "/api/auth/me") {
        return jsonResponse({ user: authenticatedUser });
      }
      if (path === "/api/dashboard") {
        return jsonResponse({ cards: [] });
      }
      if (path === "/api/finance/summary") {
        financeSignal = init?.signal ?? null;
        return new Promise<Response>(() => undefined);
      }
      if (path === "/api/auth/logout") {
        throw new TypeError("Failed to fetch");
      }
      throw new Error(`Unexpected API path: ${path}`);
    });
    const user = userEvent.setup();
    const queryClient = createAppQueryClient();
    queryClient.setQueryData(financeQueryKeys.accounts(), { accounts: [financeAccount] });
    renderApp("/finance-lab", queryClient);

    expect(await screen.findByRole("status")).toHaveTextContent("Finance概要を読み込んでいます");
    await user.click(screen.getByRole("button", { name: "Logout" }));

    expect(await screen.findByRole("heading", { name: "ログイン" })).toBeInTheDocument();
    expect(financeSignal).not.toBeNull();
    expect(financeSignal?.aborted).toBe(true);
    expect(queryClient.getQueriesData({ queryKey: financeQueryKeys.all })).toEqual([]);
  });

  // テスト内容: 未認証のFinance直接アクセスで既存の認証画面を表示することを確認する。
  // 必要な理由: Finance routeが既存session境界を迂回して表示される回帰を防ぐため。
  test("keeps the existing authentication boundary for direct Finance access", async () => {
    mockedApiFetch.mockResolvedValue(jsonResponse({ error: { code: "unauthorized" } }, 401));
    renderApp("/finance-lab");

    expect(await screen.findByRole("heading", { name: "ログイン" })).toBeInTheDocument();
    expect(screen.queryByLabelText("Finance navigation")).not.toBeInTheDocument();
  });

  // テスト内容: Finance Summary取得中に支援技術へloading状態を通知することを確認する。
  // 必要な理由: 認証完了後もデータ取得待ちをempty表示と誤認させないため。
  test("announces the Finance summary loading state", async () => {
    let resolveSummary: ((response: Response) => void) | undefined;
    mockedApiFetch.mockImplementation(async (path) => {
      if (path === "/api/auth/me") {
        return jsonResponse({ user: authenticatedUser });
      }
      if (path === "/api/dashboard") {
        return jsonResponse({ cards: [] });
      }
      if (path === "/api/finance/summary") {
        return new Promise<Response>((resolve) => {
          resolveSummary = resolve;
        });
      }
      throw new Error(`Unexpected API path: ${path}`);
    });
    renderApp("/finance-lab");

    expect(await screen.findByRole("status")).toHaveTextContent("Finance概要を読み込んでいます");

    resolveSummary?.(
      jsonResponse({
        summary: { accountCount: 0, balances: [], recentTransactions: [], asOf: null },
      }),
    );
  });

  // テスト内容: Summary API失敗後にretryしてempty stateへ回復できることを確認する。
  // 必要な理由: 一時的なDB・network障害で画面全体が操作不能になる回帰を防ぐため。
  test("retries the Finance summary after an API error", async () => {
    let summaryRequests = 0;
    mockedApiFetch.mockImplementation(async (path) => {
      if (path === "/api/auth/me") {
        return jsonResponse({ user: authenticatedUser });
      }
      if (path === "/api/dashboard") {
        return jsonResponse({ cards: [] });
      }
      if (path === "/api/finance/summary") {
        summaryRequests += 1;
        return summaryRequests === 1
          ? jsonResponse({ error: { code: "finance_summary_unavailable" } }, 500)
          : jsonResponse({
              summary: { accountCount: 0, balances: [], recentTransactions: [], asOf: null },
            });
      }
      throw new Error(`Unexpected API path: ${path}`);
    });
    const user = userEvent.setup();
    renderApp("/finance-lab");

    expect(await screen.findByRole("alert")).toHaveTextContent("Finance概要を取得できませんでした");
    await user.click(screen.getByRole("button", { name: "再試行" }));

    expect(await screen.findByRole("heading", { name: "口座データがありません" })).toBeInTheDocument();
    expect(summaryRequests).toBe(2);
  });

  // テスト内容: Finance Summaryのnetwork失敗後もretry stateから回復できることを確認する。
  // 必要な理由: HTTP responseを持たない通信失敗を認証・権限エラーと誤分類する回帰を防ぐため。
  test("retries the Finance summary after a network error", async () => {
    let summaryRequests = 0;
    mockedApiFetch.mockImplementation(async (path) => {
      if (path === "/api/auth/me") {
        return jsonResponse({ user: authenticatedUser });
      }
      if (path === "/api/dashboard") {
        return jsonResponse({ cards: [] });
      }
      if (path === "/api/finance/summary") {
        summaryRequests += 1;
        if (summaryRequests === 1) {
          throw new TypeError("Failed to fetch");
        }
        return jsonResponse({
          summary: { accountCount: 0, balances: [], recentTransactions: [], asOf: null },
        });
      }
      throw new Error(`Unexpected API path: ${path}`);
    });
    const user = userEvent.setup();
    renderApp("/finance-lab");

    expect(await screen.findByRole("alert")).toHaveTextContent("Finance概要を取得できませんでした");
    await user.click(screen.getByRole("button", { name: "再試行" }));

    expect(await screen.findByRole("heading", { name: "口座データがありません" })).toBeInTheDocument();
    expect(summaryRequests).toBe(2);
  });

  // テスト内容: Finance Summaryの初回取得で401になった場合にsessionを破棄してログイン画面へ戻ることを確認する。
  // 必要な理由: 認証確認後にsessionが期限切れになっても、復旧不能な一般エラー画面へ利用者を留めないため。
  test("returns to login when the initial Finance summary request is unauthorized", async () => {
    mockedApiFetch.mockImplementation(async (path) => {
      if (path === "/api/auth/me") {
        return jsonResponse({ user: authenticatedUser });
      }
      if (path === "/api/dashboard") {
        return jsonResponse({ cards: [] });
      }
      if (path === "/api/finance/summary") {
        return jsonResponse(
          { error: { code: "session_required", message: "Authentication required." } },
          401,
        );
      }
      throw new Error(`Unexpected API path: ${path}`);
    });
    renderApp("/finance-lab");

    expect(await screen.findByRole("heading", { name: "ログイン" })).toBeInTheDocument();
    expect(screen.queryByLabelText("Finance navigation")).not.toBeInTheDocument();
  });

  // テスト内容: Finance APIの401で既存のFinance query cacheを破棄してログインへ戻ることを確認する。
  // 必要な理由: session失効後に前ユーザーの口座情報が再表示される回帰を防ぐため。
  test("clears all Finance queries when the session expires", async () => {
    mockedApiFetch.mockImplementation(async (path) => {
      if (path === "/api/auth/me") {
        return jsonResponse({ user: authenticatedUser });
      }
      if (path === "/api/dashboard") {
        return jsonResponse({ cards: [] });
      }
      if (path === "/api/finance/summary") {
        return jsonResponse({ error: { code: "unauthorized" } }, 401);
      }
      throw new Error(`Unexpected API path: ${path}`);
    });
    const queryClient = createAppQueryClient();
    queryClient.setQueryData(financeQueryKeys.accounts(), { accounts: [financeAccount] });
    renderApp("/finance-lab", queryClient);

    expect(await screen.findByRole("heading", { name: "ログイン" })).toBeInTheDocument();
    expect(queryClient.getQueriesData({ queryKey: financeQueryKeys.all })).toEqual([]);
  });

  // テスト内容: Accounts一覧の401でApp認証状態を破棄してログイン画面へ戻ることを確認する。
  // 必要な理由: Summary以外のFinance routeでも同じsession境界を維持するため。
  test("returns to login when the Accounts list request is unauthorized", async () => {
    mockedApiFetch.mockImplementation(async (path) => {
      if (path === "/api/auth/me") {
        return jsonResponse({ user: authenticatedUser });
      }
      if (path === "/api/dashboard") {
        return jsonResponse({ cards: [] });
      }
      if (path === "/api/finance/accounts") {
        return jsonResponse({ error: { code: "unauthorized" } }, 401);
      }
      throw new Error(`Unexpected API path: ${path}`);
    });

    renderApp("/finance-lab/accounts");

    expect(await screen.findByRole("heading", { name: "ログイン" })).toBeInTheDocument();
    expect(screen.queryByLabelText("Finance navigation")).not.toBeInTheDocument();
  });

  // テスト内容: Account Detailの401でApp認証状態を破棄してログイン画面へ戻ることを確認する。
  // 必要な理由: 公開IDを含む直接URLでも認証切れ後の口座情報を残さないため。
  test("returns to login when the Account Detail request is unauthorized", async () => {
    mockedApiFetch.mockImplementation(async (path) => {
      if (path === "/api/auth/me") {
        return jsonResponse({ user: authenticatedUser });
      }
      if (path === "/api/dashboard") {
        return jsonResponse({ cards: [] });
      }
      if (path === `/api/finance/accounts/${financeAccount.id}`) {
        return jsonResponse({ error: { code: "unauthorized" } }, 401);
      }
      throw new Error(`Unexpected API path: ${path}`);
    });

    renderApp(`/finance-lab/accounts/${financeAccount.id}`);

    expect(await screen.findByRole("heading", { name: "ログイン" })).toBeInTheDocument();
    expect(screen.queryByLabelText("Finance navigation")).not.toBeInTheDocument();
  });

  // テスト内容: Finance Summaryの初回取得で403になった場合にpermission deniedを表示することを確認する。
  // 必要な理由: 権限不足を一時障害と誤認させ、無効な再試行を案内する回帰を防ぐため。
  test("shows permission denied when the initial Finance summary request is forbidden", async () => {
    mockedApiFetch.mockImplementation(async (path) => {
      if (path === "/api/auth/me") {
        return jsonResponse({ user: authenticatedUser });
      }
      if (path === "/api/dashboard") {
        return jsonResponse({ cards: [] });
      }
      if (path === "/api/finance/summary") {
        return jsonResponse(
          { error: { code: "finance_summary_forbidden", message: "Permission denied." } },
          403,
        );
      }
      throw new Error(`Unexpected API path: ${path}`);
    });
    renderApp("/finance-lab");

    expect(await screen.findByRole("alert")).toHaveTextContent("Finance概要を表示する権限がありません");
    expect(screen.queryByRole("button", { name: "再試行" })).not.toBeInTheDocument();
  });

  // テスト内容: 一時障害からの再試行が401になった場合もsessionを破棄してログイン画面へ戻ることを確認する。
  // 必要な理由: 初回取得と再試行で認証エラーの扱いが分岐し、期限切れsessionが残る回帰を防ぐため。
  test("returns to login when a retried Finance summary request is unauthorized", async () => {
    let summaryRequests = 0;
    mockedApiFetch.mockImplementation(async (path) => {
      if (path === "/api/auth/me") {
        return jsonResponse({ user: authenticatedUser });
      }
      if (path === "/api/dashboard") {
        return jsonResponse({ cards: [] });
      }
      if (path === "/api/finance/summary") {
        summaryRequests += 1;
        return summaryRequests === 1
          ? jsonResponse({ error: { code: "finance_summary_unavailable" } }, 500)
          : jsonResponse({ error: { code: "session_required" } }, 401);
      }
      throw new Error(`Unexpected API path: ${path}`);
    });
    const user = userEvent.setup();
    renderApp("/finance-lab");

    await user.click(await screen.findByRole("button", { name: "再試行" }));

    expect(await screen.findByRole("heading", { name: "ログイン" })).toBeInTheDocument();
    expect(summaryRequests).toBe(2);
  });

  // テスト内容: 一時障害からの再試行が403になった場合もpermission deniedを表示することを確認する。
  // 必要な理由: 再試行経路だけ権限不足を一般エラーへ戻してしまう状態遷移の分岐漏れを防ぐため。
  test("shows permission denied when a retried Finance summary request is forbidden", async () => {
    let summaryRequests = 0;
    mockedApiFetch.mockImplementation(async (path) => {
      if (path === "/api/auth/me") {
        return jsonResponse({ user: authenticatedUser });
      }
      if (path === "/api/dashboard") {
        return jsonResponse({ cards: [] });
      }
      if (path === "/api/finance/summary") {
        summaryRequests += 1;
        return summaryRequests === 1
          ? jsonResponse({ error: { code: "finance_summary_unavailable" } }, 500)
          : jsonResponse({ error: { code: "finance_summary_forbidden" } }, 403);
      }
      throw new Error(`Unexpected API path: ${path}`);
    });
    const user = userEvent.setup();
    renderApp("/finance-lab");

    await user.click(await screen.findByRole("button", { name: "再試行" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("Finance概要を表示する権限がありません");
    expect(screen.queryByRole("button", { name: "再試行" })).not.toBeInTheDocument();
    expect(summaryRequests).toBe(2);
  });

  // テスト内容: 通貨・口座数・最近の取引をAPI contractどおり表示することを確認する。
  // 必要な理由: minor unitを誤った桁で表示したり、公開DTOの変更を見逃したりしないため。
  test("renders balances and recent transactions from the Finance summary", async () => {
    mockedApiFetch.mockImplementation(async (path) => {
      if (path === "/api/auth/me") {
        return jsonResponse({ user: authenticatedUser });
      }
      if (path === "/api/dashboard") {
        return jsonResponse({ cards: [] });
      }
      if (path === "/api/finance/summary") {
        return jsonResponse({
          summary: {
            accountCount: 2,
            balances: [
              { currency: "JPY", currentAmountMinor: "180000", availableAmountMinor: "165000" },
            ],
            recentTransactions: [
              {
                id: "transaction-1",
                accountId: "account-1",
                accountName: "Synthetic Checking",
                name: "Grocery Store",
                merchant: "Sample Market",
                amountMinor: "4200",
                currency: "JPY",
                direction: "debit",
                status: "posted",
                category: { code: "groceries", label: "食料品" },
                occurredAt: "2026-08-11T01:00:00Z",
              },
            ],
            asOf: "2026-08-11T01:05:00Z",
          },
        });
      }
      throw new Error(`Unexpected API path: ${path}`);
    });
    renderApp("/finance-lab");

    expect(await screen.findByRole("heading", { name: "残高概要" })).toBeInTheDocument();
    expect(screen.getByText("￥180,000")).toBeInTheDocument();
    expect(screen.getByText("2口座")).toBeInTheDocument();
    expect(screen.getByText("Grocery Store")).toBeInTheDocument();
    expect(screen.getByText("-￥4,200")).toBeInTheDocument();
  });

  // テスト内容: 安全整数上限を超える0・2・3桁通貨と負数残高をminor unit文字列の全桁を保って表示することを確認する。
  // 必要な理由: 金額をJavaScript numberへ変換して残高や取引金額の下位桁を失う回帰を防ぐため。
  test("renders exact large and signed amounts without number conversion", async () => {
    mockedApiFetch.mockImplementation(async (path) => {
      if (path === "/api/auth/me") {
        return jsonResponse({ user: authenticatedUser });
      }
      if (path === "/api/dashboard") {
        return jsonResponse({ cards: [] });
      }
      if (path === "/api/finance/summary") {
        return jsonResponse({
          summary: {
            accountCount: 4,
            balances: [
              {
                currency: "JPY",
                currentAmountMinor: "9007199254740993",
                availableAmountMinor: "-12345",
              },
              {
                currency: "USD",
                currentAmountMinor: "9007199254740993",
                availableAmountMinor: null,
              },
              {
                currency: "KWD",
                currentAmountMinor: "9007199254740993",
                availableAmountMinor: null,
              },
            ],
            recentTransactions: [],
            asOf: "2026-08-11T01:05:00Z",
          },
        });
      }
      throw new Error(`Unexpected API path: ${path}`);
    });
    renderApp("/finance-lab");

    expect(await screen.findByText("￥9,007,199,254,740,993")).toBeInTheDocument();
    expect(screen.getByText("Available: -￥12,345")).toBeInTheDocument();
    expect(screen.getByText("$90,071,992,547,409.93")).toBeInTheDocument();
    expect(screen.getByText("KWD 9,007,199,254,740.993")).toBeInTheDocument();
  });
});
