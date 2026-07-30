package scheduler

import (
	"database/sql"
	"fmt"
)

// CalendarEntry 日历条目
type CalendarEntry struct {
	ID    int64
	Date  string // YYYY-MM-DD
	Type  string // "holiday" | "workday"
	Label string
}

// LoadCalendar 加载指定日期范围内的日历条目
func LoadCalendar(db *sql.DB, start, end string) (map[string]string, error) {
	rows, err := db.Query("SELECT date, type FROM calendar WHERE date >= ? AND date <= ?", start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := make(map[string]string)
	for rows.Next() {
		var d, t string
		rows.Scan(&d, &t)
		m[d] = t
	}
	return m, nil
}

// loadWeekendCalendar 仅在内存中生成周末日历（无需 DB）
func loadWeekendCalendar() map[int]bool {
	// weekday: Mon=1 ... Sun=7 → Go time.Weekday: Sun=0, Mon=1, ..., Sat=6
	return map[int]bool{
		0: false, // Sun → 非工作日
		6: false, // Sat → 非工作日
	}
}

// IsWorkDay 判断某天是否为工作日
func IsWorkDay(cal map[string]string, date string) bool {
	// 检查自定义日历
	if t, ok := cal[date]; ok {
		return t == "workday"
	}
	// 默认规则：周一~周五 = 工作日
	return !isWeekend(date)
}

// isWeekend 判断是否为周六日
func isWeekend(date string) bool {
	var y, m, d int
	fmt.Sscanf(date, "%d-%d-%d", &y, &m, &d)
	j := julianDay(y, m, d)
	// 2000-01-01 = Saturday (Julian day of week)
	// We use a reference: 2000-01-03 = Monday
	refMon := julianDayStr("2000-01-03")
	diff := j - refMon
	wd := ((diff % 7) + 7) % 7 // 0=Mon, 1=Tue, ..., 5=Sat, 6=Sun
	return wd >= 5
}

func julianDayStr(date string) int {
	var y, m, d int
	fmt.Sscanf(date, "%d-%d-%d", &y, &m, &d)
	return julianDay(y, m, d)
}

// AddWorkDays 从 date 起加 N 个工作日，返回第 N 个工作日的日期
// 例：date=周一, workDays=5 → 返回周五（5 个工作日含周一）
func AddWorkDays(cal map[string]string, date string, workDays int) string {
	if date == "" || workDays <= 0 {
		return date
	}
	if workDays == 1 {
		return date // 1 个工作日 = 当天
	}
	var y, m, d int
	fmt.Sscanf(date, "%d-%d-%d", &y, &m, &d)

	// 需要再找 workDays-1 个工作日
	for remaining := workDays - 1; remaining > 0; {
		d++
		dim := daysInMonth(y, m)
		if d > dim {
			d = 1
			m++
			if m > 12 {
				m = 1
				y++
			}
		}
		cur := fmt.Sprintf("%04d-%02d-%02d", y, m, d)
		if IsWorkDay(cal, cur) {
			remaining--
		}
	}
	return fmt.Sprintf("%04d-%02d-%02d", y, m, d)
}

// CountWorkDays 统计日期范围内的工作日数（含 start，含 end）
func CountWorkDays(cal map[string]string, start, end string) int {
	if start == "" || end == "" || start > end {
		return 0
	}
	var sy, sm, sd, ey, em, ed int
	fmt.Sscanf(start, "%d-%d-%d", &sy, &sm, &sd)
	fmt.Sscanf(end, "%d-%d-%d", &ey, &em, &ed)

	count := 0
	for {
		cur := fmt.Sprintf("%04d-%02d-%02d", sy, sm, sd)
		if IsWorkDay(cal, cur) {
			count++
		}
		if sy == ey && sm == em && sd == ed {
			break
		}
		sd++
		dim := daysInMonth(sy, sm)
		if sd > dim {
			sd = 1
			sm++
			if sm > 12 {
				sm = 1
				sy++
			}
		}
	}
	return count
}
