import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useAuthStore } from "../stores/authStore";
import i18n, { LANGS, setLanguage } from "../i18n";

export default function Navbar() {
  const { t } = useTranslation();
  const isLoggedIn = useAuthStore((s) => s.isLoggedIn);
  const user = useAuthStore((s) => s.user);
  const logout = useAuthStore((s) => s.logout);

  /** 切换语言后整页刷新：gantt 的 scale/tooltip/缩放标签在初始化时固化，reload 是最稳妥的二次初始化 */
  const switchLang = (l: (typeof LANGS)[number]) => {
    if (l === i18n.language) return;
    setLanguage(l);
    window.location.reload();
  };

  return (
    <nav className="navbar">
      <div style={{ display: "flex", alignItems: "center", gap: 20 }}>
        <Link to="/" className="navbar-brand">
          <img src="/logo.gif" alt="FollowITup" className="navbar-logo" />
          <span>FollowITup</span>
        </Link>
      </div>
      <div className="navbar-actions">
        {/* 语言切换（登录/未登录均可见） */}
        <div className="lang-switcher">
          {LANGS.map((l) => (
            <button
              key={l}
              className={`btn btn-link lang-btn ${i18n.language === l ? "lang-active" : ""}`}
              onClick={() => switchLang(l)}
              title={l === "zh" ? "中文" : "English"}
            >
              {l === "zh" ? "中" : "EN"}
            </button>
          ))}
        </div>
        {isLoggedIn ? (
          <>
            <span className="navbar-user">{user?.display_name || user?.email}</span>
            {user?.is_admin && (
              <>
                <Link to="/admin/users" className="btn btn-link">
                  {t("nav.users")}
                </Link>
                <Link to="/admin/settings" className="btn btn-link">
                  {t("nav.settings")}
                </Link>
              </>
            )}
            <button onClick={logout} className="btn btn-link">
              {t("nav.logout")}
            </button>
          </>
        ) : (
          <Link to="/login" className="btn btn-link">
            {t("nav.login")}
          </Link>
        )}
      </div>
    </nav>
  );
}
