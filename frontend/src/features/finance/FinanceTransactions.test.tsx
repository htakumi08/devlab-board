import { QueryClientProvider } from "@tanstack/react-query";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  MemoryRouter,
  Route,
  Routes,
  useLocation,
  useNavigate,
} from "react-router-dom";
import { beforeEach, describe, expect, test, vi } from "vitest";

import { apiFetch } from "../../lib/api";
import { createAppQueryClient } from "../../lib/queryClient";
import { FinanceTransactions } from "./FinanceTransactions";

vi.mock("../../lib/api", () => ({
  apiFetch: vi.fn(),
}));

const mockedApiFetch = vi.mocked(apiFetch);

const debitTransaction = {
  id: "024c4ea1-e906-4f2a-a32b-f70f95762f76",
  accountId: "370fdd2d-aeb9-492d-a981-c40923531411",
  accountName: "Synthetic Checking",
  name: "Synthetic Grocery",
  merchant: "Synthetic Market",
  amountMinor: "9007199254740993",
  currency: "JPY",
  direction: "debit" as const,
  status: "posted" as const,
  category: { code: "groceries", label: "食料品" },
  occurredAt: "2026-08-11T01:05:00Z",
};

const creditTransaction = {
  ...debitTransaction,
  id: "ee266610-8122-4ec2-8506-6883f4d4d367",
  name: "Synthetic Salary",
  merchant: null,
  amountMinor: "4200",
  direction: "credit" as const,
  status: "pending" as const,
  category: null,
};

const reversedTransaction = {
  ...debitTransaction,
  id: "23d583b8-d464-4512-a681-033125e745cd",
  name: "Synthetic Reversal",
  merchant: "Synthetic Transit",
  amountMinor: "1200",
  status: "reversed" as const,
  category: { code: "transportation", label: "交通" },
};

const account = {
  id: debitTransaction.accountId,
  name: "Synthetic Checking",
  accountType: "checking" as const,
  mask: "4242",
  currency: "JPY",
  currentAmountMinor: "100000",
  availableAmountMinor: "80000",
  status: "active" as const,
  balanceAsOf: "2026-08-11T01:05:00Z",
};

const categories = [
  { code: "groceries", label: "食料品" },
  { code: "transportation", label: "交通" },
];

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function optionResponse(path: string) {
  if (path === "/api/finance/accounts") {
    return jsonResponse({ accounts: [account] });
  }
  if (path === "/api/finance/categories") {
    return jsonResponse({ categories });
  }
  return null;
}

function mockTransactionsResponse(body: unknown, status = 200) {
  mockedApiFetch.mockImplementation(async (path) => (
    optionResponse(path) ?? jsonResponse(body, status)
  ));
}

function LocationControls() {
  const location = useLocation();
  const navigate = useNavigate();
  return (
    <div>
      <div aria-label="現在URL">{location.pathname}{location.search}</div>
      <button type="button" onClick={() => navigate(-1)}>ブラウザ履歴を戻る</button>
    </div>
  );
}

function renderTransactions(initialPath = "/finance-lab/transactions", onSessionExpired = vi.fn()) {
  const queryClient = createAppQueryClient();
  const onLogout = vi.fn(async () => undefined);
  const result = render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[initialPath]}>
        <LocationControls />
        <Routes>
          <Route
            path="/finance-lab/transactions"
            element={
              <FinanceTransactions
                onLogout={onLogout}
                onSessionExpired={onSessionExpired}
              />
            }
          />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
  return { ...result, onLogout, onSessionExpired, queryClient };
}

