package util

import (
	"fmt"
	"time"
)

// FiscalYearRange 根据财年数字和起始月份返回日期范围
// startMonth=1 表示自然年，fiscalYear 即为日历年（如 FY27 = 2027-01-01 ~ 2027-12-31）
// startMonth>1 表示跨年财年（如 startMonth=4, FY27 = 2026-04-01 ~ 2027-03-31）
func FiscalYearRange(fiscalYear int, startMonth int) (string, string, error) {
	if startMonth < 1 || startMonth > 12 {
		return "", "", fmt.Errorf("财年起始月份无效: %d", startMonth)
	}

	var start, end string

	if startMonth == 1 {
		// 自然年：FY27 = 2027-01-01 ~ 2027-12-31
		calendarYear := 2000 + fiscalYear
		start = fmt.Sprintf("%04d-01-01", calendarYear)
		end = fmt.Sprintf("%04d-12-31", calendarYear)
	} else {
		// 跨年财年：FY27 = 2026-04-01 ~ 2027-03-31
		startCalendarYear := 2000 + fiscalYear - 1
		endCalendarYear := startCalendarYear + 1
		endMonth := startMonth - 1
		endDay := lastDayOfMonth(endCalendarYear, time.Month(endMonth))

		start = fmt.Sprintf("%04d-%02d-01", startCalendarYear, startMonth)
		end = fmt.Sprintf("%04d-%02d-%02d", endCalendarYear, endMonth, endDay)
	}

	return start, end, nil
}

// FiscalYearFromDate 根据日期和财年起始月计算该日期所属财年编号
// startMonth=1: FY 编号 = 日历年（2026-07-15 → 26）
// startMonth=4: 2026-07-15 → 27 (FY27)
func FiscalYearFromDate(date time.Time, startMonth int) int {
	year := date.Year()
	if startMonth == 1 {
		// 自然年：FY 编号 = 日历年
		return year - 2000
	}
	if int(date.Month()) >= startMonth {
		return year - 2000 + 1
	}
	return year - 2000
}

// CurrentFiscalYear 返回当前日期所属的财年编号
func CurrentFiscalYear(startMonth int) int {
	return FiscalYearFromDate(time.Now(), startMonth)
}

// AvailableFiscalYears 返回可选的财年编号列表（当前财年 ±2）
func AvailableFiscalYears(startMonth int) []int {
	current := CurrentFiscalYear(startMonth)
	years := make([]int, 0, 5)
	for i := current - 2; i <= current+2; i++ {
		years = append(years, i)
	}
	return years
}

// FiscalYearLabel 返回财年的显示标签
// 例如：27 → "FY27"
func FiscalYearLabel(fiscalYear int) string {
	return fmt.Sprintf("FY%d", fiscalYear)
}

// FiscalYearDisplayRange 返回财年的展示日期范围文字
// 例如：startMonth=4, fiscalYear=27 → "2026-04 ~ 2027-03"
func FiscalYearDisplayRange(fiscalYear int, startMonth int) string {
	start, end, err := FiscalYearRange(fiscalYear, startMonth)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%s ~ %s", start[:7], end[:7])
}

// CalendarYearRange 返回自然年日期范围
func CalendarYearRange(year int) (string, string) {
	start := fmt.Sprintf("%04d-01-01", year)
	end := fmt.Sprintf("%04d-12-31", year)
	return start, end
}

// lastDayOfMonth 返回指定年月最后一天
func lastDayOfMonth(year int, month time.Month) int {
	switch month {
	case time.February:
		if isLeapYear(year) {
			return 29
		}
		return 28
	case time.April, time.June, time.September, time.November:
		return 30
	default:
		return 31
	}
}

func isLeapYear(year int) bool {
	return year%4 == 0 && (year%100 != 0 || year%400 == 0)
}
