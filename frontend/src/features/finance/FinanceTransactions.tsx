import { useQuery } from "@tanstack/react-query";
import { useSearchParams } from "react-router-dom";

import {
  FINANCE_TRANSACTIONS_PAGE_SIZE,
  FinanceAccount,
  FinanceApiError,
  FinanceCategory,
  FinanceTransaction,
  FinanceTransactionQuery,
  financeQueryKeys,
  getFinanceAccounts,
  getFinanceCategories,
  getFinanceTransactions,
} from "./financeApi";
import { formatAccountMask, formatFinanceDateTime, formatFinanceMoney } from "./financeFormat";
import { useFinanceUnauthorized } from "./useFinanceUnauthorized";

type FinanceTransactionsProps = {
  onLogout: () => Promise<void>;
  onSessionExpired: () => void;
};

const FINANCE_TRANSACTION_SEARCH_PARAM_KEYS = [
  "account_id",
  "date_from",
  "date_to",
  "category",
  "direction",
  "status",
  "sort",
  "cursor",
] as const;

const SUPPORTED_FINANCE_TRANSACTION_SEARCH_PARAM_KEYS: ReadonlySet<string> = new Set(
  FINANCE_TRANSACTION_SEARCH_PARAM_KEYS,
);

export function FinanceTransactions({ onLogout, onSessionExpired }: FinanceTransactionsProps) {
  const [searchParams, setSearchParams] = useSearchParams();
  const isTransactionURLValid = hasValidTransactionSearchParams(searchParams);
  const transactionQuery = transactionQueryFrom(searchParams);
  const accountsQuery = useQuery({
    queryKey: financeQueryKeys.accounts(),
    queryFn: ({ signal }) => getFinanceAccounts(signal),
  });
  const categoriesQuery = useQuery({
    queryKey: financeQueryKeys.categories(),
    queryFn: ({ signal }) => getFinanceCategories(signal),
  });
  const transactionsQuery = useQuery({
    queryKey: financeQueryKeys.transactions(transactionQuery),
    queryFn: ({ signal }) => getFinanceTransactions(transactionQuery, signal),
    enabled: isTransactionURLValid,
    // 同じ絞り込み・並び順のpage移動だけ旧行を維持し、filter変更時の誤表示を避ける。
    placeholderData: (previousData, previousQuery) => (
      hasSameTransactionCriteria(previousQuery?.queryKey, transactionQuery)
        ? previousData
        : undefined
    ),
  });
  const unauthorizedError = [
    isTransactionURLValid ? transactionsQuery.error : null,
    accountsQuery.error,
    categoriesQuery.error,
  ].find(isUnauthorizedOptionError);
  useFinanceUnauthorized(
    unauthorizedError,
    onSessionExpired,
  );

  const apiError = isTransactionURLValid && transactionsQuery.error instanceof FinanceApiError
    ? transactionsQuery.error
    : null;
  const isUnauthorized = apiError?.status === 401;
  const isForbidden = apiError?.status === 403;
  const isInvalidQuery = !isTransactionURLValid || apiError?.status === 400;
  const isPageUpdating = transactionsQuery.isFetching && !transactionsQuery.isPending;
  const hasFilters = hasActiveTransactionFilters(transactionQuery);
  const hasCustomizedCriteria = hasFilters || transactionQuery.sort !== "newest";

  function changeCriterion(name: string, value: string) {
    const next = new URLSearchParams(searchParams);
    if (value === "" || (name === "sort" && value === "newest")) {
      next.delete(name);
    } else {
      next.set(name, value);
    }
    next.delete("cursor");
    setSearchParams(next);
  }

  function resetAllCriteria(replace = false) {
    setSearchParams({}, { replace });
  }

  function showFirstPage() {
    const next = new URLSearchParams(searchParams);
    next.delete("cursor");
    setSearchParams(next);
  }

  function showNextPage(nextCursor: string) {
    const next = new URLSearchParams(searchParams);
    next.set("cursor", nextCursor);
    setSearchParams(next);
  }

  return (
    <>
      <header className="finance-page-header">
        <div>
          <p className="finance-eyebrow">Complete activity</p>
          <h2>Transactions</h2>
          <p className="finance-welcome">取引を25件ずつ表示します。</p>
        </div>
        <button className="finance-logout-button" type="button" onClick={() => void onLogout()}>
          Logout
        </button>
      </header>

      <FinanceTransactionFilters
        accounts={accountsQuery.data ?? []}
        accountsError={accountsQuery.error}
        accountsPending={accountsQuery.isPending}
        categories={categoriesQuery.data ?? []}
        categoriesError={categoriesQuery.error}
        categoriesPending={categoriesQuery.isPending}
        hasCustomizedCriteria={hasCustomizedCriteria}
        onChange={changeCriterion}
        onReset={resetAllCriteria}
        onRetryAccounts={() => void accountsQuery.refetch()}
        onRetryCategories={() => void categoriesQuery.refetch()}
        query={transactionQuery}
      />

      {isTransactionURLValid && transactionsQuery.isPending && (
        <FinanceLoading message="取引履歴を読み込んでいます" />
      )}
      {transactionsQuery.isError && isForbidden && (
        <FinanceError title="取引履歴を表示する権限がありません" />
      )}
      {isInvalidQuery && (
        <FinanceError
          title="取引条件を適用できません"
          description="絞り込み、並び順、またはページ情報が無効です。"
          actionLabel="条件をすべてリセット"
          onAction={() => resetAllCriteria(true)}
        />
      )}
      {transactionsQuery.isError && !isUnauthorized && !isForbidden && !isInvalidQuery && (
        <FinanceError
          title="取引履歴を取得できませんでした"
          description="時間をおいて、もう一度お試しください。"
          actionLabel="再試行"
          onAction={() => void transactionsQuery.refetch()}
        />
      )}

      {isTransactionURLValid && transactionsQuery.isSuccess && transactionsQuery.data.transactions.length === 0 && (
        <FinanceTransactionsEmpty
          hasCursor={transactionQuery.cursor !== null}
          hasFilters={hasFilters}
          onFirstPage={showFirstPage}
        />
      )}
      {isTransactionURLValid && transactionsQuery.isSuccess && transactionsQuery.data.transactions.length > 0 && (
        <section
          aria-busy={isPageUpdating}
          aria-labelledby="finance-transactions-title"
          className="finance-transactions-history"
        >
          <div className="finance-section-heading">
            <div>
              <p className="finance-eyebrow">Transaction history</p>
              <h3 id="finance-transactions-title">取引履歴</h3>
            </div>
            <p>1ページ最大{FINANCE_TRANSACTIONS_PAGE_SIZE}件</p>
          </div>

          {isPageUpdating && (
            <p className="finance-page-updating" role="status" aria-live="polite">
              取引履歴を更新しています
            </p>
          )}

          <div
            aria-label="取引履歴の横スクロール領域"
            className="finance-transaction-table-scroll"
            role="region"
            tabIndex={0}
          >
            <table className="finance-transaction-table">
              <caption>取引履歴一覧</caption>
              <thead>
                <tr>
                  <th scope="col">取引</th>
                  <th scope="col">口座</th>
                  <th scope="col">カテゴリー</th>
                  <th scope="col">状態</th>
                  <th scope="col">日時</th>
                  <th scope="col">金額</th>
                </tr>
              </thead>
              <tbody>
                {transactionsQuery.data.transactions.map((transaction) => (
                  <FinanceTransactionRow key={transaction.id} transaction={transaction} />
                ))}
              </tbody>
            </table>
          </div>

          <nav className="finance-pagination" aria-label="取引ページ">
            {transactionQuery.cursor !== null && (
              <button type="button" onClick={() => showFirstPage()}>
                先頭へ戻る
              </button>
            )}
            {transactionsQuery.data.nextCursor === null ? (
              <p>これ以上の取引はありません</p>
            ) : (
              <button
                disabled={isPageUpdating}
                type="button"
                onClick={() => showNextPage(transactionsQuery.data.nextCursor!)}
              >
                次の{FINANCE_TRANSACTIONS_PAGE_SIZE}件
              </button>
            )}
          </nav>
        </section>
      )}
    </>
  );
}

