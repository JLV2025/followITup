import { getErrorMessage } from "../utils/errorMsg";
import { useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";
import { useAuthStore } from "../stores/authStore";
import i18n from "../i18n";

export default function Login() {
  const { t } = useTranslation();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const login = useAuthStore((s) => s.login);
  const navigate = useNavigate();

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError("");

    if (!email || !password) {
      setError(i18n.t("loginPage.errEmpty"));
      return;
    }

    setLoading(true);
    try {
      const needChange = await login(email, password);
      // 首登强制改密：跳到改密页
      navigate(needChange ? "/change-password" : "/", { replace: true });
    } catch (err: any) {
      setError(getErrorMessage(err, "common.unknownError"));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="login-page">
      <div className="login-card">
        <h1 className="login-title">{t("loginPage.title")}</h1>
        <form onSubmit={handleSubmit} className="login-form">
          <div className="form-group">
            <label htmlFor="email">{t("loginPage.email")}</label>
            <input
              id="email"
              type="email"
              autoComplete="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="admin@followitup.local"
              autoFocus
            />
          </div>
          <div className="form-group">
            <label htmlFor="password">{t("loginPage.password")}</label>
            <input
              id="password"
              type="password"
              autoComplete="current-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          </div>
          {error && <div className="form-error">{error}</div>}
          <button type="submit" disabled={loading} className="btn btn-primary btn-block">
            {loading ? t("loginPage.loggingIn") : t("loginPage.login")}
          </button>
        </form>
      </div>
    </div>
  );
}
