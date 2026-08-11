import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";

import { FinanceApiError, financeQueryKeys, getFinanceAccounts } from "./financeApi";
import {
  formatAccountMask,
  formatAccountStatus,
  formatAccountType,
  formatFinanceDateTime,
  formatFinanceMoney,
} from "./financeFormat";
import { useFinanceUnauthorized } from "./useFinanceUnauthorized";

type FinanceAccountsProps = {
  onLogout: () => Promise<void>;
  onSessionExpired: () => void;
};

export function FinanceAccounts({ onLogout, onSessionExpired }: FinanceAccountsProps) {
  const accountsQuery = useQuery({
    queryKey: financeQueryKeys.accounts(),
    queryFn: ({ signal }) => getFinanceAccounts(signal),
  });
  useFinanceUnauthorized(accountsQuery.error, onSessionExpired);
  const isForbidden = accountsQuery.error instanceof FinanceApiError
    && accountsQuery.error.status === 403;
  const isUnauthorized = accountsQuery.error instanceof FinanceApiError
    && accountsQuery.error.status === 401;

  return (
    <>
      <header className="finance-page-header">
        <div>
          <p className="finance-eyebrow">Your connected accounts</p>
          <h2>Accounts</h2>
          <p className="finance-welcome">口座番号は末尾のmaskだけを表示します。</p>
        </div>
        <button className="finance-logout-button" type="button" onClick={() => void onLogout()}>
          Logout
        </button>
      </header>

      {accountsQuery.isPending && <FinanceLoading message="口座一覧を読み込んでいます" />}
      {accountsQuery.isError && isForbidden && (
        <FinanceError title="口座一覧を表示する権限がありません" />
      )}
      {accountsQuery.isError && !isForbidden && !isUnauthorized && (
        <FinanceError
          title="口座一覧を取得できませんでした"
          onRetry={() => void accountsQuery.refetch()}
        />
      )}
      {accountsQuery.isSuccess && accountsQuery.data.length === 0 && (
        <section className="finance-empty-state" aria-labelledby="finance-accounts-empty-title">
          <span className="finance-status-badge">EMPTY</span>
          <div>
            <h3 id="finance-accounts-empty-title">口座がありません</h3>
            <p>現在のユーザーに所属するFinance口座はまだありません。</p>
          </div>
        </section>
      )}
      {accountsQuery.isSuccess && accountsQuery.data.length > 0 && (
        <section className="finance-accounts-section" aria-labelledby="finance-accounts-title">
          <div className="finance-section-heading">
            <div>
              <p className="finance-eyebrow">Account portfolio</p>
              <h3 id="finance-accounts-title">口座一覧</h3>
            </div>
            <p>{accountsQuery.data.length}口座</p>
          </div>
          <div className="finance-account-grid">
            {accountsQuery.data.map((account) => (
              <article className="finance-account-card" key={account.id}>
                <div className="finance-account-card-heading">
                  <div>
                    <p>{formatAccountType(account.accountType)}</p>
                    <h3>{account.name}</h3>
                  </div>
                  <span className={`finance-account-status ${account.status}`}>
                    {formatAccountStatus(account.status)}
                  </span>
                </div>
                <p className="finance-account-mask">{formatAccountMask(account.mask)}</p>
                <strong>{formatFinanceMoney(account.currentAmountMinor, account.currency)}</strong>
                <p>
                  Available: {account.availableAmountMinor === null
                    ? "取得不可"
                    : formatFinanceMoney(account.availableAmountMinor, account.currency)}
                </p>
                <time dateTime={account.balanceAsOf}>
                  残高基準日時 {formatFinanceDateTime(account.balanceAsOf)}
                </time>
                <Link
                  aria-label={`${account.name}の詳細を見る`}
                  className="finance-account-detail-link"
                  to={`/finance-lab/accounts/${encodeURIComponent(account.id)}`}
                >
                  詳細を見る
                </Link>
              </article>
            ))}
          </div>
        </section>
      )}
    </>
  );
}

function FinanceLoading({ message }: { message: string }) {
  return (
    <section className="finance-state-panel" role="status" aria-live="polite">
      <span className="finance-loading-dot" aria-hidden="true" />
      <p>{message}</p>
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