function FinanceTransactionFilters({
  accounts,
  accountsError,
  accountsPending,
  categories,
  categoriesError,
  categoriesPending,
  hasCustomizedCriteria,
  onChange,
  onReset,
  onRetryAccounts,
  onRetryCategories,
  query,
}: {
  accounts: FinanceAccount[];
  accountsError: unknown;
  accountsPending: boolean;
  categories: FinanceCategory[];
  categoriesError: unknown;
  categoriesPending: boolean;
  hasCustomizedCriteria: boolean;
  onChange: (name: string, value: string) => void;
  onReset: () => void;
  onRetryAccounts: () => void;
  onRetryCategories: () => void;
  query: FinanceTransactionQuery;
}) {
  const selectedAccountIsMissing = query.accountId !== null
    && !accounts.some((account) => account.id === query.accountId);
  const selectedCategoryIsMissing = query.category !== null
    && query.category !== "uncategorized"
    && !categories.some((category) => category.code === query.category);
  const accountOptionsForbidden = isForbiddenOptionError(accountsError);
  const categoryOptionsForbidden = isForbiddenOptionError(categoriesError);
  const accountOptionsUnavailable = accountsError !== null;
  const categoryOptionsUnavailable = categoriesError !== null;

  return (
    <section className="finance-transaction-filters" aria-labelledby="finance-filter-title">
      <div className="finance-section-heading">
        <div>
          <p className="finance-eyebrow">Filter activity</p>
          <h3 id="finance-filter-title">取引を絞り込む</h3>
        </div>
        {hasCustomizedCriteria && (
          <button className="finance-inline-action" type="button" onClick={() => onReset()}>
            条件をすべて解除
          </button>
        )}
      </div>

      <div className="finance-filter-grid">
        <label>
          <span>口座</span>
          <select
            disabled={accountsPending || accountOptionsUnavailable}
            onChange={(event) => onChange("account_id", event.target.value)}
            value={query.accountId ?? ""}
          >
            <option value="">すべての口座</option>
            {selectedAccountIsMissing && (
              <option value={query.accountId ?? ""}>選択中の口座</option>
            )}
            {accounts.map((account) => (
              <option key={account.id} value={account.id}>
                {account.name} {formatAccountMask(account.mask)}
              </option>
            ))}
          </select>
        </label>

        <label>
          <span>開始日</span>
          <input
            onChange={(event) => onChange("date_from", event.target.value)}
            type="date"
            value={query.dateFrom ?? ""}
          />
        </label>

        <label>
          <span>終了日</span>
          <input
            onChange={(event) => onChange("date_to", event.target.value)}
            type="date"
            value={query.dateTo ?? ""}
          />
        </label>

        <label>
          <span>カテゴリー</span>
          <select
            disabled={categoriesPending || categoryOptionsUnavailable}
            onChange={(event) => onChange("category", event.target.value)}
            value={query.category ?? ""}
          >
            <option value="">すべてのカテゴリー</option>
            <option value="uncategorized">未分類</option>
            {selectedCategoryIsMissing && (
              <option value={query.category ?? ""}>選択中のカテゴリー</option>
            )}
            {categories.map((category) => (
              <option key={category.code} value={category.code}>{category.label}</option>
            ))}
          </select>
        </label>

        <label>
          <span>入出金</span>
          <select
            onChange={(event) => onChange("direction", event.target.value)}
            value={query.direction ?? ""}
          >
            <option value="">すべて</option>
            <option value="debit">支出</option>
            <option value="credit">入金</option>
          </select>
        </label>

        <label>
          <span>状態</span>
          <select
            onChange={(event) => onChange("status", event.target.value)}
            value={query.status ?? ""}
          >
            <option value="">すべて</option>
            <option value="pending">処理中</option>
            <option value="posted">確定</option>
            <option value="reversed">取消</option>
          </select>
        </label>

        <label>
          <span>並び順</span>
          <select
            onChange={(event) => onChange("sort", event.target.value)}
            value={query.sort}
          >
            <option value="newest">新しい順</option>
            <option value="oldest">古い順</option>
          </select>
        </label>
      </div>

      {(accountsPending || categoriesPending) && (
        <p className="finance-filter-note" aria-live="polite">
          絞り込み候補を読み込んでいます
        </p>
      )}
      {accountOptionsUnavailable && !isUnauthorizedOptionError(accountsError) && (
        <div className="finance-filter-option-error">
          <p>{accountOptionsForbidden ? "口座候補を表示する権限がありません" : "口座候補を取得できませんでした"}</p>
          {!accountOptionsForbidden && (
            <button type="button" onClick={onRetryAccounts}>口座候補を再試行</button>
          )}
        </div>
      )}
      {categoryOptionsUnavailable && !isUnauthorizedOptionError(categoriesError) && (
        <div className="finance-filter-option-error">
          <p>{categoryOptionsForbidden ? "カテゴリー候補を表示する権限がありません" : "カテゴリー候補を取得できませんでした"}</p>
          {!categoryOptionsForbidden && (
            <button type="button" onClick={onRetryCategories}>カテゴリー候補を再試行</button>
          )}
        </div>
      )}
    </section>
  );
}

