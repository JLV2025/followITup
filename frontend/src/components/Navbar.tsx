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
      </div>
      <div className="navbar-actions">
        {isLoggedIn ? (
          <>
            <span className="navbar-user">{user?.display_name || user?.email}</span>
            {user?.is_admin && (
              <>
                <Link to="/admin/users" className="btn btn-link">
                  用户管理
                </Link>
                <Link to="/admin/settings" className="btn btn-link">
                  系统设置
                </Link>
              </>
            )}
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
