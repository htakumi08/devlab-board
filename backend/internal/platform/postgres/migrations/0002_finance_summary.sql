CREATE TABLE IF NOT EXISTS finance_accounts (
    id BIGSERIAL PRIMARY KEY,
    public_id UUID NOT NULL UNIQUE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    account_type TEXT NOT NULL CONSTRAINT finance_accounts_type_check
        CHECK (account_type IN ('checking', 'savings', 'credit', 'investment', 'other')),
    mask TEXT NOT NULL CONSTRAINT finance_accounts_mask_check
        CHECK (char_length(mask) BETWEEN 1 AND 4),
    currency TEXT NOT NULL CONSTRAINT finance_accounts_currency_check
        CHECK (currency ~ '^[A-Z]{3}$'),
    current_balance_minor BIGINT NOT NULL,
    available_balance_minor BIGINT,
    status TEXT NOT NULL DEFAULT 'active' CONSTRAINT finance_accounts_status_check
        CHECK (status IN ('active', 'closed')),
    balance_as_of TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT finance_accounts_id_user_unique UNIQUE (id, user_id)
);

CREATE INDEX IF NOT EXISTS finance_accounts_user_status_idx
    ON finance_accounts (user_id, status);

CREATE TABLE IF NOT EXISTS finance_transaction_categories (
    code TEXT PRIMARY KEY,
    label TEXT NOT NULL,
    display_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO finance_transaction_categories (code, label, display_order)
VALUES
    ('income', '収入', 10),
    ('groceries', '食料品', 20),
    ('transportation', '交通', 30),
    ('utilities', '公共料金', 40),
    ('other', 'その他', 999)
ON CONFLICT (code) DO NOTHING;

CREATE TABLE IF NOT EXISTS finance_transactions (
    id BIGSERIAL PRIMARY KEY,
    public_id UUID NOT NULL UNIQUE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    account_id BIGINT NOT NULL,
    name TEXT NOT NULL,
    merchant TEXT,
    amount_minor BIGINT NOT NULL CONSTRAINT finance_transactions_amount_check
        CHECK (amount_minor >= 0),
    currency TEXT NOT NULL CONSTRAINT finance_transactions_currency_check
        CHECK (currency ~ '^[A-Z]{3}$'),
    direction TEXT NOT NULL CONSTRAINT finance_transactions_direction_check
        CHECK (direction IN ('debit', 'credit')),
    status TEXT NOT NULL CONSTRAINT finance_transactions_status_check
        CHECK (status IN ('pending', 'posted', 'reversed')),
    category_code TEXT REFERENCES finance_transaction_categories(code) ON DELETE SET NULL,
    authorized_at TIMESTAMPTZ NOT NULL,
    posted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT finance_transactions_account_owner_fk
        FOREIGN KEY (account_id, user_id)
        REFERENCES finance_accounts(id, user_id)
        ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS finance_transactions_user_recent_idx
    ON finance_transactions (user_id, authorized_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS finance_transactions_account_recent_idx
    ON finance_transactions (account_id, authorized_at DESC, id DESC);
