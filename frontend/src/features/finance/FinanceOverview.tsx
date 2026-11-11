import { useQuery } from "@tanstack/react-query";

import {
  FinanceApiError,
  FinanceSummary,
  financeQueryKeys,
  getFinanceSummary,
} from "./financeApi";
import { formatFinanceDateTime, formatFinanceMoney } from "./financeFormat";
import { useFinanceUnauthorized } from "./useFinanceUnauthorized";

type FinanceOverviewProps = {
  onLogout: () => Promise<void>;
  onSessionExpired: () => void;
  user: {
    email: string;
    name: string;
  };
};

export function FinanceOverview({ onLogout, onSessionExpired, user }: FinanceOverviewProps) {
  const summaryQuery = useQuery({
    queryKey: financeQueryKeys.summary(),
    queryFn: ({ signal }) => getFinanceSummary(signal),
  });
  useFinanceUnauthorized(summaryQuery.error, onSessionExpired);
  const isForbidden = summaryQuery.error instanceof FinanceApiError
    && summaryQuery.error.status === 403;
  const isUnauthorized = summaryQuery.error instanceof FinanceApiError
    && summaryQuery.error.status === 401;

  return (
    <>
      <header className="finance-page-header">
        <div>
          <p className="finance-eyebrow">Personal finance workspace</p>
          <h2>Finance Overview</h2>
          <p className="finance-welcome">こんにちは、{user.name}さん</p>
        </div>
        <button className="finance-logout-button" type="button" onClick={() => void onLogout()}>
          Logout
        </button>
      </header>

      {summaryQuery.isPending && (
        <section className="finance-state-panel" role="status" aria-live="polite">
          <span className="finance-loading-dot" aria-hidden="true" />
          <p>Finance概要を読み込んでいます</p>
        </section>
      )}

      {summaryQuery.isError && !isForbidden && !isUnauthorized && (
        <section className="finance-state-panel finance-error-panel" role="alert">
          <div>
            <h3>Finance概要を取得できませんでした</h3>
            <p>時間をおいて、もう一度お試しください。</p>
          </div>
          <button type="button" onClick={() => void summaryQuery.refetch()}>
            再試行
          </button>
        </section>
      )}

      {summaryQuery.isError && isForbidden && (
        <section className="finance-state-panel finance-error-panel" role="alert">
          <div>
            <h3>Finance概要を表示する権限がありません</h3>
            <p>この機能を利用する権限について、管理者へ確認してください。</p>
          </div>
        </section>
      )}

      {summaryQuery.isSuccess && summaryQuery.data.accountCount === 0 && <FinanceEmptyState />}
      {summaryQuery.isSuccess && summaryQuery.data.accountCount > 0 && (
        <FinanceSummaryView summary={summaryQuery.data} />
      )}
    </>
  );
}

function FinanceEmptyState() {
  return (
    <section className="finance-empty-state" aria-labelledby="finance-empty-title">
      <span className="finance-status-badge">EMPTY</span>
      <div>
        <h3 id="finance-empty-title">口座データがありません</h3>
        <p>
          現在のユーザーに所属するFinance口座はまだありません。synthetic fixtureの投入機能を追加すると、残高と最近の取引をここで確認できます。
        </p>
      </div>
    </section>
  );
}

function FinanceSummaryView({ summary }: { summary: FinanceSummary }) {
  return (
    <>
      <section className="finance-summary-section" aria-labelledby="finance-balance-title">
        <div className="finance-section-heading">
          <div>
            <p className="finance-eyebrow">Current snapshot</p>
            <h3 id="finance-balance-title">残高概要</h3>
          </div>
          <p>
            {summary.asOf
              ? `残高基準日時（最古） ${formatFinanceDateTime(summary.asOf)}`
              : "残高基準日時なし"}
          </p>
        </div>

        <div className="finance-balance-grid">
          <article>
            <p>Accounts</p>
            <strong>{summary.accountCount}口座</strong>
          </article>
          {summary.balances.map((balance) => (
            <article key={balance.currency}>
              <p>{balance.currency} Current balance</p>
              <strong>{formatFinanceMoney(balance.currentAmountMinor, balance.currency)}</strong>
              <span>
                Available: {balance.availableAmountMinor === null
                  ? "取得不可"
                  : formatFinanceMoney(balance.availableAmountMinor, balance.currency)}
              </span>
            </article>
          ))}
        </div>
      </section>

      <section className="finance-transactions-section" aria-labelledby="finance-recent-title">
        <div className="finance-section-heading">
          <div>
            <p className="finance-eyebrow">Latest activity</p>
            <h3 id="finance-recent-title">最近の取引</h3>
          </div>
          <p>最大5件</p>
        </div>

        {summary.recentTransactions.length === 0 ? (
          <p className="finance-inline-empty">最近の取引はありません。</p>
        ) : (
          <ul className="finance-transaction-list">
            {summary.recentTransactions.map((transaction) => (
              <li key={transaction.id}>
                <div>
                  <strong>{transaction.name}</strong>
                  <p>
                    {transaction.accountName} · {transaction.category?.label ?? "未分類"} · {transaction.status}
                  </p>
                </div>
                <div className="finance-transaction-amount">
                  <strong>
                    {transaction.direction === "debit" ? "-" : "+"}
                    {formatFinanceMoney(transaction.amountMinor, transaction.currency)}
                  </strong>
                  <time dateTime={transaction.occurredAt}>
                    {formatFinanceDateTime(transaction.occurredAt)}
                  </time>
                </div>
              </li>
            ))}
          </ul>
        )}
      </section>
    </>
  );
}
