import { useQueryClient } from "@tanstack/react-query";
import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import {
  Link,
  Navigate,
  NavLink,
  Outlet,
  Route,
  Routes,
  useLocation,
} from "react-router-dom";

import { FinanceAccountDetail } from "./features/finance/FinanceAccountDetail";
import { FinanceAccounts } from "./features/finance/FinanceAccounts";
import { FinanceLayout } from "./features/finance/FinanceLayout";
import { FinanceOverview } from "./features/finance/FinanceOverview";
import { financeQueryKeys } from "./features/finance/financeApi";
import { UserAgentLab } from "./features/user-agent/UserAgentLab";
import { apiFetch } from "./lib/api";

type User = {
  id: string;
  email: string;
  name: string;
  role: string;
};

type DashboardCard = {
  label: string;
  value: string;
};

type ApiError = {
  error?: {
    code?: string;
    message?: string;
  };
};

export function App() {
  const queryClient = useQueryClient();
  const [mode, setMode] = useState<"login" | "register">("login");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [user, setUser] = useState<User | null>(null);
  const [cards, setCards] = useState<DashboardCard[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isSidebarOpen, setIsSidebarOpen] = useState(true);
  const [message, setMessage] = useState("");

  const passwordValid = useMemo(() => validatePassword(password), [password]);
  const emailValid = useMemo(() => validateEmail(email), [email]);

  useEffect(() => {
    void loadCurrentUser();
  }, []);

  async function loadCurrentUser() {
    setIsLoading(true);
    try {
      const response = await apiFetch("/api/auth/me");
      if (!response.ok) {
        setUser(null);
        return;
      }
      const payload = (await response.json()) as { user: User };
      setUser(payload.user);
      await loadDashboard();
    } finally {
      setIsLoading(false);
    }
  }

  async function loadDashboard() {
    const response = await apiFetch("/api/dashboard");
    if (!response.ok) {
      setCards([]);
      return;
    }
    const payload = (await response.json()) as { cards: DashboardCard[] };
    setCards(payload.cards);
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setMessage("");

    if (!emailValid) {
      setMessage("メールアドレスの形式で入力してください。");
      return;
    }
    if (!passwordValid) {
      setMessage("パスワードは8文字以上、英字、数字、大文字英字を含めてください。");
      return;
    }

    setIsSubmitting(true);
    try {
      const path = mode === "login" ? "/api/auth/login" : "/api/auth/register";
      const response = await apiFetch(path, {
        method: "POST",
        body: JSON.stringify({ email, password }),
      });
      const payload = (await response.json()) as ({ user: User } & ApiError);
      if (!response.ok) {
        setMessage(payload.error?.message ?? "認証に失敗しました。");
        return;
      }
      setUser(payload.user);
      setPassword("");
      await loadDashboard();
    } finally {
      setIsSubmitting(false);
    }
  }

  async function handleLogout() {
    setMessage("");
    try {
      await apiFetch("/api/auth/logout", { method: "POST" });
    } catch {
      // logout APIが失敗しても、client上のsessionと機微なFinance cacheは残さない。
    } finally {
      clearSessionState();
    }
  }

  const clearSessionState = useCallback(() => {
    // signalを消費するFinance queryを中断してから、prefix単位で機微cacheを即時破棄する。
    void queryClient.cancelQueries(
      { queryKey: financeQueryKeys.all },
      { silent: true },
    );
    queryClient.removeQueries({ queryKey: financeQueryKeys.all });
    setMode("login");
    setMessage("");
    setUser(null);
    setCards([]);
    setPassword("");
  }, [queryClient]);

  if (isLoading) {
    return (
      <main className="center-shell">
        <div className="loading-panel">Loading</div>
      </main>
    );
  }

  if (!user) {
    return (
      <main className="auth-shell">
        <section className="auth-visual" aria-label="Project context">
          <p className="brand-label">devlab</p>
          <h1>Board</h1>
          <p className="auth-copy">
            Go と React の実験機能を、ログイン済みのダッシュボードとして育てます。
          </p>
        </section>

        <section className="auth-panel" aria-label="Authentication">
          <div className="auth-header">
            <p className="eyebrow">Session auth</p>
            <h2>{mode === "login" ? "ログイン" : "アカウント登録"}</h2>
          </div>

          <div className="segmented-control" aria-label="Auth mode">
            <button
              className={mode === "login" ? "active" : ""}
              type="button"
              onClick={() => setMode("login")}
            >
              Login
            </button>
            <button
              className={mode === "register" ? "active" : ""}
              type="button"
              onClick={() => setMode("register")}
            >
              Register
            </button>
          </div>

          <form className="auth-form" onSubmit={handleSubmit}>
            <label>
              <span>Email</span>
              <input
                autoComplete="email"
                inputMode="email"
                name="email"
                onChange={(event) => setEmail(event.target.value)}
                placeholder="user@example.com"
                type="email"
                value={email}
              />
            </label>

            <label>
              <span>Password</span>
              <input
                autoComplete={mode === "login" ? "current-password" : "new-password"}
                name="password"
                onChange={(event) => setPassword(event.target.value)}
                placeholder="Password1"
                type="password"
                value={password}
              />
            </label>

            <ul className="password-rules" aria-label="Password requirements">
              <li className={password.length >= 8 ? "valid" : ""}>8文字以上</li>
              <li className={/[A-Za-z]/.test(password) && /\d/.test(password) ? "valid" : ""}>
                英字と数字を含む
              </li>
              <li className={/[A-Z]/.test(password) ? "valid" : ""}>大文字英字を含む</li>
            </ul>

            {message && <p className="form-message">{message}</p>}

            <button className="primary-button" disabled={isSubmitting} type="submit">
              {isSubmitting ? "Processing" : mode === "login" ? "Login" : "Create account"}
            </button>
          </form>
        </section>
      </main>
    );
  }

  return (
    <>
      <ScrollToHash />
      <Routes>
        <Route
          element={
            <DevLabLayout
              isSidebarOpen={isSidebarOpen}
              onSidebarToggle={() => setIsSidebarOpen((current) => !current)}
            />
          }
        >
          <Route
            path="/"
            element={<OverviewPage cards={cards} onLogout={handleLogout} user={user} />}
          />
          <Route
            path="/user-agent-lab"
            element={<UserAgentLabPage onLogout={handleLogout} />}
          />
        </Route>
        <Route element={<FinanceLayout />} path="/finance-lab">
          <Route
            index
            element={
              <FinanceOverview
                onLogout={handleLogout}
                onSessionExpired={clearSessionState}
                user={{ email: user.email, name: user.name }}
              />
            }
          />
          <Route
            path="accounts"
            element={
              <FinanceAccounts
                onLogout={handleLogout}
                onSessionExpired={clearSessionState}
              />
            }
          />
          <Route
            path="accounts/:accountId"
            element={
              <FinanceAccountDetail
                onLogout={handleLogout}
                onSessionExpired={clearSessionState}
              />
            }
          />
          <Route path="*" element={<Navigate replace to="/finance-lab" />} />
        </Route>
        <Route path="*" element={<Navigate replace to="/" />} />
      </Routes>
    </>
  );
}

