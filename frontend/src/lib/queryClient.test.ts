import { describe, expect, test } from "vitest";

import { financeQueryKeys } from "../features/finance/financeApi";
import { createAppQueryClient } from "./queryClient";

describe("App QueryClient", () => {
  // テスト内容: Appごとに独立したQueryClientを生成できることを確認する。
  // 必要な理由: テスト・session間でFinanceの機微なserver stateが共有される回帰を防ぐため。
  test("creates an isolated Finance query cache", () => {
    const firstClient = createAppQueryClient();
    const secondClient = createAppQueryClient();

    firstClient.setQueryData(financeQueryKeys.accounts(), { accounts: [{ id: "account-1" }] });

    expect(firstClient.getQueryData(financeQueryKeys.accounts())).toBeDefined();
    expect(secondClient.getQueryData(financeQueryKeys.accounts())).toBeUndefined();
  });
});
