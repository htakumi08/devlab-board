import { describe, expect, test } from "vitest";

import { formatAccountMask } from "./financeFormat";

describe("Finance presentation formatting", () => {
  // テスト内容: APIが想定より長いmaskを返してもUIでは末尾4文字だけを表示することを確認する。
  // 必要な理由: upstreamの契約違反が完全な口座番号の画面露出へ直結しないよう防御するため。
  test("limits an account mask to the final four characters", () => {
    expect(formatAccountMask("987654321234")).toBe("•••• 1234");
  });
});
