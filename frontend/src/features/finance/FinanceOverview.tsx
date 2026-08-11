import { useEffect, useState } from "react";

import { FinanceApiError, FinanceSummary, getFinanceSummary } from "./financeApi";

type FinanceOverviewProps = {
  onLogout: () => Promise<void>;
  onSessionExpired: () => void;
  user: {
    email: string;
    name: string;
  };
};

type FinanceOverviewStatus = "loading" | "success" | "error" | "permission-denied";

export function FinanceOverview({ onLogout, onSessionExpired, user }: FinanceOverviewProps) {
  const [summary, setSummary] = useState<FinanceSummary | null>(null);
  const [status, setStatus] = useState<FinanceOverviewStatus>("loading");

  async function loadSummary(isActive: () => boolean) {
    if (isActive()) {
      setStatus("loading");
    }

    try {
      const result = await getFinanceSummary();
      if (isActive()) {
        setSummary(result);
        setStatus("success");
      }
    } catch (error) {
      if (!isActive()) {
        return;
      }
      if (error instanceof FinanceApiError && error.status === 401) {
        onSessionExpired();
        return;
      }
      setStatus(error instanceof FinanceApiError && error.status === 403
        ? "permission-denied"
        : "error");
    }
  }

  useEffect(() => {
    let active = true;

    // 初回取得と再試行を同じ状態遷移へ通し、401/403の扱いが分岐しないようにする。
    void loadSummary(() => active);

    return () => {
      active = false;
    };
  }, []);

  async function retrySummary() {
    await loadSummary(() => true);
  }

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

      {status === "loading" && (
        <section className="finance-state-panel" role="status" aria-live="polite">
          <span className="finance-loading-dot" aria-hidden="true" />
          <p>Finance概要を読み込んでいます</p>
        </section>
      )}

      {status === "error" && (
        <section className="finance-state-panel finance-error-panel" role="alert">
          <div>
            <h3>Finance概要を取得できませんでした</h3>
            <p>時間をおいて、もう一度お試しください。</p>
          </div>
          <button type="button" onClick={() => void retrySummary()}>
            再試行
          </button>
        </section>
      )}

      {status === "permission-denied" && (
        <section className="finance-state-panel finance-error-panel" role="alert">
          <div>
            <h3>Finance概要を表示する権限がありません</h3>
            <p>この機能を利用する権限について、管理者へ確認してください。</p>
          </div>
        </section>
      )}

      {status === "success" && summary?.accountCount === 0 && <FinanceEmptyState />}
      {status === "success" && summary && summary.accountCount > 0 && (
        <FinanceSummaryView summary={summary} />
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
              ? `残高基準日時（最古） ${formatDateTime(summary.asOf)}`
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
              <strong>{formatMoney(balance.currentAmountMinor, balance.currency)}</strong>
              <span>
                Available: {balance.availableAmountMinor === null
                  ? "取得不可"
                  : formatMoney(balance.availableAmountMinor, balance.currency)}
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
                    {formatMoney(transaction.amountMinor, transaction.currency)}
                  </strong>
                  <time dateTime={transaction.occurredAt}>{formatDateTime(transaction.occurredAt)}</time>
                </div>
              </li>
            ))}
          </ul>
        )}
      </section>
    </>
  );
}

function formatMoney(amountMinor: string, currency: string) {
  const formatter = new Intl.NumberFormat("ja-JP", { style: "currency", currency });
  const fractionDigits = formatter.resolvedOptions().maximumFractionDigits ?? 2;
  const minorAmount = BigInt(amountMinor);
  const isNegative = minorAmount < 0n;
  const absoluteMinorAmount = isNegative ? -minorAmount : minorAmount;
  const minorUnitScale = 10n ** BigInt(fractionDigits);
  const majorAmount = absoluteMinorAmount / minorUnitScale;
  const fractionAmount = absoluteMinorAmount % minorUnitScale;
  const integerParts = formatter
    .formatToParts(majorAmount)
    .filter((part) => part.type === "integer" || part.type === "group");
  const templateParts = formatter.formatToParts(isNegative ? -1n : 1n);

  // Intlの通貨記号・符号・区切り位置を保ち、実額だけはBigIntのまま組み立てる。
  return templateParts
    .flatMap((part) => {
      if (part.type === "integer") {
        return integerParts;
      }
      if (part.type === "fraction") {
        return [{ ...part, value: fractionAmount.toString().padStart(fractionDigits, "0") }];
      }
      return [part];
    })
    .map((part) => part.value)
    .join("");
}

function formatDateTime(value: string) {
  return new Intl.DateTimeFormat("ja-JP", {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(new Date(value));
}
