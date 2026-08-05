import { useEffect, useState } from "react";
import api from "../api/client";
import { useAuthStore } from "../stores/authStore";

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
    if (!email) { setError("邮箱不能为空"); return; }
    setCreating(true);
    try {
      const res = await api.post("/api/admin/users", {
        email: email.trim(),
        display_name: displayName.trim() || undefined,
        is_admin: isAdminChecked,
      });
      const d = res.data?.data;
      setMessage(d?.initial_password
        ? `${d.message}（初始密码：${d.initial_password}）`
        : d?.message || "用户创建成功");
      setEmail(""); setDisplayName(""); setIsAdminChecked(false); setShowForm(false);
      fetchUsers();
    } catch (err: any) {
      setError(err.response?.data?.error?.message || "创建失败");
    }
    setCreating(false);
  };

  const handleRole = async (u: User) => {
    try {
      await api.put(`/api/admin/users/${u.id}/role`, { is_admin: !u.is_admin });
      fetchUsers();
    } catch (err: any) {
      alert(err?.response?.data?.error?.message || "操作失败");
    }
  };

  const handleDelete = async (u: User) => {
    if (!confirm(`确认删除用户「${u.display_name || u.email}」？\n\n历史项目和任务上的名字保留备查。`)) return;
    try {
      await api.delete(`/api/admin/users/${u.id}`);
      fetchUsers();
    } catch (err: any) {
      alert(err?.response?.data?.error?.message || "删除失败");
    }
  };

  return (
    <div style={{ maxWidth: 720, margin: "0 auto" }}>
      <div className="dashboard-header-row" style={{ marginBottom: 24 }}>
        <div>
          <h1 style={{ fontSize: 24, fontWeight: 600, marginBottom: 4 }}>用户管理</h1>
          <p className="text-secondary">管理本地账号，LDAP 用户自动同步</p>
        </div>
        <button className="btn btn-primary btn-sm" onClick={() => setShowForm(!showForm)}>
          {showForm ? "取消" : "+ 添加用户"}
        </button>
      </div>

      {showForm && (
        <form onSubmit={handleCreate} style={{ background: "var(--card-bg)", border: "1px solid var(--card-border)", borderRadius: 8, padding: 20, marginBottom: 20 }}>
          <h3 style={{ fontSize: 16, fontWeight: 600, marginBottom: 16 }}>新建本地用户</h3>
          <p className="text-secondary" style={{ marginBottom: 12, fontSize: 13 }}>
            密码由系统随机生成，通过邮件发送至用户邮箱；首次登录需修改密码。
          </p>
          {error && <div className="form-error">{error}</div>}
          <div className="form-row">
            <div className="form-group">
              <label>邮箱（登录名）</label>
              <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} placeholder="user@example.com" autoFocus />
            </div>
            <div className="form-group">
              <label>显示名称（留空自动从邮箱推导）</label>
              <input value={displayName} onChange={(e) => setDisplayName(e.target.value)} placeholder="如 john.doe → John Doe" />
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
          <button className="btn btn-primary" type="submit" disabled={creating}>{creating ? "创建中..." : "创建用户"}</button>
        </form>
      )}

      {loading ? (
        <p className="text-secondary">加载中...</p>
      ) : (
        <table className="task-table" style={{ marginTop: 16 }}>
          <thead>
            <tr>
              <th style={{ width: 50 }}>ID</th>
              <th>显示名称</th>
              <th>邮箱</th>
              <th style={{ width: 80 }}>来源</th>
              <th style={{ width: 80 }}>角色</th>
              {isAdmin && <th style={{ width: 200 }}>操作</th>}
            </tr>
          </thead>
          <tbody>
            {users.map((u) => (
              <tr key={u.id}>
                <td className="cell-id">{u.id}</td>
                <td style={{ fontWeight: 500 }}>{u.display_name || u.email}</td>
                <td className="text-secondary">{u.email}</td>
                <td><span className="status-badge" style={{ background: u.auth_source === "local" ? "var(--surface-alt)" : "rgba(8, 145, 178, 0.1)", color: u.auth_source === "local" ? "var(--text-secondary)" : "var(--accent)" }}>{u.auth_source === "local" ? "本地" : "LDAP"}</span></td>
                <td>{u.is_admin ? "管理员" : "成员"}</td>
                {isAdmin && (
                  <td>
                    <button className="btn btn-ghost btn-sm" onClick={() => handleRole(u)}>
                      {u.is_admin ? "取消管理员" : "设为管理员"}
                    </button>{" "}
                    <button className="btn btn-ghost btn-sm" style={{ color: "var(--danger)" }} onClick={() => handleDelete(u)}>
                      删除
                    </button>
                  </td>
                )}
              </tr>
            ))}
            {users.length === 0 && (
              <tr><td colSpan={isAdmin ? 6 : 5} className="text-secondary" style={{ textAlign: "center", padding: 32 }}>暂无用户</td></tr>
            )}
          </tbody>
        </table>
      )}
      {message && <div className="form-error" style={{ marginTop: 12 }}>{message}</div>}
    </div>
  );
}
