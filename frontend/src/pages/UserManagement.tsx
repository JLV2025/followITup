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
  const [password, setPassword] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [error, setError] = useState("");
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
    if (!email || !password) { setError("邮箱和密码不能为空"); return; }
    if (password.length < 6) { setError("密码长度不少于6位"); return; }
    setCreating(true);
    try {
      await api.post("/api/admin/users", { email: email.trim(), password, display_name: displayName.trim() || email.trim() });
      setEmail(""); setPassword(""); setDisplayName(""); setShowForm(false);
      fetchUsers();
    } catch (err: any) {
      setError(err.response?.data?.error?.message || "创建失败");
    }
    setCreating(false);
  };

  if (!isAdmin) return <p className="text-secondary p-4">仅管理员可访问</p>;

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
          {error && <div className="form-error">{error}</div>}
          <div className="form-row">
            <div className="form-group">
              <label>邮箱</label>
              <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} placeholder="user@example.com" autoFocus />
            </div>
            <div className="form-group">
              <label>显示名称</label>
              <input value={displayName} onChange={(e) => setDisplayName(e.target.value)} placeholder="张三" />
            </div>
          </div>
          <div className="form-group">
            <label>密码</label>
            <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} placeholder="至少6位" />
          </div>
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
              </tr>
            ))}
            {users.length === 0 && (
              <tr><td colSpan={5} className="text-secondary" style={{ textAlign: "center", padding: 32 }}>暂无用户</td></tr>
            )}
          </tbody>
        </table>
      )}
    </div>
  );
}
