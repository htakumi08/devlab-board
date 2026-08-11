DROP INDEX IF EXISTS finance_transactions_account_recent_idx;

CREATE INDEX finance_transactions_account_recent_idx
    ON finance_transactions (
        account_id,
        (COALESCE(posted_at, authorized_at)) DESC,
        id DESC
    );