describe("Finance Transactions", () => {
  beforeEach(() => {
    mockedApiFetch.mockReset();
  });

  // テスト内容: 取引履歴の初回取得中にloading状態を支援技術へ通知することを確認する。
  // 必要な理由: 正常な空一覧と通信待ちを利用者が区別できるようにするため。
  test("announces the initial loading state", async () => {
    mockedApiFetch.mockImplementation((path) => (
      optionResponse(path) ?? new Promise<Response>(() => undefined)
    ));

    renderTransactions();

    expect(await screen.findByRole("status")).toHaveTextContent("取引履歴を読み込んでいます");
  });

  // テスト内容: Transactions画面のLogout操作を既存session終了callbackへ渡すことを確認する。
  // 必要な理由: Financeのserver stateを既存App境界で破棄するlogout導線を各画面で維持するため。
  test("forwards the Logout action to the session boundary", async () => {
    mockTransactionsResponse({ transactions: [], nextCursor: null });
    const user = userEvent.setup();
    const { onLogout } = renderTransactions();

    await user.click(screen.getByRole("button", { name: "Logout" }));

    expect(onLogout).toHaveBeenCalledTimes(1);
  });

  // テスト内容: semantic tableに取引全項目、nullable値、状態・方向、大整数金額を文字で表示することを確認する。
  // 必要な理由: 金融情報を色やJavaScript numberだけに依存せず正確に読み取れるようにするため。
  test("renders complete Transactions in an accessible table", async () => {
    mockTransactionsResponse({
      transactions: [debitTransaction, creditTransaction, reversedTransaction],
      nextCursor: null,
    });

    renderTransactions();

    const table = await screen.findByRole("table", { name: "取引履歴一覧" });
    expect(table).toBeInTheDocument();
    for (const header of ["取引", "口座", "カテゴリー", "状態", "日時", "金額"]) {
      expect(screen.getByRole("columnheader", { name: header })).toBeInTheDocument();
    }
    expect(within(table).getByText("Synthetic Grocery")).toBeInTheDocument();
    expect(within(table).getByText("Synthetic Market")).toBeInTheDocument();
    expect(within(table).getAllByText("Synthetic Checking")).toHaveLength(3);
    expect(within(table).getByText("食料品")).toBeInTheDocument();
    expect(within(table).getByText("確定")).toBeInTheDocument();
    expect(within(table).getByText("処理中")).toBeInTheDocument();
    expect(within(table).getByText("取消")).toBeInTheDocument();
    expect(within(table).getAllByText("支出")).toHaveLength(2);
    expect(within(table).getByText("入金")).toBeInTheDocument();
    expect(within(table).getByText("-￥9,007,199,254,740,993")).toBeInTheDocument();
    expect(within(table).getByText("+￥4,200")).toBeInTheDocument();
    expect(within(table).getAllByText("未取得")).toHaveLength(2);
    expect(screen.getByRole("navigation", { name: "取引ページ" })).toHaveTextContent(
      "これ以上の取引はありません",
    );
  });

  // テスト内容: URLで指定した取引条件をlabel付きのnative controlへ復元することを確認する。
  // 必要な理由: URLを適用済みfilterの正として、直接URL・再読込・keyboard操作を同じ状態へ揃えるため。
  test("restores labeled transaction filters from the URL", async () => {
    mockTransactionsResponse({ transactions: [debitTransaction], nextCursor: null });

    renderTransactions(
      "/finance-lab/transactions?account_id=370fdd2d-aeb9-492d-a981-c40923531411&date_from=2026-08-01&date_to=2026-08-11&category=groceries&direction=debit&status=posted&sort=oldest",
    );

    expect(await screen.findByRole("combobox", { name: "口座" })).toHaveValue(
      "370fdd2d-aeb9-492d-a981-c40923531411",
    );
    expect(screen.getByLabelText("開始日")).toHaveValue("2026-08-01");
    expect(screen.getByLabelText("終了日")).toHaveValue("2026-08-11");
    expect(screen.getByRole("combobox", { name: "カテゴリー" })).toHaveValue("groceries");
    expect(screen.getByRole("combobox", { name: "入出金" })).toHaveValue("debit");
    expect(screen.getByRole("combobox", { name: "状態" })).toHaveValue("posted");
    expect(screen.getByRole("combobox", { name: "並び順" })).toHaveValue("oldest");
    expect(screen.getByRole("option", { name: "未分類" })).toHaveValue("uncategorized");
    expect(mockedApiFetch).toHaveBeenCalledWith(
      "/api/finance/transactions?account_id=370fdd2d-aeb9-492d-a981-c40923531411&date_from=2026-08-01&date_to=2026-08-11&category=groceries&direction=debit&status=posted&sort=oldest&limit=25",
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    );
  });

  // テスト内容: 未対応keyを含む直接URLをAPIへ送らず、不正条件としてリセットできることを確認する。
  // 必要な理由: frontendとbackendで許可queryの解釈がずれたURLを、取引APIへ到達する前に遮断するため。
  test("rejects an unknown direct URL key before requesting Transactions", async () => {
    mockedApiFetch.mockImplementation(async (path) => (
      optionResponse(path) ?? jsonResponse({ transactions: [], nextCursor: null })
    ));
    const user = userEvent.setup();

    renderTransactions("/finance-lab/transactions?category=groceries&unexpected=value");

    expect(await screen.findByRole("alert")).toHaveTextContent("取引条件を適用できません");
    expect(mockedApiFetch).toHaveBeenCalledWith(
      "/api/finance/accounts",
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    );
    expect(mockedApiFetch).toHaveBeenCalledWith(
      "/api/finance/categories",
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    );
    expect(mockedApiFetch.mock.calls.some(([path]) => (
      path.startsWith("/api/finance/transactions")
    ))).toBe(false);

    await user.click(screen.getByRole("button", { name: "条件をすべてリセット" }));

    expect(await screen.findByRole("heading", { name: "取引がありません" })).toBeInTheDocument();
    expect(screen.getByLabelText("現在URL")).toHaveTextContent("/finance-lab/transactions");
  });

  // テスト内容: 同じkeyを複数持つ直接URLを、先頭値だけへ縮退させず不正として扱うことを確認する。
  // 必要な理由: duplicate queryの解釈差により、画面表示とAPIへ適用される条件が食い違う回帰を防ぐため。
  test("rejects a duplicate direct URL key before requesting Transactions", async () => {
    mockedApiFetch.mockImplementation(async (path) => (
      optionResponse(path) ?? jsonResponse({ transactions: [], nextCursor: null })
    ));

    renderTransactions("/finance-lab/transactions?sort=oldest&sort=newest");

    expect(await screen.findByRole("alert")).toHaveTextContent("取引条件を適用できません");
    expect(mockedApiFetch.mock.calls.some(([path]) => (
      path.startsWith("/api/finance/transactions")
    ))).toBe(false);
  });

  // テスト内容: 値が空の対応keyを未指定へ読み替えず、不正な直接URLとして扱うことを確認する。
  // 必要な理由: backendが400とする空値をfrontendが削除済み条件として誤表示する回帰を防ぐため。
  test("rejects an empty direct URL value before requesting Transactions", async () => {
    mockedApiFetch.mockImplementation(async (path) => (
      optionResponse(path) ?? jsonResponse({ transactions: [], nextCursor: null })
    ));

    renderTransactions("/finance-lab/transactions?status=");

    expect(await screen.findByRole("alert")).toHaveTextContent("取引条件を適用できません");
    expect(mockedApiFetch.mock.calls.some(([path]) => (
      path.startsWith("/api/finance/transactions")
    ))).toBe(false);
  });

  // テスト内容: filter変更が他条件を維持してcursorだけを削除し、新しい条件をURLへpushすることを確認する。
  // 必要な理由: 旧条件のcursorを新条件へ流用せず、Browser Backで変更前のページを復元するため。
  test("resets only the cursor when a filter changes", async () => {
    mockTransactionsResponse({ transactions: [debitTransaction], nextCursor: null });
    const user = userEvent.setup();
    renderTransactions(
      `/finance-lab/transactions?account_id=${account.id}&category=groceries&sort=oldest&cursor=old-page`,
    );

    await user.selectOptions(
      await screen.findByRole("combobox", { name: "状態" }),
      "posted",
    );

    expect(screen.getByLabelText("現在URL")).toHaveTextContent(
      `/finance-lab/transactions?account_id=${account.id}&category=groceries&sort=oldest&status=posted`,
    );
    expect(screen.getByLabelText("現在URL")).not.toHaveTextContent("cursor=");
    expect(mockedApiFetch).toHaveBeenCalledWith(
      `/api/finance/transactions?account_id=${account.id}&category=groceries&status=posted&sort=oldest&limit=25`,
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    );
    await user.click(screen.getByRole("button", { name: "ブラウザ履歴を戻る" }));
    expect(screen.getByLabelText("現在URL")).toHaveTextContent(
      `/finance-lab/transactions?account_id=${account.id}&category=groceries&sort=oldest&cursor=old-page`,
    );
  });

  // テスト内容: filter変更中は旧条件の行をplaceholderとして表示しないことを確認する。
  // 必要な理由: 新しい条件に一致しない金融取引を結果として誤認させないため。
  test("hides stale rows while a changed filter is loading", async () => {
    let resolveFiltered: ((response: Response) => void) | undefined;
    mockedApiFetch.mockImplementation(async (path) => {
      const options = optionResponse(path);
      if (options) {
        return options;
      }
      if (!path.includes("status=pending")) {
        return jsonResponse({ transactions: [debitTransaction], nextCursor: null });
      }
      return new Promise<Response>((resolve) => {
        resolveFiltered = resolve;
      });
    });
    const user = userEvent.setup();
    renderTransactions();

    await user.selectOptions(
      await screen.findByRole("combobox", { name: "状態" }),
      "pending",
    );

    expect(screen.queryByText("Synthetic Grocery")).not.toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveTextContent("取引履歴を読み込んでいます");
    resolveFiltered?.(jsonResponse({ transactions: [creditTransaction], nextCursor: null }));
    expect(await screen.findByText("Synthetic Salary")).toBeInTheDocument();
  });

  // テスト内容: 先頭ページが0件の場合に取引なしのempty stateを表示することを確認する。
  // 必要な理由: 正常な0件をAPI失敗や最終ページと混同しないため。
  test("renders an empty state for a user without Transactions", async () => {
    mockTransactionsResponse({ transactions: [], nextCursor: null });

    renderTransactions();

    expect(await screen.findByRole("heading", { name: "取引がありません" })).toBeInTheDocument();
  });

  // テスト内容: filter適用後の0件を通常の取引0件と区別し、条件解除を提示することを確認する。
  // 必要な理由: データ自体がない状態と検索条件に一致しない状態を誤案内しないため。
  test("offers to clear filters when no Transactions match", async () => {
    mockTransactionsResponse({ transactions: [], nextCursor: null });
    const user = userEvent.setup();
    renderTransactions("/finance-lab/transactions?category=groceries&sort=oldest");

    expect(
      await screen.findByRole("heading", { name: "条件に一致する取引がありません" }),
    ).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "条件をすべて解除" }));
    expect(screen.getByLabelText("現在URL")).toHaveTextContent("/finance-lab/transactions");
  });

  // テスト内容: filter付き最終ページが0件の場合にfilterを残した先頭ページへ戻れることを確認する。
  // 必要な理由: cursorだけを破棄する操作で、利用者が指定した絞り込み条件を失わないため。
  test("returns to the filtered first page from an empty cursor page", async () => {
    mockTransactionsResponse({ transactions: [], nextCursor: null });
    const user = userEvent.setup();
    renderTransactions("/finance-lab/transactions?category=groceries&cursor=last-page");

    await user.click(await screen.findByRole("button", { name: "先頭へ戻る" }));

    expect(screen.getByLabelText("現在URL")).toHaveTextContent(
      "/finance-lab/transactions?category=groceries",
    );
    expect(screen.getByLabelText("現在URL")).not.toHaveTextContent("cursor=");
  });

  // テスト内容: 口座・カテゴリー候補取得の失敗が取引一覧を隠さず、局所的な再試行を提示することを確認する。
  // 必要な理由: 補助データの部分失敗で、取得済みの金融取引まで利用不能にしないため。
  test("keeps Transactions visible when filter options fail", async () => {
    let accountRequests = 0;
    let categoryRequests = 0;
    mockedApiFetch.mockImplementation(async (path) => {
      if (path === "/api/finance/accounts") {
        accountRequests += 1;
        return jsonResponse({ error: { code: "finance_options_unavailable" } }, 500);
      }
      if (path === "/api/finance/categories") {
        categoryRequests += 1;
        return jsonResponse({ error: { code: "finance_options_unavailable" } }, 500);
      }
      return jsonResponse({ transactions: [debitTransaction], nextCursor: null });
    });
    const user = userEvent.setup();

    renderTransactions();

    expect(await screen.findByText("Synthetic Grocery")).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "口座" })).toBeDisabled();
    expect(screen.getByRole("combobox", { name: "カテゴリー" })).toBeDisabled();
    await user.click(screen.getByRole("button", { name: "口座候補を再試行" }));
    await user.click(screen.getByRole("button", { name: "カテゴリー候補を再試行" }));
    expect(accountRequests).toBe(2);
    expect(categoryRequests).toBe(2);
  });

  // テスト内容: opaque cursor付き直接URLをAPIへ復元し、先頭へ戻る操作を表示することを確認する。
  // 必要な理由: 再読込・共有URLでも現在ページを維持し、cursor内容をclientで解釈しないため。
  test("restores an opaque cursor from a direct URL", async () => {
    mockTransactionsResponse({ transactions: [creditTransaction], nextCursor: null });

    renderTransactions("/finance-lab/transactions?cursor=cursor%2Fpage+2");

    expect(await screen.findByText("Synthetic Salary")).toBeInTheDocument();
    expect(mockedApiFetch).toHaveBeenCalledWith(
      "/api/finance/transactions?cursor=cursor%2Fpage+2&limit=25",
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    );
    expect(screen.getByRole("button", { name: "先頭へ戻る" })).toBeInTheDocument();
  });

  // テスト内容: 次ページcursorをURL履歴へpushし、ブラウザBackで先頭ページを復元することを確認する。
  // 必要な理由: paginationをcomponent内部stateへ閉じ込めず戻る・進む操作と整合させるため。
  test("preserves filters on the next page and restores them with browser Back", async () => {
    mockedApiFetch.mockImplementation(async (path) => {
      const options = optionResponse(path);
      if (options) {
        return options;
      }
      if (path === "/api/finance/transactions?category=groceries&sort=oldest&limit=25") {
        return jsonResponse({ transactions: [debitTransaction], nextCursor: "cursor/next" });
      }
      if (path === "/api/finance/transactions?category=groceries&sort=oldest&cursor=cursor%2Fnext&limit=25") {
        return jsonResponse({ transactions: [creditTransaction], nextCursor: null });
      }
      throw new Error(`Unexpected API path: ${path}`);
    });
    const user = userEvent.setup();
    renderTransactions("/finance-lab/transactions?category=groceries&sort=oldest");

    const nextButton = await screen.findByRole("button", { name: "次の25件" });
    nextButton.focus();
    expect(nextButton).toHaveFocus();
    await user.keyboard("{Enter}");

    expect(await screen.findByText("Synthetic Salary")).toBeInTheDocument();
    expect(screen.getByLabelText("現在URL")).toHaveTextContent(
      "/finance-lab/transactions?category=groceries&sort=oldest&cursor=cursor%2Fnext",
    );
    await user.click(screen.getByRole("button", { name: "ブラウザ履歴を戻る" }));
    expect(await screen.findByText("Synthetic Grocery")).toBeInTheDocument();
    expect(screen.getByLabelText("現在URL")).toHaveTextContent(
      "/finance-lab/transactions?category=groceries&sort=oldest",
    );
  });

  // テスト内容: 次ページ取得中に現在行を維持しつつ更新中を通知することを確認する。
  // 必要な理由: page遷移時の空白化を防ぎながら、古い行が表示中であることを明示するため。
  test("keeps the current rows while the next page is updating", async () => {
    let resolveNext: ((response: Response) => void) | undefined;
    mockedApiFetch.mockImplementation(async (path) => {
      const options = optionResponse(path);
      if (options) {
        return options;
      }
      if (path === "/api/finance/transactions?limit=25") {
        return jsonResponse({ transactions: [debitTransaction], nextCursor: "next" });
      }
      return new Promise<Response>((resolve) => {
        resolveNext = resolve;
      });
    });
    const user = userEvent.setup();
    renderTransactions();

    await user.click(await screen.findByRole("button", { name: "次の25件" }));

    expect(screen.getByText("Synthetic Grocery")).toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveTextContent("取引履歴を更新しています");
    expect(screen.getByRole("button", { name: "次の25件" })).toBeDisabled();
    resolveNext?.(jsonResponse({ transactions: [creditTransaction], nextCursor: null }));
    expect(await screen.findByText("Synthetic Salary")).toBeInTheDocument();
  });

  // テスト内容: 不正queryの400で全条件をリセットし正常表示へ復旧できることを確認する。
  // 必要な理由: 壊れたfilter・sort・cursorの共有URLから再試行を繰り返さないため。
  test("recovers from an invalid query by resetting all conditions", async () => {
    mockedApiFetch.mockImplementation(async (path) => (
      optionResponse(path) ?? (path.includes("status=invalid")
        ? jsonResponse({ error: { code: "finance_transactions_invalid_query" } }, 400)
        : jsonResponse({ transactions: [], nextCursor: null }))
    ));
    const user = userEvent.setup();
    renderTransactions("/finance-lab/transactions?status=invalid&sort=oldest&cursor=invalid");

    await user.click(await screen.findByRole("button", { name: "条件をすべてリセット" }));

    expect(await screen.findByRole("heading", { name: "取引がありません" })).toBeInTheDocument();
    expect(screen.getByLabelText("現在URL")).toHaveTextContent("/finance-lab/transactions");
  });

  // テスト内容: 403をpermission stateとして表示し、再試行を提示しないことを確認する。
  // 必要な理由: 権限不足を一時的な通信障害として誤案内しないため。
  test("renders permission denied without retry for a forbidden request", async () => {
    mockedApiFetch.mockImplementation(async (path) => (
      optionResponse(path) ?? jsonResponse({ error: { code: "forbidden" } }, 403)
    ));

    renderTransactions();

    expect(await screen.findByRole("alert")).toHaveTextContent("取引履歴を表示する権限がありません");
    expect(screen.queryByRole("button", { name: "再試行" })).not.toBeInTheDocument();
  });

  // テスト内容: APIの500を一時障害として表示し、再試行操作を提示することを確認する。
  // 必要な理由: server障害を権限不足や不正cursorと誤分類しないため。
  test("offers retry for a server failure", async () => {
    mockedApiFetch.mockImplementation(async (path) => (
      optionResponse(path)
      ?? jsonResponse({ error: { code: "finance_transactions_unavailable" } }, 500)
    ));

    renderTransactions();

    expect(await screen.findByRole("alert")).toHaveTextContent("取引履歴を取得できませんでした");
    expect(screen.getByRole("button", { name: "再試行" })).toBeInTheDocument();
  });

  // テスト内容: network失敗後に明示的な再試行でempty stateへ回復することを確認する。
  // 必要な理由: 一時障害から画面再読込なしで復旧できるようにするため。
  test("retries the Transactions request after a network failure", async () => {
    let requests = 0;
    mockedApiFetch.mockImplementation(async (path) => {
      const options = optionResponse(path);
      if (options) {
        return options;
      }
      requests += 1;
      if (requests === 1) {
        throw new TypeError("Failed to fetch");
      }
      return jsonResponse({ transactions: [], nextCursor: null });
    });
    const user = userEvent.setup();
    renderTransactions();

    await user.click(await screen.findByRole("button", { name: "再試行" }));

    expect(await screen.findByRole("heading", { name: "取引がありません" })).toBeInTheDocument();
    expect(requests).toBe(2);
  });

  // テスト内容: 401を既存session失効callbackへ渡すことを確認する。
  // 必要な理由: 取引履歴と進行中queryを認証切れ後にmemoryへ残さないため。
  test("reports an unauthorized Transactions request", async () => {
    mockedApiFetch.mockResolvedValue(jsonResponse({ error: { code: "unauthorized" } }, 401));
    const onSessionExpired = vi.fn();

    renderTransactions("/finance-lab/transactions", onSessionExpired);

    await vi.waitFor(() => expect(onSessionExpired).toHaveBeenCalledTimes(1));
    expect(screen.queryByRole("button", { name: "再試行" })).not.toBeInTheDocument();
  });
});
