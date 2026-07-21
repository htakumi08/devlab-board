import { FormEvent, useEffect, useMemo, useState } from "react";

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

const apiBaseUrl = import.meta.env.VITE_API_BASE_URL ?? "http://localhost:30102";

export function App() {
  const [mode, setMode] = useState<"login" | "register">("login");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [user, setUser] = useState<User | null>(null);
  const [cards, setCards] = useState<DashboardCard[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [isSubmitting, setIsSubmitting] = useState(false);
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
    await apiFetch("/api/auth/logout", { method: "POST" });
    setUser(null);
    setCards([]);
    setPassword("");
  }

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
    <main className="app-shell">
      <aside className="sidebar" aria-label="Primary navigation">
        <div>
          <p className="brand-label">devlab</p>
          <h1>Board</h1>
        </div>

        <nav className="nav-list" aria-label="Sections">
          <a className="nav-item active" href="#overview">
            Overview
          </a>
          <a className="nav-item" href="#session">
            Session
          </a>
        </nav>
      </aside>

      <section className="workspace" id="overview">
        <header className="page-header">
          <div>
            <p className="eyebrow">Authenticated workspace</p>
            <h2>開発状況ダッシュボード</h2>
          </div>
          <button className="secondary-button" type="button" onClick={handleLogout}>
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
      </section>
    </main>
  );
}

async function apiFetch(path: string, init: RequestInit = {}) {
  return fetch(`${apiBaseUrl}${path}`, {
    ...init,
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      ...(init.headers ?? {}),
    },
  });
}

function validateEmail(value: string) {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value.trim());
}

function validatePassword(value: string) {
  return value.length >= 8 && /[A-Za-z]/.test(value) && /\d/.test(value) && /[A-Z]/.test(value);
}
