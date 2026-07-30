package util

import (
	"testing"
	"time"
)

func TestFiscalYearRange(t *testing.T) {
	tests := []struct {
		fy        int
		startMon  int
		wantStart string
		wantEnd   string
		wantErr   bool
	}{
		// 财年模式（startMonth=4）
		{27, 4, "2026-04-01", "2027-03-31", false},
		{28, 4, "2027-04-01", "2028-03-31", false},
		{26, 4, "2025-04-01", "2026-03-31", false},
		// 财年模式（startMonth=7）
		{27, 7, "2026-07-01", "2027-06-30", false},
		// 财年模式（startMonth=10）
		{27, 10, "2026-10-01", "2027-09-30", false},
		// 自然年（startMonth=1）
		{27, 1, "2027-01-01", "2027-12-31", false},
		{26, 1, "2026-01-01", "2026-12-31", false},
		// 无效月份
		{27, 0, "", "", true},
		{27, 13, "", "", true},
	}

	for _, tt := range tests {
		start, end, err := FiscalYearRange(tt.fy, tt.startMon)
		if tt.wantErr && err == nil {
			t.Errorf("FiscalYearRange(%d, %d) 期望出错但没有", tt.fy, tt.startMon)
		}
		if !tt.wantErr {
			if start != tt.wantStart {
				t.Errorf("FiscalYearRange(%d, %d) start = %s, 期望 %s", tt.fy, tt.startMon, start, tt.wantStart)
			}
			if end != tt.wantEnd {
				t.Errorf("FiscalYearRange(%d, %d) end = %s, 期望 %s", tt.fy, tt.startMon, end, tt.wantEnd)
			}
		}
	}
}

func TestFiscalYearFromDate(t *testing.T) {
	tests := []struct {
		date     string
		startMon int
		want     int
	}{
		// 财年模式（startMonth=4）
		{"2026-07-15", 4, 27}, // 7月 >= 4月, FY27
		{"2026-03-15", 4, 26}, // 3月 < 4月, FY26
		{"2026-04-01", 4, 27}, // 刚好在起始月, FY27
		{"2026-03-31", 4, 26}, // 起始月前一天, FY26
		// 自然年模式（startMonth=1）
		{"2026-07-15", 1, 26}, // FY = 日历年
		{"2026-01-01", 1, 26},
		{"2027-03-01", 1, 27},
	}

	for _, tt := range tests {
		date, _ := time.Parse("2006-01-02", tt.date)
		got := FiscalYearFromDate(date, tt.startMon)
		if got != tt.want {
			t.Errorf("FiscalYearFromDate(%s, %d) = %d, 期望 %d", tt.date, tt.startMon, got, tt.want)
		}
	}
}

func TestAvailableFiscalYears(t *testing.T) {
	years := AvailableFiscalYears(4)
	if len(years) != 5 {
		t.Errorf("AvailableFiscalYears(4) 长度 = %d, 期望 5", len(years))
	}
	for _, y := range years {
		if y == 0 {
			t.Errorf("AvailableFiscalYears 返回了 0 值")
		}
	}
}

func TestFiscalYearLabel(t *testing.T) {
	if label := FiscalYearLabel(27); label != "FY27" {
		t.Errorf("FiscalYearLabel(27) = %s, 期望 FY27", label)
	}
}

func TestCalendarYearRange(t *testing.T) {
	s, e := CalendarYearRange(2026)
	if s != "2026-01-01" || e != "2026-12-31" {
		t.Errorf("CalendarYearRange(2026) = (%s, %s)", s, e)
	}
}

func TestFiscalYearDisplayRange(t *testing.T) {
	r := FiscalYearDisplayRange(27, 4)
	if r != "2026-04 ~ 2027-03" {
		t.Errorf("FiscalYearDisplayRange(27, 4) = %s, 期望 2026-04 ~ 2027-03", r)
	}
}
