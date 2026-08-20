import { getErrorMessage } from "../utils/errorMsg";
import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import api from "../api/client";
import i18n from "../i18n";

interface Holiday {
  id: number;
  date: string;
  type: string;
  label: string;
}

export default function SystemSettings() {
  const { t } = useTranslation();
  // SMTP 配置
  const [smtp, setSmtp] = useState({ smtp_host: "", smtp_port: "25", smtp_username: "", smtp_password: "", smtp_sender: "" });
  // 部署网址（邮件发送前提：邮件必须带服务器地址，未填写不发邮件）
  const [baseURL, setBaseURL] = useState("");
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
  // 栏目分页：邮件通知 / 财年与密码 / 节假日
  const [activeTab, setActiveTab] = useState<"smtp" | "fiscal" | "holiday">("smtp");

  const fetchHolidays = async () => {
    try {
      const res = await api.get("/api/calendar");
      setHolidays(res.data?.data || []);
    } catch {
      setMessage(i18n.t("settingsPage.loadHolidayFail"));
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
      setBaseURL(d.base_url || "");
    }).catch(() => setMessage(i18n.t("settingsPage.loadConfigFail")));
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
    const to = window.prompt(i18n.t("settingsPage.promptTestEmail"));
    if (!to) return;
    try {
      await api.post("/api/settings/test-email", { to });
      setMessage(i18n.t("settingsPage.testSent"));
    } catch (err: any) {
      setMessage(getErrorMessage(err, "settingsPage.sendFail"));
    }
  };

  const runReminder = async () => {
    try {
      await api.post("/api/settings/reminder/run", {});
      setMessage(i18n.t("settingsPage.reminderSent"));
    } catch (err: any) {
      setMessage(getErrorMessage(err, "settingsPage.reminderFail"));
    }
  };

  const addHolidayRange = async () => {
    if (!holidayStart) { setMessage(i18n.t("settingsPage.pickStartDate")); return; }
    try {
      const res = await api.post("/api/calendar", {
        start_date: holidayStart,
        end_date: holidayEnd || undefined,
        type: holidayType,
        label: holidayLabel,
      });
      setMessage(res.data?.data?.message || i18n.t("settingsPage.added"));
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

  // 分页标签样式：激活标签高亮底部边框
  const tabStyle = (k: "smtp" | "fiscal" | "holiday") => ({
    padding: "8px 16px",
    border: "none",
    background: "none",
    borderBottom: activeTab === k ? "2px solid var(--primary)" : "2px solid transparent",
    color: activeTab === k ? "var(--primary)" : "var(--text-secondary)",
    fontWeight: activeTab === k ? 600 : 400,
    cursor: "pointer",
    fontSize: 14,
    marginBottom: -1,
  });

  return (
    <div style={{ maxWidth: 800, margin: "0 auto" }}>
      <div className="dashboard-header-row" style={{ marginBottom: 16 }}>
        <div>
          <h1 style={{ fontSize: 22, fontWeight: 600, marginBottom: 2 }}>{t("settingsPage.title")}</h1>
          <p className="text-secondary" style={{ fontSize: 13 }}>{t("settingsPage.subtitle")}</p>
        </div>
        <Link to="/" className="btn btn-ghost btn-sm">{t("nav.backDashboard")}</Link>
      </div>

      <div style={{ display: "flex", gap: 8, marginBottom: 14, borderBottom: "1px solid var(--card-border)" }}>
        <button style={tabStyle("smtp")} onClick={() => setActiveTab("smtp")}>{t("settingsPage.tabSmtp")}</button>
        <button style={tabStyle("fiscal")} onClick={() => setActiveTab("fiscal")}>{t("settingsPage.tabFiscal")}</button>
        <button style={tabStyle("holiday")} onClick={() => setActiveTab("holiday")}>{t("settingsPage.tabHoliday")}</button>
      </div>

      {activeTab === "smtp" && (
        <div style={{ background: "var(--card-bg)", border: "1px solid var(--card-border)", borderRadius: 8, padding: 14 }}>
          <h3 style={{ fontSize: 15, fontWeight: 600, marginBottom: 10 }}>{t("settingsPage.sectionSmtp")}</h3>
          {/* 部署网址：邮件发送前提 */}
          <div className="form-group" style={{ marginBottom: 12 }}>
            <label style={{ fontSize: 13 }}>{t("settingsPage.baseUrl")}</label>
            <div style={{ display: "flex", gap: 8 }}>
              <input style={{ ...inputStyle, flex: 1 }} value={baseURL}
                placeholder="https://server.example.com:8080"
                onChange={(e) => setBaseURL(e.target.value)} />
              <button className="btn btn-primary" onClick={() => saveSettings({ base_url: baseURL }, i18n.t("settingsPage.baseUrlSaved"))}>{t("settingsPage.save")}</button>
            </div>
            <span className="form-hint">{t("settingsPage.baseUrlHint")}</span>
          </div>
          <hr style={{ border: "none", borderTop: "1px solid var(--card-border)", margin: "12px 0 14px" }} />
          <div className="form-row" style={{ gap: 10 }}>
            <div className="form-group" style={{ marginBottom: 8, flex: 2 }}>
              <label style={{ fontSize: 13 }}>{t("settingsPage.smtpHost")}</label>
              <input style={inputStyle} value={smtp.smtp_host}
                onChange={(e) => setSmtp({ ...smtp, smtp_host: e.target.value })} placeholder="smtp.example.com" />
            </div>
            <div className="form-group" style={{ marginBottom: 8, flex: 1 }}>
              <label style={{ fontSize: 13 }}>{t("settingsPage.smtpPort")}</label>
              <input style={inputStyle} value={smtp.smtp_port}
                onChange={(e) => setSmtp({ ...smtp, smtp_port: e.target.value })} />
            </div>
          </div>
          <div className="form-row" style={{ gap: 10 }}>
            <div className="form-group" style={{ marginBottom: 8, flex: 1 }}>
              <label style={{ fontSize: 13 }}>{t("settingsPage.smtpSender")}</label>
              <input style={inputStyle} value={smtp.smtp_sender}
                onChange={(e) => setSmtp({ ...smtp, smtp_sender: e.target.value })} placeholder="FollowITup@qorvo.com" />
            </div>
            <div className="form-group" style={{ marginBottom: 8, flex: 1 }}>
              <label style={{ fontSize: 13 }}>{t("settingsPage.smtpUser")}</label>
              <input style={inputStyle} value={smtp.smtp_username}
                onChange={(e) => setSmtp({ ...smtp, smtp_username: e.target.value })} />
            </div>
          </div>
          <div className="form-group" style={{ marginBottom: 8 }}>
            <label style={{ fontSize: 13 }}>{t("settingsPage.smtpPass")}</label>
            <input style={inputStyle} type="password" value={smtp.smtp_password}
              onChange={(e) => setSmtp({ ...smtp, smtp_password: e.target.value })} />
          </div>
          <div className="form-row">
            <button className="btn btn-primary" onClick={() => saveSettings(smtp, i18n.t("settingsPage.smtpSaved"))}>{t("settingsPage.save")}</button>
            <button className="btn btn-ghost" onClick={testEmail}>{t("settingsPage.testSend")}</button>
          </div>
          {/* 到期提醒 */}
          <hr style={{ border: "none", borderTop: "1px solid var(--card-border)", margin: "14px 0" }} />
          <div style={{ fontSize: 13, marginBottom: 8 }}>{t("settingsPage.dueReminder")}</div>
          <div className="form-row" style={{ gap: 10, alignItems: "center" }}>
            <label style={{ fontSize: 13, display: "flex", alignItems: "center", gap: 6 }}>
              <input
                type="checkbox"
                checked={dueReminderOn}
                onChange={(e) => {
                  setDueReminderOn(e.target.checked);
                  saveSettings({ due_reminder_enabled: e.target.checked ? "1" : "0" }, e.target.checked ? i18n.t("settingsPage.reminderOn") : i18n.t("settingsPage.reminderOff"));
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
                  saveSettings({ due_reminder_days: String(Number(e.target.value) || 3) }, i18n.t("settingsPage.daysSaved"));
                }}
              />
              天
            </label>
            <button className="btn btn-ghost btn-sm" onClick={runReminder} title={t("settingsPage.runNowTitle")}>{t("settingsPage.runNow")}</button>
          </div>
        </div>
      )}

      {activeTab === "fiscal" && (
        <div style={{ background: "var(--card-bg)", border: "1px solid var(--card-border)", borderRadius: 8, padding: 14 }}>
          <h3 style={{ fontSize: 15, fontWeight: 600, marginBottom: 10 }}>{t("settingsPage.sectionFiscal")}</h3>
          <div className="form-row" style={{ gap: 10 }}>
            <div className="form-group" style={{ marginBottom: 8, flex: 1 }}>
              <label style={{ fontSize: 13 }}>{t("settingsPage.fiscalMonth")}</label>
              <select style={inputStyle} value={fiscalStartMonth}
                onChange={(e) => setFiscalStartMonth(Number(e.target.value))}>
                {[1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12].map((m) => (
                  <option key={m} value={m}>{t("settingsPage.monthStart", { m })}</option>
                ))}
              </select>
            </div>
            <div className="form-group" style={{ marginBottom: 8, flex: 1 }}>
              <label style={{ fontSize: 13 }}>{t("settingsPage.pwdMinLen")}</label>
              <input style={inputStyle} type="number" min={6} max={32} value={passwordMinLength}
                onChange={(e) => setPasswordMinLength(Number(e.target.value))} />
            </div>
          </div>
          <button className="btn btn-primary"
            onClick={() => saveSettings({ fiscal_start_month: fiscalStartMonth, password_min_length: passwordMinLength }, i18n.t("settingsPage.fiscalSaved"))}>
            保存
          </button>
        </div>
      )}

      {activeTab === "holiday" && (
        <div style={{ background: "var(--card-bg)", border: "1px solid var(--card-border)", borderRadius: 8, padding: 14 }}>
        <h3 style={{ fontSize: 15, fontWeight: 600, marginBottom: 10 }}>{t("settingsPage.sectionHoliday")}</h3>
        <div className="form-row" style={{ gap: 8 }}>
          <div className="form-group" style={{ marginBottom: 8 }}>
            <label style={{ fontSize: 13 }}>{t("settingsPage.holidayStart")}</label>
            <input style={inputStyle} type="date" value={holidayStart}
              onChange={(e) => setHolidayStart(e.target.value)} />
          </div>
          <div className="form-group" style={{ marginBottom: 8 }}>
            <label style={{ fontSize: 13 }}>{t("settingsPage.holidayEnd")}</label>
            <input style={inputStyle} type="date" value={holidayEnd}
              onChange={(e) => setHolidayEnd(e.target.value)} />
          </div>
          <div className="form-group" style={{ marginBottom: 8 }}>
            <label style={{ fontSize: 13 }}>{t("settingsPage.holidayType")}</label>
            <select style={inputStyle} value={holidayType}
              onChange={(e) => setHolidayType(e.target.value)}>
              <option value="holiday">{t("settingsPage.holidayTypeHoliday")}</option>
              <option value="workday">{t("settingsPage.holidayTypeWorkday")}</option>
            </select>
          </div>
          <div className="form-group" style={{ marginBottom: 8, flex: 1 }}>
            <label style={{ fontSize: 13 }}>{t("settingsPage.holidayName")}</label>
            <input style={inputStyle} placeholder={t("settingsPage.holidayNamePlaceholder")} value={holidayLabel}
              onChange={(e) => setHolidayLabel(e.target.value)} />
          </div>
          <div className="form-group" style={{ marginBottom: 8, alignSelf: "flex-end" }}>
            <button className="btn btn-primary" onClick={addHolidayRange}>{t("settingsPage.holidayAdd")}</button>
          </div>
        </div>
        <table className="task-table" style={{ marginTop: 8 }}>
          <thead>
            <tr>
              <th>{t("settingsPage.colDate")}</th>
              <th style={{ width: 70 }}>{t("settingsPage.colType")}</th>
              <th>{t("settingsPage.colName")}</th>
              <th style={{ width: 80 }}>{t("settingsPage.colActions")}</th>
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
                    {h.type === "workday" ? t("settingsPage.typeWorkday") : t("settingsPage.typeHoliday")}
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
                  }}>{t("common.delete")}</button>
                </td>
              </tr>
            ))}
            {holidays.length === 0 && (
              <tr><td colSpan={4} className="text-secondary">{t("settingsPage.holidayEmpty")}</td></tr>
            )}
          </tbody>
        </table>
      </div>
      )}

      {message && <div className="form-error" style={{ marginTop: 16 }}>{message}</div>}
    </div>
  );
}
