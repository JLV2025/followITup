package api

import (
	"strings"
	"testing"
	"time"
)

// 实际日期自动填充：进行中→记实际开始；完成→记实际结束；已有值不覆盖
func TestFillActualDates(t *testing.T) {
	today := time.Now().Format("2006-01-02")

	cases := []struct {
		name         string
		status       string
		actualStart  string
		actualEnd    string
		wantStart    string
		wantEnd      string
	}{
		{"变为进行中记实际开始", "in_progress", "", "", today, ""},
		{"变为完成记实际结束", "completed", "", "", "", today},
		{"已有实际开始不覆盖", "in_progress", "2026-07-01", "", "2026-07-01", ""},
		{"已有实际结束不覆盖", "completed", "", "2026-07-20", "", "2026-07-20"},
		{"其他状态不变", "open", "", "", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotS, gotE := fillActualDates(c.status, c.actualStart, c.actualEnd)
			if gotS != c.wantStart || gotE != c.wantEnd {
				t.Errorf("fillActualDates(%q,%q,%q) = (%q,%q), want (%q,%q)",
					c.status, c.actualStart, c.actualEnd, gotS, gotE, c.wantStart, c.wantEnd)
			}
		})
	}
	_ = strings.TrimSpace // 保留占位避免未用导入
}
