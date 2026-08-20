import { getErrorMessage } from "../utils/errorMsg";
import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import api from "../api/client";
import { useAuthStore } from "../stores/authStore";
import i18n from "../i18n";

interface User {
  id: number;
  login: string;
  email: string;
  display_name: string;
  auth_source: string;
  is_admin: boolean;
  is_active: boolean;
}

export default function UserManagement() {
  const { t } = useTranslation();
  const isAdmin = useAuthStore((s) => s.user?.is_admin);
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [email, setEmail] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [isAdminChecked, setIsAdminChecked] = useState(false);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const [creating, setCreating] = useState(false);
  // 重置密码弹窗
  const [resetTarget, setResetTarget] = useState<User | null>(null);
  const [resetMustChange, setResetMustChange] = useState(true);
  const [resetting, setResetting] = useState(false);

  const fetchUsers = async () => {
    try {
      const res = await api.get("/api/admin/users");
      setUsers(res.data.data || []);
    } catch { /* ignore */ }
    setLoading(false);
  };

  useEffect(() => { fetchUsers(); }, []);

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    if (!email) { setError(i18n.t("usersPage.emailRequired")); return; }
    setCreating(true);
    try {
      const res = await api.post("/api/admin/users", {
        email: email.trim(),
        display_name: displayName.trim() || undefined,
        is_admin: isAdminChecked,
      });
      const d = res.data?.data;
      setMessage(d?.initial_password
        ? i18n.t("usersPage.createdWithPwd", { message: d.message, pwd: d.initial_password })
        : d?.message || i18n.t("usersPage.created"));
      setEmail(""); setDisplayName(""); setIsAdminChecked(false); setShowForm(false);
      fetchUsers();
    } catch (err: any) {
      setError(getErrorMessage(err, "usersPage.errCreate"));
    }
    setCreating(false);
  };

  const handleRole = async (u: User) => {
    try {
      await api.put(`/api/admin/users/${u.id}/role`, { is_admin: !u.is_admin });
      fetchUsers();
    } catch (err: any) {
      alert(getErrorMessage(err, "common.unknownError"));
    }
  };

  const handleDelete = async (u: User) => {
    if (!confirm(i18n.t("usersPage.confirmDelete", { name: u.display_name || u.email }))) return;
    try {
      await api.delete(`/api/admin/users/${u.id}`);
      fetchUsers();
    } catch (err: any) {
      alert(getErrorMessage(err, "common.unknownError"));
    }
  };

  const handleResetPassword = async () => {
    if (!resetTarget) return;
    setResetting(true);
    try {
      const res = await api.post(`/api/admin/users/${resetTarget.id}/reset-password`, {
        must_change: resetMustChange,
      });
      const d = res.data?.data;
      setMessage(d?.initial_password
        ? i18n.t("usersPage.resetWithPwd", { message: d.message, pwd: d.initial_password })
        : d?.message || i18n.t("usersPage.resetDone"));
      setResetTarget(null);
    } catch (err: any) {
      alert(getErrorMessage(err, "common.unknownError"));
      setResetTarget(null);
    } finally {
      setResetting(false);
    }
  };

  return (
    <div style={{ maxWidth: 720, margin: "0 auto" }}>
      <div className="dashboard-header-row" style={{ marginBottom: 24 }}>
        <div>
          <h1 style={{ fontSize: 24, fontWeight: 600, marginBottom: 4 }}>{t("usersPage.title")}</h1>
          <p className="text-secondary">{t("usersPage.subtitle")}</p>
        </div>
        <div style={{ display: "flex", gap: 8 }}>
          <Link to="/" className="btn btn-ghost btn-sm">{t("nav.backDashboard")}</Link>
          <button className="btn btn-primary btn-sm" onClick={() => setShowForm(!showForm)}>
            {showForm ? t("usersPage.cancel") : t("usersPage.addUser")}
          </button>
        </div>
      </div>

      {showForm && (
        <form onSubmit={handleCreate} style={{ background: "var(--card-bg)", border: "1px solid var(--card-border)", borderRadius: 8, padding: 20, marginBottom: 20 }}>
          <h3 style={{ fontSize: 16, fontWeight: 600, marginBottom: 16 }}>{t("usersPage.formTitle")}</h3>
          <p className="text-secondary" style={{ marginBottom: 12, fontSize: 13 }}>
            密码由系统随机生成，通过邮件发送至用户邮箱；首次登录需修改密码。
          </p>
          {error && <div className="form-error">{error}</div>}
          <div className="form-row">
            <div className="form-group">
              <label>{t("usersPage.emailLabel")}</label>
              <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} placeholder="user@example.com" autoFocus />
            </div>
            <div className="form-group">
              <label>{t("usersPage.displayName")}</label>
              <input value={displayName} onChange={(e) => setDisplayName(e.target.value)} placeholder={t("usersPage.namePlaceholder")} />
            </div>
          </div>
          {isAdmin && (
            <div className="form-group">
              <label>
                <input type="checkbox" checked={isAdminChecked}
                  onChange={(e) => setIsAdminChecked(e.target.checked)} />
                {" "}设为管理员
              </label>
            </div>
          )}
          <button className="btn btn-primary" type="submit" disabled={creating}>{creating ? t("usersPage.creating") : t("usersPage.createUser")}</button>
        </form>
      )}

      {loading ? (
        <p className="text-secondary">{t("common.loading")}</p>
      ) : (
        <table className="task-table" style={{ marginTop: 16 }}>
          <thead>
            <tr>
              <th style={{ width: 50 }}>ID</th>
              <th>{t("usersPage.colName")}</th>
              <th>{t("usersPage.colEmail")}</th>
              <th style={{ width: 80 }}>{t("usersPage.colSource")}</th>
              <th style={{ width: 80 }}>{t("usersPage.colRole")}</th>
              {isAdmin && <th style={{ width: 200 }}>{t("usersPage.colActions")}</th>}
            </tr>
          </thead>
          <tbody>
            {users.map((u) => (
              <tr key={u.id}>
                <td className="cell-id">{u.id}</td>
                <td style={{ fontWeight: 500 }}>{u.display_name || u.email}</td>
                <td className="text-secondary">{u.email}</td>
                <td><span className="status-badge" style={{ background: u.auth_source === "local" ? "var(--surface-alt)" : "rgba(8, 145, 178, 0.1)", color: u.auth_source === "local" ? "var(--text-secondary)" : "var(--accent)" }}>{u.auth_source === "local" ? t("usersPage.sourceLocal") : t("usersPage.sourceLdap")}</span></td>
                <td>{u.is_admin ? t("usersPage.roleAdmin") : t("usersPage.roleMember")}</td>
                {isAdmin && (
                  <td>
                    <button className="btn btn-ghost btn-sm" onClick={() => handleRole(u)}>
                      {u.is_admin ? t("usersPage.toggleAdminOff") : t("usersPage.toggleAdminOn")}
                    </button>{" "}
                    <button className="btn btn-ghost btn-sm"
                      onClick={() => { setResetMustChange(true); setResetTarget(u); }}>
                      重置密码
                    </button>{" "}
                    <button className="btn btn-ghost btn-sm" style={{ color: "var(--danger)" }} onClick={() => handleDelete(u)}>
                      删除
                    </button>
                  </td>
                )}
              </tr>
            ))}
            {users.length === 0 && (
              <tr><td colSpan={isAdmin ? 6 : 5} className="text-secondary" style={{ textAlign: "center", padding: 32 }}>{t("usersPage.empty")}</td></tr>
            )}
          </tbody>
        </table>
      )}
      {message && <div className="form-error" style={{ marginTop: 12 }}>{message}</div>}

      {/* 重置密码弹窗 */}
      {resetTarget && (
        <div className="modal-overlay" onClick={() => setResetTarget(null)}>
          <div className="modal-card" onClick={(e) => e.stopPropagation()}>
            <div className="modal-title">
              <h2>{t("usersPage.resetTitle", { name: resetTarget.display_name || resetTarget.email })}</h2>
              <button className="btn btn-ghost btn-sm" onClick={() => setResetTarget(null)}>×</button>
            </div>
            <div className="modal-body">
              <p className="text-secondary" style={{ fontSize: 13, marginBottom: 12 }}>
                将生成随机新密码，通过邮件发送；邮件不可达时会在下方显示新密码。
              </p>
              <label style={{ display: "flex", alignItems: "center", gap: 8, fontSize: 14 }}>
                <input type="checkbox" checked={resetMustChange}
                  onChange={(e) => setResetMustChange(e.target.checked)} />
                用户下次登录时须更改密码
              </label>
            </div>
            <div className="modal-actions">
              <button className="btn btn-ghost" onClick={() => setResetTarget(null)}>{t("usersPage.cancel")}</button>
              <button className="btn btn-primary" disabled={resetting}
                onClick={handleResetPassword}>
                {resetting ? t("usersPage.resetting") : t("usersPage.confirmReset")}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