function transactionQueryFrom(searchParams: URLSearchParams): FinanceTransactionQuery {
  return {
    accountId: searchParams.get("account_id"),
    dateFrom: searchParams.get("date_from"),
    dateTo: searchParams.get("date_to"),
    category: searchParams.get("category"),
    direction: searchParams.get("direction"),
    status: searchParams.get("status"),
    sort: searchParams.get("sort") ?? "newest",
    cursor: searchParams.get("cursor"),
  };
}

function hasValidTransactionSearchParams(searchParams: URLSearchParams) {
  const seenKeys = new Set<string>();
  for (const [key, value] of searchParams) {
    if (
      !SUPPORTED_FINANCE_TRANSACTION_SEARCH_PARAM_KEYS.has(key)
      || seenKeys.has(key)
      || value === ""
    ) {
      return false;
    }
    seenKeys.add(key);
  }
  return true;
}

function hasActiveTransactionFilters(query: FinanceTransactionQuery) {
  return [
    query.accountId,
    query.dateFrom,
    query.dateTo,
    query.category,
    query.direction,
    query.status,
  ].some((value) => value !== null);
}

function hasSameTransactionCriteria(
  previousKey: readonly unknown[] | undefined,
  current: FinanceTransactionQuery,
) {
  const previous = previousKey?.[3] as FinanceTransactionQuery | undefined;
  return previous !== undefined
    && previous.accountId === current.accountId
    && previous.dateFrom === current.dateFrom
    && previous.dateTo === current.dateTo
    && previous.category === current.category
    && previous.direction === current.direction
    && previous.status === current.status
    && previous.sort === current.sort;
}

