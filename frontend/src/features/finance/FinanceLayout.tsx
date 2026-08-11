import { useState } from "react";
import { Link, NavLink, Outlet } from "react-router-dom";

export function FinanceLayout() {
  const [isSidebarOpen, setIsSidebarOpen] = useState(true);

  return (
    <main className={`finance-shell${isSidebarOpen ? "" : " finance-sidebar-closed"}`}>
      <button
        aria-controls="finance-sidebar"
        aria-expanded={isSidebarOpen}
        aria-label={isSidebarOpen ? "Financeサイドバーを閉じる" : "Financeサイドバーを開く"}
        className="finance-sidebar-toggle"
        type="button"
        onClick={() => setIsSidebarOpen((current) => !current)}
      >
        <span aria-hidden="true">{isSidebarOpen ? "←" : "→"}</span>
      </button>

      <aside
        aria-label="Finance sidebar"
        className="finance-sidebar"
        hidden={!isSidebarOpen}
        id="finance-sidebar"
      >
        <div className="finance-brand">
          <span className="finance-brand-mark" aria-hidden="true">
            F
          </span>
          <div>
            <p>devlab</p>
            <h1>Finance</h1>
          </div>
        </div>

        <nav className="finance-nav" aria-label="Finance navigation">
          <NavLink className={financeNavItemClassName} end to="/finance-lab">
            Home
          </NavLink>
          <NavLink className={financeNavItemClassName} to="/finance-lab/accounts">
            Accounts
          </NavLink>
          <span aria-disabled="true" className="finance-nav-item finance-nav-disabled">
            <span aria-disabled="true">Transactions</span>
            <small>Coming soon</small>
          </span>
        </nav>

        <Link className="finance-back-link" to="/">
          <span aria-hidden="true">←</span>
          DevLab Boardへ戻る
        </Link>
      </aside>

      <section className="finance-workspace">
        <Outlet />
      </section>
    </main>
  );
}

function financeNavItemClassName({ isActive }: { isActive: boolean }) {
  return `finance-nav-item${isActive ? " active" : ""}`;
}
