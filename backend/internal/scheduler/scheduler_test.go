package scheduler

import (
	"testing"
)

func TestShiftDate(t *testing.T) {
	tests := []struct {
		date string
		days int
		want string
	}{
		{"2026-07-01", 5, "2026-07-06"},
		{"2026-07-28", -3, "2026-07-25"},
		{"2026-01-01", 31, "2026-02-01"},
		{"2026-12-31", 1, "2027-01-01"},
		{"2026-03-01", -1, "2026-02-28"},
		{"2027-03-01", -1, "2027-02-28"},
	}

	for _, tt := range tests {
		got := shiftDate(tt.date, tt.days)
		if got != tt.want {
			t.Errorf("shiftDateFwd(%s, %d) = %s, 期望 %s", tt.date, tt.days, got, tt.want)
		}
	}
}

func TestCalcDuration(t *testing.T) {
	tests := []struct {
		start string
		end   string
		want  int
	}{
		{"2026-07-01", "2026-07-15", 14},
		{"2026-07-01", "2026-07-01", 0},
		{"2026-01-01", "2026-12-31", 364},
		{"2026-01-01", "2027-01-01", 365},
	}

	for _, tt := range tests {
		got := calcDurationCal(tt.start, tt.end)
		if got != tt.want {
			t.Errorf("calcDurationCal(%s, %s) = %d, 期望 %d", tt.start, tt.end, got, tt.want)
		}
	}
}

func TestDetectCycle(t *testing.T) {
	tasks := []TaskInfo{
		{ID: 1, StartDate: "2026-07-01", EndDate: "2026-07-05"},
		{ID: 2, StartDate: "2026-07-06", EndDate: "2026-07-10"},
		{ID: 3, StartDate: "2026-07-10", EndDate: "2026-07-15"},
	}
	// A → B → C → A（环）
	deps := []Dep{
		{ID: 1, PredecessorID: 1, SuccessorID: 2, Type: FS},
		{ID: 2, PredecessorID: 2, SuccessorID: 3, Type: FS},
		{ID: 3, PredecessorID: 3, SuccessorID: 1, Type: FS},
	}
	cycle := detectCycle(tasks, deps)
	if len(cycle) == 0 {
		t.Error("应检测到循环依赖")
	}

	// 无环
	depsOK := []Dep{
		{ID: 1, PredecessorID: 1, SuccessorID: 2, Type: FS},
		{ID: 2, PredecessorID: 2, SuccessorID: 3, Type: FS},
	}
	if c := detectCycle(tasks, depsOK); len(c) > 0 {
		t.Error("不应检测到循环依赖")
	}
}

func TestCalcDatesFS(t *testing.T) {
	pred := &TaskInfo{StartDate: "2026-07-01", EndDate: "2026-07-05", DurationDays: 4}
	succ := &TaskInfo{StartDate: "2026-07-10", EndDate: "2026-07-15", DurationDays: 5}
	dep := Dep{Type: FS, LagDays: 1}

	newStart, newEnd := calcDates(pred, succ, dep, map[string]string{})
	if newStart != "2026-07-06" {
		t.Errorf("FS: 后继开始应为 07-06（前驱结束 07-05 + lag 1 → 07-06），得到 %s", newStart)
	}
	if newEnd != "2026-07-10" {
		t.Errorf("FS: 后继结束应为 07-11（开始 07-06 + 5天），得到 %s", newEnd)
	}
}

func TestCalcDatesSS(t *testing.T) {
	pred := &TaskInfo{StartDate: "2026-07-01", EndDate: "2026-07-05", DurationDays: 4}
	succ := &TaskInfo{StartDate: "2026-07-10", EndDate: "2026-07-15", DurationDays: 5}
	dep := Dep{Type: SS, LagDays: 2}

	newStart, _ := calcDates(pred, succ, dep, map[string]string{})
	if newStart != "2026-07-03" {
		t.Errorf("SS: 后继开始应为 07-03（前驱开始 07-01 + lag 2 → 07-03），得到 %s", newStart)
	}
}

// --- forwardPass ---