type DevLabLayoutProps = {
  isSidebarOpen: boolean;
  onSidebarToggle: () => void;
};

function DevLabLayout({ isSidebarOpen, onSidebarToggle }: DevLabLayoutProps) {
  return (
    <main className={`app-shell${isSidebarOpen ? "" : " sidebar-closed"}`}>
      <button
        aria-controls="primary-sidebar"
        aria-expanded={isSidebarOpen}
        aria-label={isSidebarOpen ? "サイドバーを閉じる" : "サイドバーを開く"}
        className="sidebar-toggle"
        title={isSidebarOpen ? "サイドバーを閉じる" : "サイドバーを開く"}
        type="button"
        onClick={onSidebarToggle}
      >
        <SidebarIcon />
      </button>

      <aside
        className="sidebar"
        hidden={!isSidebarOpen}
        id="primary-sidebar"
        aria-label="Primary navigation"
      >
        <div>
          <p className="brand-label">devlab</p>
          <h1>Board</h1>
        </div>

        <nav className="nav-list" aria-label="Sections">
          <NavLink className={navItemClassName} end to="/">
            Overview
          </NavLink>
          <Link className="nav-item" to="/#session">
            Session
          </Link>
          <NavLink className={navItemClassName} to="/user-agent-lab">
            User-Agent Lab
          </NavLink>
          <NavLink className={navItemClassName} to="/finance-lab">
            Finance Dashboard
          </NavLink>
        </nav>
      </aside>

      <section className="workspace">
        <Outlet />
      </section>
    </main>
  );
}

