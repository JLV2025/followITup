import { Link } from "react-router-dom";
import { useAuthStore } from "../stores/authStore";

export default function Navbar() {
  const isLoggedIn = useAuthStore((s) => s.isLoggedIn);
  const user = useAuthStore((s) => s.user);
  const logout = useAuthStore((s) => s.logout);

  return (
    <nav className="navbar">
      <div style={{ display: "flex", alignItems: "center", gap: 20 }}>
        <Link to="/" className="navbar-brand">
          <img src="/logo.gif" alt="FollowITup" className="navbar-logo" />
          <span>FollowITup</span>
        </Link>
        {isLoggedIn && user?.is_admin && (
          <Link to="/admin/users" style={{ fontSize: 13, color: "var(--text-secondary)", textDecoration: "none" }}>
            用户管理
          </Link>
        )}
      </div>
      <div className="navbar-actions">
        {isLoggedIn ? (
          <>
            <span className="navbar-user">{user?.display_name || user?.email}</span>
            <button onClick={logout} className="btn btn-link">
              退出
            </button>
          </>
        ) : (
          <Link to="/login" className="btn btn-link">
            登录
          </Link>
        )}
      </div>
    </nav>
  );
}
