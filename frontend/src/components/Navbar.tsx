import { Link } from "react-router-dom";
import { useAuthStore } from "../stores/authStore";

export default function Navbar() {
  const isLoggedIn = useAuthStore((s) => s.isLoggedIn);
  const user = useAuthStore((s) => s.user);
  const logout = useAuthStore((s) => s.logout);

  return (
    <nav className="navbar">
      <Link to="/" className="navbar-brand">
        FollowITup
      </Link>
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
