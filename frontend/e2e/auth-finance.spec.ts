import { expect, test } from "@playwright/test";

// テスト内容: ログイン後にFinanceへ移動し、ログアウト後は再び認証画面へ戻る。
// 必要な理由: 実ブラウザのroute遷移とsession表示境界を一つの操作で確かめるため。
test("login, open Finance, and log out", async ({ page }) => {
  let loggedIn = false;
  await page.route("**/api/**", async (route) => {
    const path = new URL(route.request().url()).pathname;
    const method = route.request().method();
    const respond = (body: unknown, status = 200) =>
      route.fulfill({ status, contentType: "application/json", body: JSON.stringify(body) });

    if (path === "/api/auth/me") {
      await respond(loggedIn ? { user: { id: "test-user", email: "test@example.invalid", name: "Test", role: "user" } } : { error: { code: "unauthorized" } }, loggedIn ? 200 : 401);
    } else if (path === "/api/auth/login" && method === "POST") {
      loggedIn = true;
      await respond({ user: { id: "test-user", email: "test@example.invalid", name: "Test", role: "user" } });
    } else if (path === "/api/dashboard") {
      await respond({ cards: [] });
    } else if (path === "/api/finance/summary") {
      await respond({ summary: { accountCount: 0, balances: [], recentTransactions: [], asOf: null } });
    } else if (path === "/api/auth/logout" && method === "POST") {
      loggedIn = false;
      await respond({});
    } else {
      await respond({ error: { code: "unexpected_request" } }, 404);
    }
  });

  await page.goto("/");
  await expect(page.getByRole("heading", { name: "ログイン" })).toBeVisible();
  await page.getByRole("textbox", { name: "Email" }).fill("test@example.invalid");
  await page.getByRole("textbox", { name: "Password" }).fill("Password01");
  await page.getByRole("button", { name: "Login" }).last().click();
  await expect(page.getByRole("heading", { name: "開発状況ダッシュボード" })).toBeVisible();

  await page.getByRole("link", { name: "Finance Dashboard" }).click();
  await expect(page).toHaveURL(/\/finance-lab$/);
  await expect(page.getByRole("heading", { name: "Finance Overview" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "口座データがありません" })).toBeVisible();

  await page.getByRole("button", { name: "Logout" }).click();
  await expect(page.getByRole("heading", { name: "ログイン" })).toBeVisible();
});
