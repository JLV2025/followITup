import { useState } from "react";
import { useNavigate } from "react-router-dom";
import api from "../api/client";
import { useAuthStore } from "../stores/authStore";

export default function ChangePassword() {
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
      setError("两次输入的新密码不一致");
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
      alert("密码修改成功");
      navigate("/", { replace: true });
    } catch (err: any) {
      setError(err?.response?.data?.error?.message || "修改失败");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="login-page">
      <div className="login-card">
        <h1 className="login-title">首次登录：请修改密码</h1>
        <p className="text-secondary">为保障账号安全，请设置你的新密码。</p>
        <form onSubmit={handleSubmit} className="login-form">
          <div className="form-group">
            <label htmlFor="old-password">初始密码</label>
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
            <label htmlFor="new-password">新密码</label>
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
            <label htmlFor="confirm-password">确认新密码</label>
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
            {submitting ? "提交中..." : "修改密码"}
          </button>
        </form>
      </div>
    </div>
  );
}