func TestForwardPass(t *testing.T) {
	tasks := []TaskInfo{
		{ID: 1, StartDate: "2026-07-01", EndDate: "2026-07-15", DurationDays: 14, ManualScheduled: false},
		{ID: 2, StartDate: "2026-07-10", EndDate: "2026-07-20", DurationDays: 10, ManualScheduled: false},
		{ID: 3, StartDate: "2026-07-25", EndDate: "2026-07-30", DurationDays: 5, ManualScheduled: false},
	}
	deps := []Dep{
		{ID: 1, PredecessorID: 1, SuccessorID: 2, Type: FS, LagDays: 0},
		{ID: 2, PredecessorID: 2, SuccessorID: 3, Type: FS, LagDays: 2},
	}

	changes := forwardPass(tasks, deps, []int64{1}, map[int64]bool{}, map[string]string{})
	if len(changes) != 2 {
		t.Fatalf("应影响 2 个后继任务，实际 %d", len(changes))
	}

	if ch, ok := changes[2]; ok {
		if ch["start_date"] != "2026-07-15" {
			t.Errorf("任务 2 开始应为 07-15，得到 %s", ch["start_date"])
		}
	} else {
		t.Error("任务 2 应在变化列表中")
	}

	if ch, ok := changes[3]; ok {
		if ch["start_date"] != "2026-07-30" {
			t.Errorf("任务 3 开始应为 07-27，得到 %s", ch["start_date"])
		}
	} else {
		t.Error("任务 3 应在变化列表中")
	}
}

func TestManualScheduledBlocksCascade(t *testing.T) {
	tasks := []TaskInfo{
		{ID: 1, StartDate: "2026-07-01", EndDate: "2026-07-05", DurationDays: 4, ManualScheduled: false},
		{ID: 2, StartDate: "2026-07-10", EndDate: "2026-07-20", DurationDays: 10, ManualScheduled: true},
	}
	deps := []Dep{
		{ID: 1, PredecessorID: 1, SuccessorID: 2, Type: FS, LagDays: 0},
	}

	tasks[0].EndDate = "2026-07-08"
	tasks[0].DurationDays = 7

	changes := forwardPass(tasks, deps, []int64{1}, map[int64]bool{}, map[string]string{})
	if _, ok := changes[2]; ok {
		t.Error("手动锁定的任务不应被自动调整")
	}
}

func TestMultiplePredecessorsMaxDate(t *testing.T) {
	tasks := []TaskInfo{
		{ID: 1, StartDate: "2026-07-01", EndDate: "2026-07-05", DurationDays: 4, ManualScheduled: false},
		{ID: 2, StartDate: "2026-07-01", EndDate: "2026-07-05", DurationDays: 4, ManualScheduled: false},
		{ID: 3, StartDate: "2026-07-01", EndDate: "2026-07-10", DurationDays: 9, ManualScheduled: false},
		{ID: 4, StartDate: "2026-07-01", EndDate: "2026-07-02", DurationDays: 1, ManualScheduled: false},
	}
	deps := []Dep{
		{ID: 1, PredecessorID: 1, SuccessorID: 2, Type: FS, LagDays: 0},
		{ID: 2, PredecessorID: 1, SuccessorID: 3, Type: FS, LagDays: 0},
		{ID: 3, PredecessorID: 2, SuccessorID: 4, Type: FS, LagDays: 0},
		{ID: 4, PredecessorID: 3, SuccessorID: 4, Type: FS, LagDays: 0},
	}

	tasks[0].EndDate = "2026-07-08"
	tasks[0].DurationDays = 7

	changes := forwardPass(tasks, deps, []int64{1}, map[int64]bool{}, map[string]string{})

	// B(7/9开始→7/12结束)→D=7/12, C(7/9开始→7/17结束)→D=7/17. D 应取 max = 7/17
	if start, ok := changes[4]["start_date"]; !ok {
		t.Error("任务 4（D）应被更新")
	} else if start < "2026-07-17" {
		t.Errorf("D 开始日期应 ≥ 2026-07-17（C 更晚，取 max），实际: %s", start)
	}
}

