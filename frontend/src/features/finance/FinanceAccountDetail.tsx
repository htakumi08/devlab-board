import { useQuery } from "@tanstack/react-query";
import { Link, useParams } from "react-router-dom";

import { FinanceApiError, financeQueryKeys, getFinanceAccount } from "./financeApi";
import {
  formatAccountMask,
  formatAccountStatus,
  formatAccountType,
  formatFinanceDateTime,
  formatFinanceMoney,
} from "./financeFormat";
import { useFinanceUnauthorized } from "./useFinanceUnauthorized";

type FinanceAccountDetailProps = {
  onLogout: () => Promise<void>;
  onSessionExpired: () => void;
};

export function FinanceAccountDetail({ onLogout, onSessionExpired }: FinanceAccountDetailProps) {
  const { accountId = "" } = useParams();
  const accountQuery = useQuery({
    queryKey: financeQueryKeys.account(accountId),
    queryFn: ({ signal }) => getFinanceAccount(accountId, signal),
    enabled: accountId.length > 0,
  });
  useFinanceUnauthorized(accountQuery.error, onSessionExpired);
  const isForbidden = accountQuery.error instanceof FinanceApiError
    && accountQuery.error.status === 403;
  const isUnauthorized = accountQuery.error instanceof FinanceApiError
    && accountQuery.error.status === 401;
  const isNotFound = accountQuery.error instanceof FinanceApiError
    && accountQuery.error.status === 404;

  return (
    <>
      <header className="finance-page-header">
        <div>
          <p className="finance-eyebrow">Account detail</p>
          <h2>{accountQuery.data?.account.name ?? "Account Detail"}</h2>
          <Link
            aria-label="口座一覧へ戻る"
            className="finance-account-back-link"
            to="/finance-lab/accounts"
          >
            <span aria-hidden="true">← </span>
            口座一覧へ戻る
          </Link>
        </div>
        <button className="finance-logout-button" type="button" onClick={() => void onLogout()}>
          Logout
        </button>
      </header>

      {accountQuery.isPending && <FinanceLoading />}
      {accountQuery.isError && isForbidden && (
        <FinanceError title="口座詳細を表示する権限がありません" />
      )}
      {accountQuery.isError && isNotFound && <FinanceNotFound />}
      {accountQuery.isError && !isForbidden && !isUnauthorized && !isNotFound && (
        <FinanceError
          title="口座詳細を取得できませんでした"
          onRetry={() => void accountQuery.refetch()}
        />
      )}
      {accountQuery.isSuccess && (
        <>
          <section className="finance-account-detail-section" aria-labelledby="account-balance-title">
            <div className="finance-account-card-heading">
              <div>
                <p>{formatAccountType(accountQuery.data.account.accountType)}</p>
                <h3 id="account-balance-title">口座情報</h3>
              </div>
              <span className={`finance-account-status ${accountQuery.data.account.status}`}>
                {formatAccountStatus(accountQuery.data.account.status)}
              </span>
            </div>
            <dl className="finance-account-detail-grid">
              <div>
                <dt>口座番号</dt>
                <dd>{formatAccountMask(accountQuery.data.account.mask)}</dd>
              </div>
              <div>
                <dt>Current balance</dt>
                <dd>
                  {formatFinanceMoney(
                    accountQuery.data.account.currentAmountMinor,
                    accountQuery.data.account.currency,
                  )}
                </dd>
              </div>
              <div>
                <dt>Available balance</dt>
                <dd>
                  {accountQuery.data.account.availableAmountMinor === null
                    ? "取得不可"
                    : formatFinanceMoney(
                        accountQuery.data.account.availableAmountMinor,
                        accountQuery.data.account.currency,
                      )}
                </dd>
              </div>
              <div>
                <dt>残高基準日時</dt>
                <dd>{formatFinanceDateTime(accountQuery.data.account.balanceAsOf)}</dd>
              </div>
            </dl>
          </section>

          <section className="finance-transactions-section" aria-labelledby="account-transactions-title">
            <div className="finance-section-heading">
              <div>
                <p className="finance-eyebrow">Latest activity</p>
                <h3 id="account-transactions-title">最近の取引</h3>
              </div>
              <p>最大5件</p>
            </div>
            {accountQuery.data.recentTransactions.length === 0 ? (
              <p className="finance-inline-empty">最近の取引はありません。</p>
            ) : (
              <ul className="finance-transaction-list">
                {accountQuery.data.recentTransactions.map((transaction) => (
                  <li key={transaction.id}>
                    <div>
                      <strong>{transaction.name}</strong>
                      <p>{transaction.category?.label ?? "未分類"} · {transaction.status}</p>
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
      )}
    </>
  );
}

function FinanceLoading() {
  return (
    <section className="finance-state-panel" role="status" aria-live="polite">
      <span className="finance-loading-dot" aria-hidden="true" />
      <p>口座詳細を読み込んでいます</p>
    </section>
  );
}

function FinanceError({ onRetry, title }: { onRetry?: () => void; title: string }) {
  return (
    <section className="finance-state-panel finance-error-panel" role="alert">
      <div>
        <h3>{title}</h3>
        <p>{onRetry ? "時間をおいて、もう一度お試しください。" : "管理者へ確認してください。"}</p>
      </div>
      {onRetry && (
        <button type="button" onClick={onRetry}>
          再試行
        </button>
      )}
    </section>
  );
}

function FinanceNotFound() {
  return (
    <section className="finance-empty-state" aria-labelledby="finance-account-not-found-title">
      <span className="finance-status-badge">404</span>
      <div>
        <h3 id="finance-account-not-found-title">口座が見つかりません</h3>
        <p>指定された口座は存在しないか、表示できません。</p>
      </div>
    </section>
  );
}
