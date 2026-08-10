import { getErrorMessage } from "../utils/errorMsg";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";
import api from "../api/client";
import { useAuthStore } from "../stores/authStore";
import i18n from "../i18n";

export default function ChangePassword() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const setToken = useAuthStore((s) => s.setToken);
  const [oldPassword, setOldPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    if (newPassword !== confirmPassword) {
      setError(i18n.t("changePassword.errMismatch"));
      return;
    }
    setSubmitting(true);
    try {
      const res = await api.post("/api/auth/change-password", {
        old_password: oldPassword,
        new_password: newPassword,
      });
      // 用新 token 替换（原 token 带首登标记）
      setToken(res.data?.data?.token);
      alert(i18n.t("changePassword.success"));
      navigate("/", { replace: true });
    } catch (err: any) {
      setError(getErrorMessage(err, "common.unknownError"));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="login-page">
      <div className="login-card">
        <h1 className="login-title">{t("changePassword.title")}</h1>
        <p className="text-secondary">{t("changePassword.subtitle")}</p>
        <form onSubmit={handleSubmit} className="login-form">
          <div className="form-group">
            <label htmlFor="old-password">{t("changePassword.oldPwd")}</label>
            <input
              id="old-password"
              type="password"
              required
              autoComplete="current-password"
              value={oldPassword}
              onChange={(e) => setOldPassword(e.target.value)}
              autoFocus
            />
          </div>
          <div className="form-group">
            <label htmlFor="new-password">{t("changePassword.newPwd")}</label>
            <input
              id="new-password"
              type="password"
              required
              autoComplete="new-password"
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
            />
          </div>
          <div className="form-group">
            <label htmlFor="confirm-password">{t("changePassword.confirmPwd")}</label>
            <input
              id="confirm-password"
              type="password"
              required
              autoComplete="new-password"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
            />
          </div>
          {error && <div className="form-error">{error}</div>}
          <button type="submit" disabled={submitting} className="btn btn-primary btn-block">
            {submitting ? t("changePassword.submitting") : t("changePassword.submit")}
          </button>
        </form>
      </div>
    </div>
  );
}
