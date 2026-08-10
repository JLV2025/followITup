package mail

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"followitup/internal/settings"
)

// StartDueReminderScheduler 每日 09:00 触发一次到期提醒扫描发送。
// ctx 取消（服务关闭）时立即退出，不泄漏 goroutine。
func StartDueReminderScheduler(ctx context.Context, db *sql.DB) {
	for {
		now := time.Now()
		next := time.Date(now.Year(), now.Month(), now.Day(), 9, 0, 0, 0, now.Location())
		if !now.Before(next) {
			next = next.Add(24 * time.Hour)
		}
		select {
		case <-time.After(time.Until(next)):
			if _, err := RunDueReminder(db); err != nil {
				log.Printf("[Reminder] 每日到期提醒失败: %v", err)
			}
		case <-ctx.Done():
			return
		}
	}
}

// DueTask 到期任务（按负责人汇总）
type DueTask struct {
	ProjectName string
	TaskName    string
	EndDate     string
}

// RunDueReminder 执行一次到期提醒扫描并发送汇总邮件。
// 开关未开启时直接返回 0；返回发送的收件人数（错误邮件数累加但继续）。
func RunDueReminder(db *sql.DB) (int, error) {
	if v, _ := settings.Get(db, settings.KeyDueReminderOn); v != "1" {
		return 0, nil
	}
	days := settings.GetInt(db, settings.KeyDueReminderDays, 3)
	if days < 1 {
		days = 1
	}

	// 窗口边界用 Go 侧本地日期（服务器时区，与 9:00 触发一致），
	// 不用 SQLite 的 date('now')（UTC，对中国时区服务器会整体偏移一天）
	today := time.Now().Format("2006-01-02")
	deadline := time.Now().AddDate(0, 0, days).Format("2006-01-02")

	// 未来 N 天内到期、未完成、有关联负责人(且活跃)的任务。
	// 通过 task_assignees 关联表匹配多负责人，JOIN 天然去重，每个活跃负责人各收一封。
	rows, err := db.Query(`
		SELECT p.name, t.name, t.end_date, u.email
		FROM tasks t
		JOIN projects p ON p.id = t.project_id
		JOIN task_assignees ta ON ta.task_id = t.id
		JOIN users u ON u.id = ta.user_id AND u.is_active = 1
		WHERE t.deleted_at IS NULL AND p.deleted_at IS NULL
		  AND t.status != 'completed'
		  AND t.end_date != ''
		  AND t.end_date >= ?
		  AND t.end_date <= ?
		ORDER BY u.email, t.end_date`, today, deadline)
	if err != nil {
		return 0, fmt.Errorf("扫描到期任务失败: %w", err)
	}
	defer rows.Close()

	groups := map[string][]DueTask{}
	var order []string
	for rows.Next() {
		var d DueTask
		var email string
		if err := rows.Scan(&d.ProjectName, &d.TaskName, &d.EndDate, &email); err != nil {
			continue
		}
		if _, ok := groups[email]; !ok {
			order = append(order, email)
		}
		groups[email] = append(groups[email], d)
	}

	sent := 0
	var lastErr error
	for _, email := range order {
		list := groups[email]
		body := fmt.Sprintf("Hi,\n\nThe following %d task(s) are due within %d day(s). Please follow up:\n\n", len(list), days)
		for _, t := range list {
			body += fmt.Sprintf("  • [%s] %s — due %s\n", t.ProjectName, t.TaskName, t.EndDate)
		}
		body += "\n— FollowITup"
		if err := Send(db, email, "FollowITup Task Due Reminder", body); err != nil {
			log.Printf("[Reminder] 发送到期提醒失败 %s: %v", email, err)
			lastErr = err
			continue
		}
		sent++
	}
	return sent, lastErr
}