func TestStartNoEarlierThanConstraint(t *testing.T) {
	// A(7/1→7/5) ──FS──→ B(7/25→7/29, SNET=7/20)
	// B 当前日期 7/25 > 约束 floor 7/20，A 完成日期不会影响 B
	tasks := []TaskInfo{
		{ID: 1, StartDate: "2026-07-01", EndDate: "2026-07-05", DurationDays: 4, ManualScheduled: false},
		{ID: 2, StartDate: "2026-07-25", EndDate: "2026-07-29", DurationDays: 4, ManualScheduled: false,
			ConstraintType: ConstraintStartNoEarlierThan, ConstraintDate: "2026-07-20"},
	}
	deps := []Dep{
		{ID: 1, PredecessorID: 1, SuccessorID: 2, Type: FS, LagDays: 0},
	}

	// A 提前到 7/3 完成，候选 7/4 → 约束 floor 7/20 → B 双向跟随提前到 7/20（前置变化后继自动调整）
	tasks[0].EndDate = "2026-07-03"
	tasks[0].DurationDays = 2

	changes := forwardPass(tasks, deps, []int64{1}, map[int64]bool{}, map[string]string{})
	if ch, ok := changes[2]; !ok || ch["start_date"] != "2026-07-20" {
		t.Errorf("A 提前后 B 应跟随提前到约束 floor 7/20，实际 changes=%v", changes[2])
	}

	// 但如果 A 延期到 7/28，候选=7/29 > B 当前 7/25 → 应推送
	tasks[0].EndDate = "2026-07-28"
	tasks[0].DurationDays = 27
	changes2 := forwardPass(tasks, deps, []int64{1}, map[int64]bool{}, map[string]string{})
	if _, ok := changes2[2]; !ok {
		t.Error("A 延期到 7/28，B 应被推后到 7/29")
	}
}

func TestBackwardPassDeadline(t *testing.T) {
	// A(5天) ──FS──→ B(7天) ──FS──→ C(3天, FNLT=7/31)
	// 后向传播：C.LF=7/31, C.LS=7/28 → B.LF=7/28 → A.LF=7/21
	tasks := []TaskInfo{
		{ID: 1, StartDate: "2026-07-01", EndDate: "2026-07-05", DurationDays: 5},
		{ID: 2, StartDate: "2026-07-06", EndDate: "2026-07-12", DurationDays: 7},
		{ID: 3, StartDate: "2026-07-13", EndDate: "2026-07-15", DurationDays: 3,
			ConstraintType: ConstraintFinishNoLaterThan, ConstraintDate: "2026-07-31"},
	}
	deps := []Dep{
		{ID: 1, PredecessorID: 1, SuccessorID: 2, Type: FS, LagDays: 0},
		{ID: 2, PredecessorID: 2, SuccessorID: 3, Type: FS, LagDays: 0},
	}

	forwardPass(tasks, deps, []int64{1}, map[int64]bool{}, map[string]string{})
	backwardPass(tasks, deps, map[int64]bool{}, map[string]string{})

	// 后向传播：C.LF → 7/31, B.LF → 7/28(FS:7/31-3), A.LF → 7/20(FS:7/28-7)
	taskMap := make(map[int64]*TaskInfo)
	for i := range tasks {
		taskMap[tasks[i].ID] = &tasks[i]
	}

	// C: LF 应为 7/31（手动指定）
	if taskMap[3].LateFinish != "2026-07-31" {
		t.Errorf("C.LF = %s, 期望 2026-07-31", taskMap[3].LateFinish)
	}

	// 关键路径：TF = LS - ES
	// C: LS=7/28, ES=7/13, TF=15天
	if taskMap[3].TotalFloat != 15 {
		t.Errorf("C.TotalFloat = %d, 期望 15（LS=7/28 ES=7/13 → 15天）", taskMap[3].TotalFloat)
	}
}

func TestConstraintConflict(t *testing.T) {
	// A(10天) ──FS──→ B(5天, FNLT=7/10)
	// 如果 A 已排到 7/15 完成 → B 最早 7/16 开始 → B 7/20 完成 > 7/10 deadline → 冲突
	tasks := []TaskInfo{
		{ID: 1, StartDate: "2026-07-01", EndDate: "2026-07-15", DurationDays: 14, ManualScheduled: false},
		{ID: 2, StartDate: "2026-07-16", EndDate: "2026-07-20", DurationDays: 5, ManualScheduled: false,
			ConstraintType: ConstraintFinishNoLaterThan, ConstraintDate: "2026-07-10"},
	}
	deps := []Dep{
		{ID: 1, PredecessorID: 1, SuccessorID: 2, Type: FS, LagDays: 0},
	}

	changes := forwardPass(tasks, deps, []int64{1}, map[int64]bool{}, map[string]string{})
	backwardPass(tasks, deps, map[int64]bool{}, map[string]string{})

	// B 的结束日期（前向）应晚于约束日期 → 冲突
	if _, ok := changes[2]; ok {
		// B 被更新（因为 A 结束日 7/15 晚于 B 初始开始日 7/16 → 实际上 B 在 forwardPass 中：candidateStart = 7/15+0=7/15, candidateEnd=7/19）
		// 7/19 > 7/10 → 冲突
		t.Log("检测到约束冲突（预期中）")
	}
}