function SidebarIcon() {
  return (
    <svg aria-hidden="true" fill="none" focusable="false" viewBox="0 0 24 24">
      <rect
        height="16"
        rx="3"
        stroke="currentColor"
        strokeWidth="1.8"
        width="17"
        x="3.5"
        y="4"
      />
      <path d="M9 4.5v15" stroke="currentColor" strokeWidth="1.8" />
    </svg>
  );
}

type OverviewPageProps = {
  cards: DashboardCard[];
  onLogout: () => Promise<void>;
  user: User;
};

function OverviewPage({ cards, onLogout, user }: OverviewPageProps) {
  return (
    <>
      <header className="page-header" id="overview">
        <div>
          <p className="eyebrow">Authenticated workspace</p>
          <h2>開発状況ダッシュボード</h2>
        </div>
        <button className="secondary-button" type="button" onClick={() => void onLogout()}>
          Logout
        </button>
      </header>

      <section className="summary-grid" aria-label="Authenticated API summary">
        {cards.map((item) => (
          <article className="summary-card" key={item.label}>
            <p>{item.label}</p>
            <strong>{item.value}</strong>
          </article>
        ))}
      </section>

      <section className="content-grid">
        <article className="panel" id="session">
          <div className="panel-header">
            <h3>Session</h3>
            <span>scs</span>
          </div>
          <div className="profile-row">
            <span>{user.name.slice(0, 1).toUpperCase()}</span>
            <div>
              <strong>{user.email}</strong>
              <p>Role: {user.role}</p>
            </div>
          </div>
        </article>

        <article className="panel">
          <div className="panel-header">
            <h3>Next Tasks</h3>
            <span>auth</span>
          </div>
          <ol className="task-list">
            <li>CSRF token の導入方針を決める</li>
            <li>ECS Fargate + Redis の構成を設計する</li>
            <li>ログイン監査ログの要否を決める</li>
          </ol>
        </article>
      </section>
    </>
  );
}

function UserAgentLabPage({ onLogout }: { onLogout: () => Promise<void> }) {
  return (
    <>
      <header className="page-header">
        <div>
          <p className="eyebrow">Experiment workspace</p>
          <h2>User-Agent Lab</h2>
        </div>
        <button className="secondary-button" type="button" onClick={() => void onLogout()}>
          Logout
        </button>
      </header>

      <UserAgentLab />
    </>
  );
}

function ScrollToHash() {
  const location = useLocation();

  useEffect(() => {
    if (location.hash === "") {
      return;
    }

    const frame = window.requestAnimationFrame(() => {
      const target = document.getElementById(decodeURIComponent(location.hash.slice(1)));
      target?.scrollIntoView();
    });

    return () => window.cancelAnimationFrame(frame);
  }, [location.hash, location.pathname]);

  return null;
}

function navItemClassName({ isActive }: { isActive: boolean }) {
  return `nav-item${isActive ? " active" : ""}`;
}

function validateEmail(value: string) {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value.trim());
}

function validatePassword(value: string) {
  return value.length >= 8 && /[A-Za-z]/.test(value) && /\d/.test(value) && /[A-Z]/.test(value);
}
