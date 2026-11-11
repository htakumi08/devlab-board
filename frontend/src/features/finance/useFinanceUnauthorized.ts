import { useEffect } from "react";

import { FinanceApiError } from "./financeApi";

// TanStack Queryの401を既存App session境界へ一方向に通知する。
export function useFinanceUnauthorized(error: unknown, onSessionExpired: () => void) {
  useEffect(() => {
    if (error instanceof FinanceApiError && error.status === 401) {
      onSessionExpired();
    }
  }, [error, onSessionExpired]);
}
