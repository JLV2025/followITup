import { getErrorMessage } from "../utils/errorMsg";
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
  // 到期提醒
  const [dueReminderOn, setDueReminderOn] = useState(false);
  const [dueReminderDays, setDueReminderDays] = useState(3);
  // 节假日
  const [holidays, setHolidays] = useState<Holiday[]>([]);
  const [holidayStart, setHolidayStart] = useState("");
  const [holidayEnd, setHolidayEnd] = useState("");
  const [holidayType, setHolidayType] = useState("holiday");
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
      setDueReminderOn(d.due_reminder_enabled === "1");
      setDueReminderDays(Number(d.due_reminder_days) || 3);
    }).catch(() => setMessage("加载配置失败"));
    fetchHolidays();
  }, []);

  const saveSettings = async (patch: Record<string, any>, okMsg: string) => {
    try {
      await api.put("/api/settings", patch);
      setMessage(okMsg);
    } catch (err: any) {
      setMessage(getErrorMessage(err, "common.unknownError"));
    }
  };

  const testEmail = async () => {
    const to = window.prompt("输入测试收件邮箱：");
    if (!to) return;
    try {
      await api.post("/api/settings/test-email", { to });
      setMessage("测试邮件已发送");
    } catch (err: any) {
      setMessage("发送失败: " + (getErrorMessage(err, "common.unknownError")));
    }
  };

  const runReminder = async () => {
    try {
      await api.post("/api/settings/reminder/run", {});
      setMessage("到期提醒已扫描并发送（见服务端日志）");
    } catch (err: any) {
      setMessage("提醒发送失败: " + (getErrorMessage(err, "common.unknownError")));
    }
  };

  const addHolidayRange = async () => {
    if (!holidayStart) { setMessage("请选择开始日期"); return; }
    try {
      const res = await api.post("/api/calendar", {
        start_date: holidayStart,
        end_date: holidayEnd || undefined,
        type: holidayType,
        label: holidayLabel,
      });
      setMessage(res.data?.data?.message || "已添加");
      setHolidayStart(""); setHolidayEnd(""); setHolidayLabel("");
      fetchHolidays();
    } catch (err: any) {
      setMessage(getErrorMessage(err, "common.unknownError"));
    }
  };

  const inputStyle = {
    width: "100%", padding: "8px 10px", borderRadius: 6,
    border: "1px solid var(--card-border)", fontSize: 14,
  };

  return (
    <div style={{ maxWidth: 1100, margin: "0 auto" }}>
      <div className="dashboard-header-row" style={{ marginBottom: 16 }}>
        <div>
          <h1 style={{ fontSize: 22, fontWeight: 600, marginBottom: 2 }}>系统设置</h1>
          <p className="text-secondary" style={{ fontSize: 13 }}>SMTP 邮件、财年、节假日与密码策略（仅管理员）</p>
        </div>
      </div>

      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 14 }}>
        {/* SMTP */}
        <div style={{ background: "var(--card-bg)", border: "1px solid var(--card-border)", borderRadius: 8, padding: 14 }}>
          <h3 style={{ fontSize: 15, fontWeight: 600, marginBottom: 10 }}>邮件通知（SMTP）</h3>
          <div className="form-row" style={{ gap: 10 }}>
            <div className="form-group" style={{ marginBottom: 8, flex: 2 }}>
              <label style={{ fontSize: 13 }}>服务器地址</label>
              <input style={inputStyle} value={smtp.smtp_host}
                onChange={(e) => setSmtp({ ...smtp, smtp_host: e.target.value })} placeholder="smtp.example.com" />
            </div>
            <div className="form-group" style={{ marginBottom: 8, flex: 1 }}>
              <label style={{ fontSize: 13 }}>端口</label>
              <input style={inputStyle} value={smtp.smtp_port}
                onChange={(e) => setSmtp({ ...smtp, smtp_port: e.target.value })} />
            </div>
          </div>
          <div className="form-row" style={{ gap: 10 }}>
            <div className="form-group" style={{ marginBottom: 8, flex: 1 }}>
              <label style={{ fontSize: 13 }}>发件人</label>
              <input style={inputStyle} value={smtp.smtp_sender}
                onChange={(e) => setSmtp({ ...smtp, smtp_sender: e.target.value })} placeholder="FollowITup@qorvo.com" />
            </div>
            <div className="form-group" style={{ marginBottom: 8, flex: 1 }}>
              <label style={{ fontSize: 13 }}>认证用户名（留空 = 无需登录）</label>
              <input style={inputStyle} value={smtp.smtp_username}
                onChange={(e) => setSmtp({ ...smtp, smtp_username: e.target.value })} />
            </div>
          </div>
          <div className="form-group" style={{ marginBottom: 8 }}>
            <label style={{ fontSize: 13 }}>认证密码</label>
            <input style={inputStyle} type="password" value={smtp.smtp_password}
              onChange={(e) => setSmtp({ ...smtp, smtp_password: e.target.value })} />
          </div>
          <div className="form-row">
            <button className="btn btn-primary" onClick={() => saveSettings(smtp, "SMTP 配置已保存")}>保存</button>
            <button className="btn btn-ghost" onClick={testEmail}>测试发送</button>
          </div>
          {/* 到期提醒 */}
          <hr style={{ border: "none", borderTop: "1px solid var(--card-border)", margin: "14px 0" }} />
          <div style={{ fontSize: 13, marginBottom: 8 }}>任务到期提醒（每日 9:00 扫描，按负责人汇总发送）</div>
          <div className="form-row" style={{ gap: 10, alignItems: "center" }}>
            <label style={{ fontSize: 13, display: "flex", alignItems: "center", gap: 6 }}>
              <input
                type="checkbox"
                checked={dueReminderOn}
                onChange={(e) => {
                  setDueReminderOn(e.target.checked);
                  saveSettings({ due_reminder_enabled: e.target.checked ? "1" : "0" }, e.target.checked ? "到期提醒已开启" : "到期提醒已关闭");
                }}
              />
              开启提醒
            </label>
            <label style={{ fontSize: 13, display: "flex", alignItems: "center", gap: 6 }}>
              提前
              <input
                type="number"
                min={1}
                max={30}
                style={{ ...inputStyle, width: 56, padding: "4px 6px" }}
                value={dueReminderDays}
                onChange={(e) => {
                  setDueReminderDays(Number(e.target.value) || 3);
                  saveSettings({ due_reminder_days: String(Number(e.target.value) || 3) }, "提前天数已保存");
                }}
              />
              天
            </label>
            <button className="btn btn-ghost btn-sm" onClick={runReminder} title="立即扫描并发送一次(验证用)">立即发送一次</button>
          </div>
        </div>

        {/* 财年 + 密码策略 */}
        <div style={{ background: "var(--card-bg)", border: "1px solid var(--card-border)", borderRadius: 8, padding: 14 }}>
          <h3 style={{ fontSize: 15, fontWeight: 600, marginBottom: 10 }}>财年与密码</h3>
          <div className="form-row" style={{ gap: 10 }}>
            <div className="form-group" style={{ marginBottom: 8, flex: 1 }}>
              <label style={{ fontSize: 13 }}>财年起始月份（首页只读）</label>
              <select style={inputStyle} value={fiscalStartMonth}
                onChange={(e) => setFiscalStartMonth(Number(e.target.value))}>
                {[1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12].map((m) => (
                  <option key={m} value={m}>{m} 月起始</option>
                ))}
              </select>
            </div>
            <div className="form-group" style={{ marginBottom: 8, flex: 1 }}>
              <label style={{ fontSize: 13 }}>密码最小长度</label>
              <input style={inputStyle} type="number" min={6} max={32} value={passwordMinLength}
                onChange={(e) => setPasswordMinLength(Number(e.target.value))} />
            </div>
          </div>
          <button className="btn btn-primary"
            onClick={() => saveSettings({ fiscal_start_month: fiscalStartMonth, password_min_length: passwordMinLength }, "财年与密码设置已保存")}>
            保存
          </button>
        </div>
      </div>

      {/* 节假日与补班 */}
      <div style={{ background: "var(--card-bg)", border: "1px solid var(--card-border)", borderRadius: 8, padding: 14, marginTop: 14 }}>
        <h3 style={{ fontSize: 15, fontWeight: 600, marginBottom: 10 }}>节假日与补班（假日排除工作日 / 补班周末计工作日）</h3>
        <div className="form-row" style={{ gap: 8 }}>
          <div className="form-group" style={{ marginBottom: 8 }}>
            <label style={{ fontSize: 13 }}>开始日期</label>
            <input style={inputStyle} type="date" value={holidayStart}
              onChange={(e) => setHolidayStart(e.target.value)} />
          </div>
          <div className="form-group" style={{ marginBottom: 8 }}>
            <label style={{ fontSize: 13 }}>结束日期</label>
            <input style={inputStyle} type="date" value={holidayEnd}
              onChange={(e) => setHolidayEnd(e.target.value)} />
          </div>
          <div className="form-group" style={{ marginBottom: 8 }}>
            <label style={{ fontSize: 13 }}>类型</label>
            <select style={inputStyle} value={holidayType}
              onChange={(e) => setHolidayType(e.target.value)}>
              <option value="holiday">假日（排除工作日）</option>
              <option value="workday">补班（周末计工作日）</option>
            </select>
          </div>
          <div className="form-group" style={{ marginBottom: 8, flex: 1 }}>
            <label style={{ fontSize: 13 }}>名称</label>
            <input style={inputStyle} placeholder="如：春节" value={holidayLabel}
              onChange={(e) => setHolidayLabel(e.target.value)} />
          </div>
          <div className="form-group" style={{ marginBottom: 8, alignSelf: "flex-end" }}>
            <button className="btn btn-primary" onClick={addHolidayRange}>添加</button>
          </div>
        </div>
        <table className="task-table" style={{ marginTop: 8 }}>
          <thead>
            <tr>
              <th>日期</th>
              <th style={{ width: 70 }}>类型</th>
              <th>名称</th>
              <th style={{ width: 80 }}>操作</th>
            </tr>
          </thead>
          <tbody>
            {holidays.map((h) => (
              <tr key={h.id}>
                <td>{h.date}</td>
                <td>
                  <span className="status-badge"
                    style={h.type === "workday"
                      ? { background: "rgba(8, 145, 178, 0.1)", color: "var(--accent)" }
                      : { background: "var(--surface-alt)", color: "var(--text-secondary)" }}>
                    {h.type === "workday" ? "补班" : "假日"}
                  </span>
                </td>
                <td>{h.label || "—"}</td>
                <td>
                  <button className="btn btn-ghost btn-sm" onClick={async () => {
                    try {
                      await api.delete(`/api/calendar/${h.id}`);
                      fetchHolidays();
                    } catch (err: any) {
                      setMessage(getErrorMessage(err, "common.unknownError"));
                    }
                  }}>删除</button>
                </td>
              </tr>
            ))}
            {holidays.length === 0 && (
              <tr><td colSpan={4} className="text-secondary">暂无节假日</td></tr>
            )}
          </tbody>
        </table>
      </div>

      {message && <div className="form-error" style={{ marginTop: 16 }}>{message}</div>}
    </div>
  );
}