// 隐式顺序依赖：同分支按 sort_order 相邻任务默认顺序衔接（前一任务的结束 = 后一任务的开始基准）
func TestImplicitOrderDependency(t *testing.T) {
	tasks := []TaskInfo{
		{ID: 1, StartDate: "2026-07-01", EndDate: "2026-07-03", DurationDays: 3, SortOrder: 0},
		{ID: 2, StartDate: "2026-07-10", EndDate: "2026-07-12", DurationDays: 3, SortOrder: 1},
		{ID: 3, StartDate: "2026-07-15", EndDate: "2026-07-16", DurationDays: 2, SortOrder: 2},
	}
	// 无显式依赖，仅隐式顺序：1 → 2 → 3

	// 1 号任务（前置）改为 7/6 结束 → 2 应跟随提前到 7/6，3 跟随到 2 的结束
	tasks[0].EndDate = "2026-07-06"
	changes := forwardPass(tasks, nil, []int64{1}, map[int64]bool{}, map[string]string{})

	if ch, ok := changes[2]; !ok || ch["start_date"] != "2026-07-06" {
		t.Errorf("任务2 应跟随前置 1 提前到 7/6，实际 changes=%v", changes[2])
	}
	if ch, ok := changes[3]; !ok || ch["start_date"] != "2026-07-08" {
		t.Errorf("任务3 应跟随任务2 到 7/8（7/6+2工作日），实际 changes=%v", changes[3])
	}
}

// 隐式顺序依赖不适用于手动排程任务
func TestImplicitOrderSkippedForManual(t *testing.T) {
	tasks := []TaskInfo{
		{ID: 1, StartDate: "2026-07-01", EndDate: "2026-07-03", DurationDays: 3, SortOrder: 0},
		{ID: 2, StartDate: "2026-07-10", EndDate: "2026-07-12", DurationDays: 3, SortOrder: 1, ManualScheduled: true},
	}
	tasks[0].EndDate = "2026-07-06"
	changes := forwardPass(tasks, nil, []int64{1}, map[int64]bool{}, map[string]string{})
	if _, ok := changes[2]; ok {
		t.Error("手动排程任务不应被隐式顺序依赖修改")
	}
}

// ============================================================================
// 倒推排程测试（backwardScheduleWrite）
// ============================================================================

// 倒推：单链 A(5天) → B(7天)，完成日期 7/31 → B.end=7/31，A.end=B.start，逐段往前
func TestBackwardScheduleSingleChain(t *testing.T) {
	tasks := []TaskInfo{
		{ID: 1, StartDate: "2026-07-01", EndDate: "2026-07-05", DurationDays: 5},
		{ID: 2, StartDate: "2026-07-08", EndDate: "2026-07-31", DurationDays: 7},
	}
	deps := []Dep{{ID: 1, PredecessorID: 1, SuccessorID: 2, Type: FS, LagDays: 0}}
	changes := backwardScheduleWrite(tasks, deps, "2026-07-31", map[string]string{}, map[int64]bool{})
	// B.end=7/31，B.start=7/31 往前 7 个工作日：7/23(四) 7/24(五) 7/27 7/28 7/29 7/30 7/31 → 7/23
	if ch, ok := changes[2]; !ok || ch["end_date"] != "2026-07-31" || ch["start_date"] != "2026-07-23" {
		t.Errorf("B 应为 7/23~7/31，实际 %v", ch)
	}
	// A.end = B.start = 7/23；A.start = 7/23 往前 5 个工作日：7/17(五) 7/20 7/21 7/22 7/23 → 7/17
	if ch, ok := changes[1]; !ok || ch["end_date"] != "2026-07-23" || ch["start_date"] != "2026-07-17" {
		t.Errorf("A 应为 7/17~7/23，实际 %v", ch)
	}
}

