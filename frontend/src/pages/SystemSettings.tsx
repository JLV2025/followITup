import { useEffect, useState } from "react";
import api from "../api/client";

interface Holiday {
  id: number;
  date: string;
  type: string;
  label: string;
}

export default function SystemSettings() {
  // SMTP 配置
  const [smtp, setSmtp] = useState({ smtp_host: "", smtp_port: "25", smtp_username: "", smtp_password: "", smtp_sender: "" });
  // 财年 + 密码策略
  const [fiscalStartMonth, setFiscalStartMonth] = useState(4);
  const [passwordMinLength, setPasswordMinLength] = useState(8);
  // 节假日
  const [holidays, setHolidays] = useState<Holiday[]>([]);
  const [holidayDate, setHolidayDate] = useState("");
  const [holidayLabel, setHolidayLabel] = useState("");
  const [message, setMessage] = useState("");

  const fetchHolidays = async () => {
    try {
      const res = await api.get("/api/calendar");
      setHolidays(res.data?.data || []);
    } catch {
      setMessage("加载节假日失败");
    }
  };

  useEffect(() => {
    api.get("/api/settings/admin").then((res) => {
      const d = res.data?.data || {};
      setSmtp({
        smtp_host: d.smtp_host || "",
        smtp_port: d.smtp_port || "25",
        smtp_username: d.smtp_username || "",
        smtp_password: d.smtp_password || "",
        smtp_sender: d.smtp_sender || "",
      });
      setFiscalStartMonth(Number(d.fiscal_start_month) || 4);
      setPasswordMinLength(Number(d.password_min_length) || 8);
    }).catch(() => setMessage("加载配置失败"));
    fetchHolidays();
  }, []);

  const saveSettings = async (patch: Record<string, any>, okMsg: string) => {
    try {
      await api.put("/api/settings", patch);
      setMessage(okMsg);
    } catch (err: any) {
      setMessage(err?.response?.data?.error?.message || "保存失败");
    }
  };

  const testEmail = async () => {
    const to = window.prompt("输入测试收件邮箱：");
    if (!to) return;
    try {
      await api.post("/api/settings/test-email", { to });
      setMessage("测试邮件已发送");
    } catch (err: any) {
      setMessage("发送失败: " + (err?.response?.data?.error?.message || ""));
    }
  };

  const inputStyle = {
    width: "100%", padding: "8px 10px", borderRadius: 6,
    border: "1px solid var(--card-border)", fontSize: 14,
  };

  return (
    <div style={{ maxWidth: 960, margin: "0 auto" }}>
      <div className="dashboard-header-row" style={{ marginBottom: 24 }}>
        <div>
          <h1 style={{ fontSize: 24, fontWeight: 600, marginBottom: 4 }}>系统设置</h1>
          <p className="text-secondary">SMTP 邮件、财年、节假日与密码策略（仅管理员）</p>
        </div>
      </div>

      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 20 }}>
        {/* SMTP */}
        <div style={{ background: "var(--card-bg)", border: "1px solid var(--card-border)", borderRadius: 8, padding: 20 }}>
          <h3 style={{ fontSize: 16, fontWeight: 600, marginBottom: 16 }}>邮件通知（SMTP）</h3>
          <div className="form-group">
            <label>服务器地址</label>
            <input style={inputStyle} value={smtp.smtp_host}
              onChange={(e) => setSmtp({ ...smtp, smtp_host: e.target.value })} placeholder="smtp.example.com" />
          </div>
          <div className="form-group">
            <label>端口</label>
            <input style={inputStyle} value={smtp.smtp_port}
              onChange={(e) => setSmtp({ ...smtp, smtp_port: e.target.value })} />
          </div>
          <div className="form-group">
            <label>发件人</label>
            <input style={inputStyle} value={smtp.smtp_sender}
              onChange={(e) => setSmtp({ ...smtp, smtp_sender: e.target.value })} placeholder="FollowITup@qorvo.com" />
          </div>
          <div className="form-group">
            <label>认证用户名（留空 = 无需登录）</label>
            <input style={inputStyle} value={smtp.smtp_username}
              onChange={(e) => setSmtp({ ...smtp, smtp_username: e.target.value })} />
          </div>
          <div className="form-group">
            <label>认证密码</label>
            <input style={inputStyle} type="password" value={smtp.smtp_password}
              onChange={(e) => setSmtp({ ...smtp, smtp_password: e.target.value })} />
          </div>
          <div className="form-row">
            <button className="btn btn-primary" onClick={() => saveSettings(smtp, "SMTP 配置已保存")}>保存</button>
            <button className="btn btn-ghost" onClick={testEmail}>测试发送</button>
          </div>
        </div>

        {/* 财年 + 密码策略 */}
        <div style={{ background: "var(--card-bg)", border: "1px solid var(--card-border)", borderRadius: 8, padding: 20 }}>
          <h3 style={{ fontSize: 16, fontWeight: 600, marginBottom: 16 }}>财年与密码</h3>
          <div className="form-group">
            <label>财年起始月份（首页只读）</label>
            <select style={inputStyle} value={fiscalStartMonth}
              onChange={(e) => setFiscalStartMonth(Number(e.target.value))}>
              {[1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12].map((m) => (
                <option key={m} value={m}>{m} 月起始</option>
              ))}
            </select>
          </div>
          <div className="form-group">
            <label>密码最小长度（随机初始密码/改密校验）</label>
            <input style={inputStyle} type="number" min={6} max={32} value={passwordMinLength}
              onChange={(e) => setPasswordMinLength(Number(e.target.value))} />
          </div>
          <button className="btn btn-primary"
            onClick={() => saveSettings({ fiscal_start_month: fiscalStartMonth, password_min_length: passwordMinLength }, "财年与密码设置已保存")}>
            保存
          </button>
        </div>
      </div>

      {/* 节假日 */}
      <div style={{ background: "var(--card-bg)", border: "1px solid var(--card-border)", borderRadius: 8, padding: 20, marginTop: 20 }}>
        <h3 style={{ fontSize: 16, fontWeight: 600, marginBottom: 16 }}>节假日（排程自动排除）</h3>
        <div className="form-row">
          <div className="form-group">
            <label>日期</label>
            <input style={inputStyle} type="date" value={holidayDate}
              onChange={(e) => setHolidayDate(e.target.value)} />
          </div>
          <div className="form-group">
            <label>名称</label>
            <input style={inputStyle} placeholder="如：春节" value={holidayLabel}
              onChange={(e) => setHolidayLabel(e.target.value)} />
          </div>
          <div className="form-group" style={{ alignSelf: "flex-end" }}>
            <button className="btn btn-primary" onClick={async () => {
              if (!holidayDate) { setMessage("请选择日期"); return; }
              try {
                await api.post("/api/calendar", { date: holidayDate, type: "holiday", label: holidayLabel });
                setHolidayDate(""); setHolidayLabel("");
                fetchHolidays();
                setMessage("节假日已添加");
              } catch (err: any) {
                setMessage(err?.response?.data?.error?.message || "添加失败");
              }
            }}>新增</button>
          </div>
        </div>
        <table className="task-table" style={{ marginTop: 16 }}>
          <thead>
            <tr>
              <th>日期</th>
              <th>名称</th>
              <th style={{ width: 80 }}>操作</th>
            </tr>
          </thead>
          <tbody>
            {holidays.map((h) => (
              <tr key={h.id}>
                <td>{h.date}</td>
                <td>{h.label || "—"}</td>
                <td>
                  <button className="btn btn-ghost btn-sm" onClick={async () => {
                    try {
                      await api.delete(`/api/calendar/${h.id}`);
                      fetchHolidays();
                    } catch (err: any) {
                      setMessage(err?.response?.data?.error?.message || "删除失败");
                    }
                  }}>删除</button>
                </td>
              </tr>
            ))}
            {holidays.length === 0 && (
              <tr><td colSpan={3} className="text-secondary">暂无节假日</td></tr>
            )}
          </tbody>
        </table>
      </div>

      {message && <div className="form-error" style={{ marginTop: 16 }}>{message}</div>}
    </div>
  );
}