function isUnauthorizedOptionError(error: unknown) {
  return error instanceof FinanceApiError && error.status === 401;
}

function isForbiddenOptionError(error: unknown) {
  return error instanceof FinanceApiError && error.status === 403;
}

function FinanceTransactionRow({ transaction }: { transaction: FinanceTransaction }) {
  const directionLabel = transaction.direction === "debit" ? "支出" : "入金";
  const amountPrefix = transaction.direction === "debit" ? "-" : "+";

  return (
    <tr>
      <td>
        <div className="finance-transaction-name">
          <strong>{transaction.name}</strong>
          <span>{transaction.merchant ?? "未取得"}</span>
        </div>
      </td>
      <td>{transaction.accountName}</td>
      <td>{transaction.category?.label ?? "未取得"}</td>
      <td>{formatTransactionStatus(transaction.status)}</td>
      <td>
        <time dateTime={transaction.occurredAt}>
          {formatFinanceDateTime(transaction.occurredAt)}
        </time>
      </td>
      <td className={`finance-table-amount ${transaction.direction}`}>
        <div>
          <span>{directionLabel}</span>
          <strong>
            {amountPrefix}{formatFinanceMoney(transaction.amountMinor, transaction.currency)}
          </strong>
        </div>
      </td>
    </tr>
  );
}

function formatTransactionStatus(status: FinanceTransaction["status"]) {
  const labels = {
    pending: "処理中",
    posted: "確定",
    reversed: "取消",
  } satisfies Record<FinanceTransaction["status"], string>;
  return labels[status];
}

function FinanceLoading({ message }: { message: string }) {
  return (
    <section className="finance-state-panel" role="status" aria-live="polite">
      <span className="finance-loading-dot" aria-hidden="true" />
      <p>{message}</p>
    </section>
  );
}

function FinanceTransactionsEmpty({
  hasCursor,
  hasFilters,
  onFirstPage,
}: {
  hasCursor: boolean;
  hasFilters: boolean;
  onFirstPage: () => void;
}) {
  const title = hasFilters ? "条件に一致する取引がありません" : "取引がありません";
  return (
    <section className="finance-empty-state" aria-labelledby="finance-transactions-empty-title">
      <span className="finance-status-badge">EMPTY</span>
      <div>
        <h3 id="finance-transactions-empty-title">{title}</h3>
        <p>
          {hasCursor
            ? "このページ以降の取引はありません。"
            : hasFilters
              ? "絞り込み条件を変更して、もう一度お試しください。"
              : "表示できるFinance取引はまだありません。"}
        </p>
        {hasCursor && (
          <button
            className="finance-inline-action"
            type="button"
            onClick={() => onFirstPage()}
          >
            先頭へ戻る
          </button>
        )}
      </div>
    </section>
  );
}

function FinanceError({
  actionLabel,
  description = "管理者へ確認してください。",
  onAction,
  title,
}: {
  actionLabel?: string;
  description?: string;
  onAction?: () => void;
  title: string;
}) {
  return (
    <section className="finance-state-panel finance-error-panel" role="alert">
      <div>
        <h3>{title}</h3>
        <p>{description}</p>
      </div>
      {onAction && actionLabel && (
        <button type="button" onClick={onAction}>
          {actionLabel}
        </button>
      )}
    </section>
  );
}