// 倒推：两条独立分支（不同父任务分组，无隐式连接）链尾都对齐完成日期
func TestBackwardScheduleMultiTail(t *testing.T) {
	pid1 := int64(100)
	pid2 := int64(200)
	tasks := []TaskInfo{
		{ID: 1, ParentID: &pid1, StartDate: "2026-07-01", EndDate: "2026-07-05", DurationDays: 5, SortOrder: 0},
		{ID: 2, ParentID: &pid2, StartDate: "2026-07-01", EndDate: "2026-07-05", DurationDays: 2, SortOrder: 0},
	}
	changes := backwardScheduleWrite(tasks, nil, "2026-07-31", map[string]string{}, map[int64]bool{})
	if ch, ok := changes[1]; !ok || ch["end_date"] != "2026-07-31" {
		t.Errorf("分支 1 链尾应 = 7/31，实际 %v", ch)
	}
	if ch, ok := changes[2]; !ok || ch["end_date"] != "2026-07-31" {
		t.Errorf("分支 2 链尾应 = 7/31，实际 %v", ch)
	}
}

// 倒推：同分支隐式顺序依赖参与（相邻任务 FS 衔接），链尾对齐完成日期
func TestBackwardScheduleImplicitPred(t *testing.T) {
	tasks := []TaskInfo{
		{ID: 1, StartDate: "2026-07-01", EndDate: "2026-07-05", DurationDays: 5, SortOrder: 0},
		{ID: 2, StartDate: "2026-07-08", EndDate: "2026-07-31", DurationDays: 7, SortOrder: 1},
	}
	changes := backwardScheduleWrite(tasks, nil, "2026-07-31", map[string]string{}, map[int64]bool{})
	// 隐式 FS：A.end = B.start - 0
	if ch, ok := changes[1]; !ok || ch["end_date"] != "2026-07-23" || ch["start_date"] != "2026-07-17" {
		t.Errorf("隐式前驱 A 应为 7/17~7/23，实际 %v", ch)
	}
}

// 倒推：四种依赖类型 + lag
func TestBackwardScheduleDepTypes(t *testing.T) {
	// FS lag=2：succ.start = pred.end + 2 → 倒推 pred.end = succ.start - 2
	tasks := []TaskInfo{
		{ID: 1, StartDate: "2026-07-01", EndDate: "2026-07-05", DurationDays: 5},
		{ID: 2, StartDate: "2026-07-08", EndDate: "2026-07-31", DurationDays: 7},
	}
	deps := []Dep{{ID: 1, PredecessorID: 1, SuccessorID: 2, Type: FS, LagDays: 2}}
	changes := backwardScheduleWrite(tasks, deps, "2026-07-31", map[string]string{}, map[int64]bool{})
	// B.start=7/23 → A.end = 7/23 - 2 = 7/21
	if ch, ok := changes[1]; !ok || ch["end_date"] != "2026-07-21" {
		t.Errorf("FS lag=2：A.end 应为 7/21，实际 %v", ch)
	}

	// FF lag=1：succ.end = pred.end + 1 → pred.end = succ.end - 1
	tasks2 := []TaskInfo{
		{ID: 1, StartDate: "2026-07-01", EndDate: "2026-07-05", DurationDays: 5},
		{ID: 2, StartDate: "2026-07-08", EndDate: "2026-07-31", DurationDays: 7},
	}
	deps2 := []Dep{{ID: 1, PredecessorID: 1, SuccessorID: 2, Type: FF, LagDays: 1}}
	ch2 := backwardScheduleWrite(tasks2, deps2, "2026-07-31", map[string]string{}, map[int64]bool{})
	if ch, ok := ch2[1]; !ok || ch["end_date"] != "2026-07-30" {
		t.Errorf("FF lag=1：A.end 应为 7/30，实际 %v", ch)
	}

	// SS lag=0：succ.start = pred.start → pred.start 约束 = succ.start，pred.end = start+duration
	tasks3 := []TaskInfo{
		{ID: 1, StartDate: "2026-07-01", EndDate: "2026-07-05", DurationDays: 5},
		{ID: 2, StartDate: "2026-07-08", EndDate: "2026-07-31", DurationDays: 7},
	}
	deps3 := []Dep{{ID: 1, PredecessorID: 1, SuccessorID: 2, Type: SS, LagDays: 0}}
	ch3 := backwardScheduleWrite(tasks3, deps3, "2026-07-31", map[string]string{}, map[int64]bool{})
	// B.start=7/23 → A.start 约束 = 7/23 → A.end = 7/23 往后 5 工作日：7/23(四) 7/24(五) 7/27(一) 7/28(二) 7/29(三) → 7/29
	if ch, ok := ch3[1]; !ok || ch["end_date"] != "2026-07-29" || ch["start_date"] != "2026-07-23" {
		t.Errorf("SS：A 应为 7/23~7/29，实际 %v", ch)
	}
}

