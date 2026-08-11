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

// Finance server stateの取得先とresponse shapeを画面から分離する。
export async function getFinanceSummary(): Promise<FinanceSummary> {
  const response = await apiFetch("/api/finance/summary");
  if (!response.ok) {
    let payload: FinanceErrorPayload = {};
    try {
      payload = (await response.json()) as FinanceErrorPayload;
    } catch {
      // Error bodyが空または非JSONでも、HTTP statusはUIの状態遷移に利用する。
    }
    throw new FinanceApiError(response.status, payload.error?.code, payload.error?.message);
  }
  const payload = (await response.json()) as { summary: FinanceSummary };
  return payload.summary;
}
