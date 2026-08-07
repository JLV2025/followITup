package mail

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"followitup/internal/settings"
)

// StartDueReminderScheduler 每日 09:00 触发一次到期提醒扫描发送
func StartDueReminderScheduler(db *sql.DB) {
	for {
		now := time.Now()
		next := time.Date(now.Year(), now.Month(), now.Day(), 9, 0, 0, 0, now.Location())
		if !now.Before(next) {
			next = next.Add(24 * time.Hour)
		}
		time.Sleep(time.Until(next))
		if _, err := RunDueReminder(db); err != nil {
			log.Printf("[Reminder] 每日到期提醒失败: %v", err)
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

	// 未来 N 天内到期、未完成、有负责人(且能解析出邮箱)的任务
	rows, err := db.Query(`
		SELECT p.name, t.name, t.end_date, u.email
		FROM tasks t
		JOIN projects p ON p.id = t.project_id
		JOIN users u ON (u.email = t.assignee OR u.display_name = t.assignee)
		WHERE t.deleted_at IS NULL AND p.deleted_at IS NULL
		  AND u.is_active = 1
		  AND t.status != 'completed'
		  AND t.end_date != ''
		  AND t.end_date >= date('now')
		  AND t.end_date <= date('now', '+' || ? || ' days')
		ORDER BY u.email, t.end_date`, days)
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
		body := fmt.Sprintf("你好：\n\n以下 %d 个任务将在 %d 天内到期，请及时跟进：\n\n", len(list), days)
		for _, t := range list {
			body += fmt.Sprintf("  • [%s] %s —— 截止 %s\n", t.ProjectName, t.TaskName, t.EndDate)
		}
		body += "\n—— FollowITup 系统通知"
		if err := Send(db, email, "FollowITup 任务到期提醒", body); err != nil {
			log.Printf("[Reminder] 发送到期提醒失败 %s: %v", email, err)
			lastErr = err
			continue
		}
		sent++
	}
	return sent, lastErr
}