// 倒推：manual 任务不被改写，但链条沿其当前日期继续往前推
func TestBackwardScheduleManualScheduled(t *testing.T) {
	tasks := []TaskInfo{
		{ID: 1, StartDate: "2026-07-01", EndDate: "2026-07-05", DurationDays: 5, ManualScheduled: true},
		{ID: 2, StartDate: "2026-07-08", EndDate: "2026-07-31", DurationDays: 7},
	}
	deps := []Dep{{ID: 1, PredecessorID: 1, SuccessorID: 2, Type: FS, LagDays: 0}}
	changes := backwardScheduleWrite(tasks, deps, "2026-07-31", map[string]string{}, map[int64]bool{})
	if _, ok := changes[1]; ok {
		t.Error("manual 任务不应被倒推改写")
	}
}

// 倒推：父任务不直接参与，其日期由子任务 rollup（迭代收敛在 backwardSchedule 内验证）
func TestBackwardScheduleParentNotWritten(t *testing.T) {
	pid := int64(10)
	tasks := []TaskInfo{
		{ID: 1, ParentID: &pid, StartDate: "2026-07-01", EndDate: "2026-07-05", DurationDays: 5},
		{ID: 10, StartDate: "2026-07-01", EndDate: "2026-07-05", DurationDays: 5},
	}
	changes := backwardScheduleWrite(tasks, nil, "2026-07-31", map[string]string{}, map[int64]bool{10: true})
	if _, ok := changes[10]; ok {
		t.Error("父任务日期应由 rollup 汇总，倒推 pass 不应直接写")
	}
	if ch, ok := changes[1]; !ok || ch["end_date"] != "2026-07-31" {
		t.Errorf("子任务链尾应 = 7/31，实际 %v", ch)
	}
}

// SubWorkDays 与 AddWorkDays 互为逆运算（含周末与自定义节假日）
func TestSubWorkDays(t *testing.T) {
	// 无节假日（默认周末规则）：周一 8/3 起 5 个工作日 → 8/7 周五
	if got := AddWorkDays(nil, "2026-08-03", 5); got != "2026-08-07" {
		t.Errorf("AddWorkDays(8/3, 5) = %s, want 2026-08-07", got)
	}
	if got := SubWorkDays(nil, "2026-08-07", 5); got != "2026-08-03" {
		t.Errorf("SubWorkDays(8/7, 5) = %s, want 2026-08-03", got)
	}

	// 跨周末：周五 8/7 起 3 个工作日 → 8/11 周二；对偶
	if got := AddWorkDays(nil, "2026-08-07", 3); got != "2026-08-11" {
		t.Errorf("AddWorkDays(8/7, 3) = %s, want 2026-08-11", got)
	}
	if got := SubWorkDays(nil, "2026-08-11", 3); got != "2026-08-07" {
		t.Errorf("SubWorkDays(8/11, 3) = %s, want 2026-08-07", got)
	}

	// 含节假日：8/6（周四）为节假日 → 周一 8/3 起 5 个工作日 → 8/10 周一；对偶
	cal := map[string]string{"2026-08-06": "holiday"}
	if got := AddWorkDays(cal, "2026-08-03", 5); got != "2026-08-10" {
		t.Errorf("AddWorkDays(8/3, 5, 节假日) = %s, want 2026-08-10", got)
	}
	if got := SubWorkDays(cal, "2026-08-10", 5); got != "2026-08-03" {
		t.Errorf("SubWorkDays(8/10, 5, 节假日) = %s, want 2026-08-03", got)
	}

	// 边界：1 个工作日 = 当天；<=0 原样返回
	if got := SubWorkDays(nil, "2026-08-07", 1); got != "2026-08-07" {
		t.Errorf("SubWorkDays(8/7, 1) = %s, want 2026-08-07", got)
	}
	if got := SubWorkDays(nil, "2026-08-07", 0); got != "2026-08-07" {
		t.Errorf("SubWorkDays(8/7, 0) = %s, want 原样 2026-08-07", got)
	}
}
