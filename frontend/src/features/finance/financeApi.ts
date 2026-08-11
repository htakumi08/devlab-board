import { apiFetch } from "../../lib/api";

export type FinanceBalance = {
  currency: string;
  currentAmountMinor: string;
  availableAmountMinor: string | null;
};

export type FinanceTransaction = {
  id: string;
  accountId: string;
  accountName: string;
  name: string;
  merchant: string | null;
  amountMinor: string;
  currency: string;
  direction: "debit" | "credit";
  status: "pending" | "posted" | "reversed";
  category: { code: string; label: string } | null;
  occurredAt: string;
};

export type FinanceSummary = {
  accountCount: number;
  balances: FinanceBalance[];
  recentTransactions: FinanceTransaction[];
  asOf: string | null;
};

export type FinanceAccount = {
  id: string;
  name: string;
  accountType: "checking" | "savings" | "credit" | "investment" | "other";
  mask: string;
  currency: string;
  currentAmountMinor: string;
  availableAmountMinor: string | null;
  status: "active" | "closed";
  balanceAsOf: string;
};

export type FinanceAccountDetail = {
  account: FinanceAccount;
  recentTransactions: FinanceTransaction[];
};

export type FinanceTransactionsPage = {
  transactions: FinanceTransaction[];
  nextCursor: string | null;
};

export type FinanceCategory = {
  code: string;
  label: string;
};

export type FinanceTransactionQuery = {
  accountId: string | null;
  dateFrom: string | null;
  dateTo: string | null;
  category: string | null;
  direction: string | null;
  status: string | null;
  sort: string;
  cursor: string | null;
};

export const FINANCE_TRANSACTIONS_PAGE_SIZE = 25;

export const financeQueryKeys = {
  all: ["finance"] as const,
  summary: () => ["finance", "summary"] as const,
  accounts: () => ["finance", "accounts", "list"] as const,
  account: (accountId: string) => ["finance", "accounts", "detail", accountId] as const,
  categories: () => ["finance", "categories", "list"] as const,
  transactions: (query: FinanceTransactionQuery) => [
    "finance",
    "transactions",
    "list",
    query,
  ] as const,
};

type FinanceErrorPayload = {
  error?: {
    code?: string;
    message?: string;
  };
};

export class FinanceApiError extends Error {
  readonly status: number;
  readonly code?: string;

  constructor(status: number, code?: string, message?: string) {
    super(message ?? code ?? "finance_summary_unavailable");
    this.name = "FinanceApiError";
    this.status = status;
    this.code = code;
  }
}

async function getFinanceResponse<T>(path: string, signal?: AbortSignal): Promise<T> {
  const response = await apiFetch(path, { signal });
  if (!response.ok) {
    let payload: FinanceErrorPayload = {};
    try {
      payload = (await response.json()) as FinanceErrorPayload;
    } catch {
      // Error bodyが空または非JSONでも、HTTP statusはUIの状態遷移に利用する。
    }
    throw new FinanceApiError(response.status, payload.error?.code, payload.error?.message);
  }
  return (await response.json()) as T;
}

// Finance server stateの取得先とresponse shapeを画面から分離する。
export async function getFinanceSummary(signal?: AbortSignal): Promise<FinanceSummary> {
  const payload = await getFinanceResponse<{ summary: FinanceSummary }>(
    "/api/finance/summary",
    signal,
  );
  return payload.summary;
}

export async function getFinanceAccounts(signal?: AbortSignal): Promise<FinanceAccount[]> {
  const payload = await getFinanceResponse<{ accounts: FinanceAccount[] }>(
    "/api/finance/accounts",
    signal,
  );
  return payload.accounts;
}

export async function getFinanceCategories(signal?: AbortSignal): Promise<FinanceCategory[]> {
  const payload = await getFinanceResponse<{ categories: FinanceCategory[] }>(
    "/api/finance/categories",
    signal,
  );
  return payload.categories;
}

export async function getFinanceAccount(
  publicAccountId: string,
  signal?: AbortSignal,
): Promise<FinanceAccountDetail> {
  return getFinanceResponse<FinanceAccountDetail>(
    `/api/finance/accounts/${encodeURIComponent(publicAccountId)}`,
    signal,
  );
}

export async function getFinanceTransactions(
  transactionQuery: FinanceTransactionQuery,
  signal?: AbortSignal,
): Promise<FinanceTransactionsPage> {
  const query = new URLSearchParams();
  const filters = [
    ["account_id", transactionQuery.accountId],
    ["date_from", transactionQuery.dateFrom],
    ["date_to", transactionQuery.dateTo],
    ["category", transactionQuery.category],
    ["direction", transactionQuery.direction],
    ["status", transactionQuery.status],
  ] as const;
  for (const [name, value] of filters) {
    if (value !== null) {
      query.set(name, value);
    }
  }
  if (transactionQuery.sort !== "newest") {
    query.set("sort", transactionQuery.sort);
  }
  if (transactionQuery.cursor !== null) {
    query.set("cursor", transactionQuery.cursor);
  }
  query.set("limit", String(FINANCE_TRANSACTIONS_PAGE_SIZE));
  return getFinanceResponse<FinanceTransactionsPage>(
    `/api/finance/transactions?${query.toString()}`,
    signal,
  );
}
