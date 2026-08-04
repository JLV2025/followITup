# 项目排程方向（正推/倒推）+ duration 驱动 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 项目创建时选择排程方向（正推/倒推），日期由引擎按方向唯一计算（排除节假日），用户只能通过修改 duration 间接影响时间，任一任务有进度后方向锁定。

**Architecture:** 后端 `projects.schedule_direction` 列决定重排方向——forward 走现有 forwardPass（不动），backward 新增倒推 pass（所有链尾对齐项目完成日期，沿前驱链倒推写回日期）。前端：创建表单选方向、任务弹窗与列表视图日期改为只读、项目页头部显示/修改方向。甘特图任务条拖拽已禁用（drag_move/resize/progress=false），无需改动。

**Tech Stack:** Go 1.22+（chi/SQLite modernc）/ React 18 TypeScript（dhtmlx-gantt）/ Vite

## Global Constraints

- 不引入新依赖（后端、前端均不新增 npm/go 包）
- 所有注释、提交信息使用简体中文；专业术语保留英文
- 改符号前 `gitnexus_impact({target, direction:"upstream"})`，HIGH/CRITICAL 风险先告知用户
- 提交前 `gitnexus_detect_changes()` 验证影响范围
- 后端每步跑 `go test ./...`；前端每步跑 `npx tsc --noEmit`
- 每个逻辑变更单独提交（中文提交信息）
- 设计依据：`docs/superpowers/specs/2026-08-04-schedule-direction-design.md`（用户确认 8 条决策）

---

### Task 1: 迁移 v5 + Project 模型加 schedule_direction 字段

**Files:**
- Modify: `backend/internal/db/sqlite.go`（migrations 数组末尾加 v5）
- Modify: `backend/internal/models/models.go`（Project 结构）
- Test: `backend/internal/db/sqlite_test.go`

**Interfaces:**
- Consumes: 无
- Produces: `migrations` 数组第 5 项（{version:5, sql}）；`models.Project.ScheduleDirection string`（json `schedule_direction`）

- [ ] **Step 1: 写失败测试**

在 `sqlite_test.go` 末尾追加（仿 `TestMigrationV4BaselineColumns`，用 PRAGMA 检查 projects 表）：

```go
// 迁移 v5 后 projects 表应包含排程方向列
func TestMigrationV5ScheduleDirection(t *testing.T) {
	d, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()

	cols := map[string]bool{}
	rows, err := d.Conn.Query(`PRAGMA table_info(projects)`)
	if err != nil {
		t.Fatalf("PRAGMA projects: %v", err)
	}
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt *string
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err == nil {
			cols[name] = true
		}
	}
	rows.Close()
	if !cols["schedule_direction"] {
		t.Error("projects 表缺少 schedule_direction 列")
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd backend && go test ./internal/db/ -run TestMigrationV5ScheduleDirection -v`
Expected: FAIL（`projects 表缺少 schedule_direction 列`）

- [ ] **Step 3: 实现迁移与模型**

`sqlite.go` migrations 数组（`{4, ...}` 之后）追加：

```go
	{5, `
	-- 项目排程方向（v5）：forward=正推(基于开始日期) backward=倒推(基于完成日期)
	ALTER TABLE projects ADD COLUMN schedule_direction TEXT NOT NULL DEFAULT 'forward';
	`},
```

`models.go` Project 结构加字段（放在 `EndDate` 之后）：

```go
	ScheduleDirection string `json:"schedule_direction"` // forward=正推 backward=倒推
```

- [ ] **Step 4: 运行确认通过**

Run: `cd backend && go test ./internal/db/ -run TestMigrationV5ScheduleDirection -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add backend/internal/db/sqlite.go backend/internal/db/sqlite_test.go backend/internal/models/models.go
git commit -m "迁移v5:projects加schedule_direction列(正推/倒推) + Project模型字段"
```

---

### Task 2: SubWorkDays 工作日倒推对偶

**Files:**
- Modify: `backend/internal/scheduler/calendar.go`
- Test: `backend/internal/scheduler/scheduler_test.go`

**Interfaces:**
- Consumes: `IsWorkDay(cal, date)`、`julianDayStr`（现有）
- Produces: `SubWorkDays(cal map[string]string, date string, workDays int) string` — 从 date 起**往前**数 workDays 个工作日（含 date 当天），返回第 workDays 个工作日的日期；`workDays <= 0` 或 `date == ""` 时原样返回。对偶性质：`AddWorkDays(cal, S, N) == E` ⟺ `SubWorkDays(cal, E, N) == S`

- [ ] **Step 1: 写失败测试**

`scheduler_test.go` 末尾追加：

```go
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
```

- [ ] **Step 2: 运行确认失败**

Run: `cd backend && go test ./internal/scheduler/ -run TestSubWorkDays -v`
Expected: FAIL（`SubWorkDays undefined`）

- [ ] **Step 3: 实现 SubWorkDays**

`calendar.go` 在 `AddWorkDays` 之后追加（镜像 AddWorkDays 逻辑，方向相反）：

```go
// SubWorkDays 从 date 起往前数 N 个工作日，返回第 N 个工作日的日期（含 date 当天）
// 与 AddWorkDays 互为逆运算：AddWorkDays(cal, S, N) == E ⟺ SubWorkDays(cal, E, N) == S
func SubWorkDays(cal map[string]string, date string, workDays int) string {
	if date == "" || workDays <= 0 {
		return date
	}
	if workDays == 1 {
		return date // 1 个工作日 = 当天
	}
	var y, m, d int
	fmt.Sscanf(date, "%d-%d-%d", &y, &m, &d)

	// 需要再往前找 workDays-1 个工作日
	for remaining := workDays - 1; remaining > 0; {
		d--
		if d < 1 {
			m--
			if m < 1 {
				m = 12
				y--
			}
			d = daysInMonth(y, m)
		}
		cur := fmt.Sprintf("%04d-%02d-%02d", y, m, d)
		if IsWorkDay(cal, cur) {
			remaining--
		}
	}
	return fmt.Sprintf("%04d-%02d-%02d", y, m, d)
}
```

- [ ] **Step 4: 运行确认通过**

Run: `cd backend && go test ./internal/scheduler/ -run TestSubWorkDays -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add backend/internal/scheduler/calendar.go backend/internal/scheduler/scheduler_test.go
git commit -m "排程:SubWorkDays工作日倒推对偶(与AddWorkDays互为逆运算)"
```

---

### Task 3: 倒推引擎（backwardScheduleWrite + 方向路由）

**Files:**
- Modify: `backend/internal/scheduler/scheduler.go`
- Test: `backend/internal/scheduler/scheduler_test.go`

**Interfaces:**
- Consumes: `SubWorkDays`（Task 2）、现有 `loadTasks/loadDeps/buildImplicitPred/rollupParentDates/shiftDate/AddWorkDays/IsWorkDay`、`ConstraintStartNoEarlierThan` 常量
- Produces:
  - `backwardScheduleWrite(tasks []TaskInfo, deps []Dep, finishDate string, cal map[string]string, parentSet map[int64]bool) map[int64]map[string]string` — 单轮倒推写回（包内）
  - `getProjectDirection(db *sql.DB, projectID int64) (string, string)` — 返回 `schedule_direction` 与 `end_date`（缺省 forward/空串）
  - `Recalculate` / `RecalculateAll` 增加方向路由：backward 时改调 `backwardSchedule(db, projectID)`

**倒推算法（backwardScheduleWrite）：**
1. 索引：`predDeps[后继id] = []Dep`（显式前驱）；`hasSucc`（显式 + 隐式后继者标记）
2. 队列初始化 = 所有**非父任务、非 manual、无后继**的任务（链尾），`EndDate = finishDate` 写入 changes
3. BFS 出队 `succ`：`succStart = SubWorkDays(cal, succ.EndDate, succ.DurationDays)`（duration 固定不写回）；对 succ 的每个前驱（显式 predDeps + 隐式 implicitPred 的 FS lag=0）计算 `pred` 的 end 候选：
   - FS: `candEnd = shiftDate(succStart, -lag)`
   - FF: `candEnd = shiftDate(succ.EndDate, -lag)`
   - SS: `candEnd = AddWorkDays(cal, shiftDate(succStart, -lag), pred.DurationDays)`
   - SF: `candEnd = AddWorkDays(cal, shiftDate(succ.EndDate, -lag), pred.DurationDays)`
4. `pred` 取**最严格**：`EndDate = min(现有值, candEnd)`（首次设置则直接赋值）；`StartDate = SubWorkDays(cal, pred.EndDate, pred.DurationDays)`
5. `pred` 是父任务或 ManualScheduled：**不写 changes**（日期由 rollup/手动锁定管），但仍入队（用其当前日期继续推它的前驱）
6. `queued` 防重入队；单轮结束返回 changes

- [ ] **Step 1: 写失败测试**

`scheduler_test.go` 末尾追加（cal 为空 map = 默认周末规则）：

```go
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
```

- [ ] **Step 2: 运行确认失败**

Run: `cd backend && go test ./internal/scheduler/ -run "TestBackwardSchedule" -v`
Expected: FAIL（`backwardScheduleWrite undefined`）

- [ ] **Step 3: 实现倒推引擎与方向路由**

`scheduler.go` 追加：

```go
// getProjectDirection 读取项目排程方向与完成日期（缺省 forward / 空串）
func getProjectDirection(db *sql.DB, projectID int64) (string, string) {
	var dir, endDate string
	err := db.QueryRow(
		`SELECT COALESCE(schedule_direction,'forward'), COALESCE(end_date,'') FROM projects WHERE id=?`,
		projectID).Scan(&dir, &endDate)
	if err != nil {
		return "forward", ""
	}
	return dir, endDate
}

// Recalculate 从 triggerTaskID 出发传播后继链（触发任务自身不被改写，保留手动调整）。
// 倒推项目改为全量倒推（duration 变更 → start 重算 → 沿前驱链传播）
func Recalculate(db *sql.DB, projectID int64, triggerTaskID int64) (map[int64]map[string]string, error) {
	if dir, _ := getProjectDirection(db, projectID); dir == "backward" {
		return backwardSchedule(db, projectID)
	}
	return recalc(db, projectID, []int64{triggerTaskID})
}

// RecalculateAll 全项目重算；倒推项目从链尾倒推，正推项目保持现有逻辑
func RecalculateAll(db *sql.DB, projectID int64) (map[int64]map[string]string, error) {
	if dir, _ := getProjectDirection(db, projectID); dir == "backward" {
		return backwardSchedule(db, projectID)
	}
	return recalcAllForward(db, projectID)
}

// recalcAllForward 原 RecalculateAll 逻辑（重命名，函数体不变：链头队列 + recalc）
func recalcAllForward(db *sql.DB, projectID int64) (map[int64]map[string]string, error) {
	tasks, err := loadTasks(db, projectID)
	if err != nil {
		return nil, fmt.Errorf("加载任务失败: %w", err)
	}
	deps, err := loadDeps(db, projectID)
	if err != nil {
		return nil, fmt.Errorf("加载依赖失败: %w", err)
	}
	implicitPred, _ := buildImplicitPred(tasks)
	predDeps := make(map[int64]bool)
	for _, d := range deps {
		predDeps[d.SuccessorID] = true
	}
	var heads []int64
	for _, t := range tasks {
		if t.ManualScheduled {
			continue // 手动任务不入队（其后继仍可通过其它链头到达）
		}
		if !predDeps[t.ID] {
			if _, ok := implicitPred[t.ID]; !ok {
				heads = append(heads, t.ID)
			}
		}
	}
	if len(heads) == 0 {
		return nil, nil
	}
	return recalc(db, projectID, heads)
}

// backwardSchedule 倒推全量重算：迭代 5 轮（rollup 参与下一轮）后落库
func backwardSchedule(db *sql.DB, projectID int64) (map[int64]map[string]string, error) {
	dir, finishDate := getProjectDirection(db, projectID)
	if dir != "backward" || finishDate == "" {
		return nil, nil // 倒推项目必须有完成日期，否则不重排
	}
	tasks, err := loadTasks(db, projectID)
	if err != nil {
		return nil, fmt.Errorf("加载任务失败: %w", err)
	}
	deps, err := loadDeps(db, projectID)
	if err != nil {
		return nil, fmt.Errorf("加载依赖失败: %w", err)
	}
	if cycle := detectCycle(tasks, deps); len(cycle) > 0 {
		return nil, fmt.Errorf("CIRCULAR_DEPENDENCY: 存在循环依赖 %v", cycle)
	}
	cal, err := LoadCalendar(db, "", "")
	if err != nil {
		cal = make(map[string]string)
	}
	parentSet := make(map[int64]bool)
	for i := range tasks {
		if tasks[i].ParentID != nil {
			parentSet[*tasks[i].ParentID] = true
		}
	}
	changes := make(map[int64]map[string]string)
	for round := 0; round < 5; round++ {
		ch := backwardScheduleWrite(tasks, deps, finishDate, cal, parentSet)
		rollupParentDates(tasks, parentSet, ch, cal)
		for id, fields := range ch {
			changes[id] = fields
		}
		if len(ch) == 0 {
			break // 收敛
		}
	}
	if len(changes) == 0 {
		return nil, nil
	}
	for id, fields := range changes {
		if _, err := db.Exec(
			"UPDATE tasks SET start_date=?, end_date=?, duration_days=?, updated_at=datetime('now') WHERE id=?",
			fields["start_date"], fields["end_date"], fields["duration_days"], id,
		); err != nil {
			log.Printf("[Scheduler] 更新任务 %d 失败: %v", id, err)
		}
	}
	return changes, nil
}

// backwardScheduleWrite 单轮倒推：所有链尾 end=finishDate，沿前驱链（显式+隐式）往前推。
// 每个非锚定任务以「本次计算的候选表 newEnd」取最严格（min），与旧日期无关（全量重排）；
// 父任务与 manual 任务不写回、不重算，但其当前日期参与链条传播
func backwardScheduleWrite(tasks []TaskInfo, deps []Dep, finishDate string, cal map[string]string, parentSet map[int64]bool) map[int64]map[string]string {
	taskMap := make(map[int64]*TaskInfo)
	for i := range tasks {
		taskMap[tasks[i].ID] = &tasks[i]
	}
	predDeps := make(map[int64][]Dep) // 后继 id → 显式前驱依赖
	hasSucc := make(map[int64]bool)   // 有显式或隐式后继者
	for _, d := range deps {
		predDeps[d.SuccessorID] = append(predDeps[d.SuccessorID], d)
		hasSucc[d.PredecessorID] = true
	}
	implicitPred, implicitSucc := buildImplicitPred(tasks)
	for predID := range implicitSucc {
		hasSucc[predID] = true
	}

	changes := make(map[int64]map[string]string)
	newEnd := make(map[int64]string) // 本次倒推计算的 end 候选（多后继取 min，与旧值无关）
	queue := []int64{}
	queued := make(map[int64]bool)

	// 队列：所有链尾（无后继、非父任务、非 manual），end = 项目完成日期
	for _, t := range tasks {
		if hasSucc[t.ID] || parentSet[t.ID] || t.ManualScheduled {
			continue
		}
		newEnd[t.ID] = finishDate
		changes[t.ID] = map[string]string{
			"start_date":    SubWorkDays(cal, finishDate, t.DurationDays),
			"end_date":      finishDate,
			"duration_days": fmt.Sprintf("%d", t.DurationDays),
		}
		t.EndDate = finishDate
		t.StartDate = changes[t.ID]["start_date"]
		queue = append(queue, t.ID)
		queued[t.ID] = true
	}

	for len(queue) > 0 {
		succID := queue[0]
		queue = queue[1:]
		succ := taskMap[succID]
		if succ == nil {
			continue
		}
		succStart := SubWorkDays(cal, succ.EndDate, succ.DurationDays)
		preds := predDeps[succID]
		if pid, ok := implicitPred[succID]; ok {
			preds = append(preds, Dep{PredecessorID: pid, SuccessorID: succID, Type: FS, LagDays: 0})
		}
		for _, dep := range preds {
			pred := taskMap[dep.PredecessorID]
			if pred == nil {
				continue
			}
			var candEnd string
			switch dep.Type {
			case FS:
				candEnd = shiftDate(succStart, -dep.LagDays)
			case FF:
				candEnd = shiftDate(succ.EndDate, -dep.LagDays)
			case SS:
				candEnd = AddWorkDays(cal, shiftDate(succStart, -dep.LagDays), pred.DurationDays)
			case SF:
				candEnd = AddWorkDays(cal, shiftDate(succ.EndDate, -dep.LagDays), pred.DurationDays)
			default:
				continue
			}
			// 多后继取最严格（更早）；父任务/manual 不重算，用当前日期继续传播
			if _, set := newEnd[pred.ID]; !set || candEnd < newEnd[pred.ID] {
				newEnd[pred.ID] = candEnd
				if !parentSet[pred.ID] && !pred.ManualScheduled {
					pred.EndDate = candEnd
					pred.StartDate = SubWorkDays(cal, candEnd, pred.DurationDays)
					changes[pred.ID] = map[string]string{
						"start_date":    pred.StartDate,
						"end_date":      candEnd,
						"duration_days": fmt.Sprintf("%d", pred.DurationDays),
					}
				}
			}
			if !queued[pred.ID] {
				queue = append(queue, pred.ID)
				queued[pred.ID] = true
			}
		}
	}
	return changes
}
```

注意：原 `RecalculateAll` 函数体整体改名 `recalcAllForward`（函数体逐行不变），`RecalculateAll` 变为上面的薄路由。

- [ ] **Step 4: 运行确认通过**

Run: `cd backend && go test ./internal/scheduler/ -v`
Expected: 全部 PASS（含既有 forward 测试，确认改名未破坏行为）

- [ ] **Step 5: 提交**

```bash
git add backend/internal/scheduler/scheduler.go backend/internal/scheduler/scheduler_test.go
git commit -m "排程引擎:倒推pass(链尾对齐完成日期,沿前驱链倒推写回,多后继取最严格)+Recalculate/All方向路由"
```

---

### Task 4: 后端项目 API 支持排程方向 + duration 下限校验

**Files:**
- Modify: `backend/internal/api/projects.go`（CreateProject/UpdateProject/GetProject/ProjectList）
- Modify: `backend/internal/api/tasks.go`（CreateTask/UpdateTask duration 校验）

**Interfaces:**
- Consumes: `models.Project.ScheduleDirection`（Task 1）、`getProjectDirection` 模式
- Produces: 请求体 `schedule_direction`（"forward"|"backward"）；`PUT /api/projects/{id}` 在方向变更且存在 `progress_pct > 0` 任务时返回 400 `DIRECTION_LOCKED`

- [ ] **Step 1: 修改 CreateProject**

`projects.go` 的 CreateProject（请求体解码为 `var p models.Project`），INSERT 前补方向默认值，INSERT 加列：

```go
	if p.ScheduleDirection == "" {
		p.ScheduleDirection = "forward" // 默认正推
	}
	result, err := h.db.Exec(
		`INSERT INTO projects (name, description, start_date, end_date, status, schedule_direction)
		 VALUES (?, ?, ?, ?, 'active', ?)`,
		p.Name, p.Description, p.StartDate, p.EndDate, p.ScheduleDirection,
	)
```

- [ ] **Step 2: 修改 UpdateProject：加方向列 + 锁定校验**

`projects.go` 的 UpdateProject（`var p models.Project` 解码后），先查当前方向与进度计数，锁定校验通过后 UPDATE 加列：

```go
	// 排程方向锁定：项目内任一任务有进度后不可修改方向
	var curDirection string
	h.db.QueryRow(`SELECT COALESCE(schedule_direction, 'forward') FROM projects WHERE id=? AND deleted_at IS NULL`, id).Scan(&curDirection)
	if p.ScheduleDirection != "" && p.ScheduleDirection != curDirection {
		var cnt int
		h.db.QueryRow(`SELECT COUNT(*) FROM tasks WHERE project_id=? AND deleted_at IS NULL AND progress_pct > 0`, id).Scan(&cnt)
		if cnt > 0 {
			writeError(w, http.StatusBadRequest, "DIRECTION_LOCKED", "项目已有任务进度，排程方向不可修改")
			return
		}
	}
	if p.ScheduleDirection == "" {
		p.ScheduleDirection = curDirection // 请求体未携带时保留旧值
	}
	_, err := h.db.Exec(
		`UPDATE projects SET name=?, description=?, start_date=?, end_date=?, status=?, is_public=?,
		       schedule_direction=?, updated_at=datetime('now')
		 WHERE id=? AND deleted_at IS NULL`,
		p.Name, p.Description, p.StartDate, p.EndDate, p.Status, boolToInt(p.IsPublic),
		p.ScheduleDirection, id,
	)
```

- [ ] **Step 3: 修改 GetProject / ProjectList 的 SELECT 加列**

`projects.go` GetProject 的查询与 Scan 追加列：

```go
	err := h.db.QueryRow(
		"SELECT id, name, description, start_date, end_date, status, is_public, schedule_direction FROM projects WHERE id = ? AND deleted_at IS NULL",
		id,
	).Scan(&p.ID, &p.Name, &p.Description, &p.StartDate, &p.EndDate, &p.Status, &isPublic, &p.ScheduleDirection)
```

ProjectList 的 query 列尾追加 `p.schedule_direction`，rows 扫描参数尾追加 `&p.ScheduleDirection`（`ProjectSummary` 内嵌 `models.Project`，字段自动带出）。

- [ ] **Step 4: tasks.go 加 duration 下限校验**

`tasks.go` CreateTask 与 UpdateTask 中，请求体解码后、写库前各加：

```go
	if t.DurationDays < 1 && t.TaskType != "milestone" {
		writeError(w, http.StatusBadRequest, "INVALID_DURATION", "工期至少 1 天")
		return
	}
```

- [ ] **Step 5: 编译 + 全量测试**

Run: `cd backend && go build ./... && go test ./...`
Expected: 编译通过、全部测试 PASS

- [ ] **Step 6: 提交**

```bash
git add backend/internal/api/projects.go backend/internal/api/tasks.go
git commit -m "后端:项目API支持排程方向(创建/更新+进度锁定校验400)+duration下限校验(里程碑除外)"
```

---

### Task 5: 前端创建表单选方向 + 项目页方向显示与修改

**Files:**
- Modify: `frontend/src/pages/Dashboard.tsx`
- Modify: `frontend/src/pages/ProjectDetail.tsx`

**Interfaces:**
- Consumes: 后端 `schedule_direction` 字段（Task 4）
- Produces: 创建项目 payload 含 `schedule_direction`；ProjectDetail 头部方向徽标与修改下拉（调用现有 `PUT /api/projects/{id}`）

- [ ] **Step 1: Dashboard 创建表单加方向选择**

`Dashboard.tsx`：

```tsx
  const [createForm, setCreateForm] = useState({
    name: "",
    start_date: "",
    end_date: "",
    schedule_direction: "forward",
    description: "",
  });
```

创建 modal 中「日期」form-row 之前插入方向选择，并按方向条件渲染日期字段（替换现有 start/end 两个 form-group）：

```tsx
              <div className="form-group">
                <label htmlFor="project-direction">排程方向</label>
                <select
                  id="project-direction"
                  value={createForm.schedule_direction}
                  onChange={(e) =>
                    setCreateForm({ ...createForm, schedule_direction: e.target.value })
                  }
                >
                  <option value="forward">正排（从开始日期向后排）</option>
                  <option value="backward">倒排（从完成日期向前排）</option>
                </select>
              </div>
              <div className="form-row">
                {createForm.schedule_direction === "forward" ? (
                  <div className="form-group">
                    <label htmlFor="project-start">开始日期</label>
                    <input
                      id="project-start"
                      type="date"
                      value={createForm.start_date}
                      onChange={(e) => setCreateForm({ ...createForm, start_date: e.target.value })}
                    />
                  </div>
                ) : (
                  <div className="form-group">
                    <label htmlFor="project-end">完成日期</label>
                    <input
                      id="project-end"
                      type="date"
                      value={createForm.end_date}
                      onChange={(e) => setCreateForm({ ...createForm, end_date: e.target.value })}
                    />
                  </div>
                )}
              </div>
```

创建提交的 payload 加 `schedule_direction: createForm.schedule_direction`（现有 `api.post("/api/projects", ...)` 处）。

- [ ] **Step 2: ProjectDetail 头部显示方向 + 修改**

`ProjectDetail.tsx`：Project 接口加 `schedule_direction: string`；头部描述下方加方向徽标与修改下拉：

```tsx
      {/* 排程方向 */}
      <div className="project-direction-row">
        {project.schedule_direction === "backward" ? (
          <span className="badge badge-blue">倒排（基于完成日期）</span>
        ) : (
          <span className="badge">正排（基于开始日期）</span>
        )}
        <select
          className="direction-select"
          value={project.schedule_direction}
          onChange={async (e) => {
            const dir = e.target.value;
            try {
              await api.put(`/api/projects/${id}`, { ...project, schedule_direction: dir });
              setProject({ ...project, schedule_direction: dir });
            } catch (err: any) {
              alert(err?.response?.data?.message || "排程方向修改失败");
              setProject({ ...project }); // 回弹原值
            }
          }}
        >
          <option value="forward">正排</option>
          <option value="backward">倒排</option>
        </select>
      </div>
```

- [ ] **Step 3: 类型检查**

Run: `cd frontend && npx tsc --noEmit`
Expected: 无类型错误

- [ ] **Step 4: 提交**

```bash
git add frontend/src/pages/Dashboard.tsx frontend/src/pages/ProjectDetail.tsx
git commit -m "前端:创建项目选排程方向(正推填开始/倒推填完成) + 项目页方向徽标与修改(锁定提示)"
```

---

### Task 6: 前端任务日期只读（duration 驱动）

**Files:**
- Modify: `frontend/src/components/TaskDetailModal.tsx`
- Modify: `frontend/src/pages/TaskListView.tsx`

**Interfaces:**
- Consumes: 无新接口
- Produces: 任务弹窗开始/结束日期 disabled（所有任务，含非父任务）；列表视图日期单元格改纯文本

- [ ] **Step 1: 任务弹窗日期改为只读**

`TaskDetailModal.tsx` 日期 form-row（约 395-407 行）：

```tsx
        <div className="form-row">
          <div className="form-group">
            <label>开始日期（由排程自动计算）</label>
            <input type="date" value={startDate} disabled />
          </div>
          <div className="form-group">
            <label>结束日期（由排程自动计算）</label>
            <input type="date" value={endDate} disabled />
          </div>
        </div>
```

（原 `disabled={isParent}` 与 onChange 移除；`startDate/endDate` state、payload 发送保持不动——保存仍携带日期，防止后端字段被清空。duration 输入（`min={1}`）保留可编辑。）

- [ ] **Step 2: 列表视图日期单元格改只读**

`TaskListView.tsx` 日期单元格（当前结构为 `<td onClick={() => startEdit(t, "start_date")} className="cell-editable">{formatDate(t.start_date)}</td>`，结束日期同理），去掉 onClick 与 cell-editable 类，改纯文本：

```tsx
                <td>{formatDate(t.start_date)}</td>
                <td>{formatDate(t.end_date)}</td>
```

（`startEdit` 中 `case "start_date"`/`case "end_date"` 分支保留不删——已无入口触发；duration 单元格的编辑入口保留。）

- [ ] **Step 3: 类型检查**

Run: `cd frontend && npx tsc --noEmit`
Expected: 无类型错误

- [ ] **Step 4: 提交**

```bash
git add frontend/src/components/TaskDetailModal.tsx frontend/src/pages/TaskListView.tsx
git commit -m "前端:任务日期只读(日期=排程输出,duration驱动)+列表视图日期改纯文本"
```

---

### Task 7: 全量验证与回归

**Files:** 无新增

- [ ] **Step 1: 后端全量测试 + 前端类型检查**

Run: `cd backend && go test ./... && cd ../frontend && npx tsc --noEmit`
Expected: 全部 PASS、无类型错误

- [ ] **Step 2: 影响范围检查**

Run: `gitnexus_detect_changes`（scope: all）
Expected: 变更符号集中在 scheduler/projects/tasks/Dashboard/ProjectDetail/TaskDetailModal/TaskListView；无 HIGH/CRITICAL 意外影响。如有异常先与用户确认。

- [ ] **Step 3: 构建 + 浏览器验证**

```bash
cd frontend && npm run build
cd backend && go build -o followitup.exe ./cmd/server/
```

浏览器验证清单（项目 6）：
- 创建倒推项目（填完成日期）→ 建任务链 → 日期从完成日期往前铺开，无任务超过完成日期
- 改某任务 duration → 该任务 start 与前置链自动调整，甘特图自动刷新（无需刷新页面）
- 任务弹窗：开始/结束日期置灰不可编辑；duration 可编辑
- 列表视图：日期单元格只读，duration 可编辑
- 任一任务填进度 → ProjectDetail 方向下拉修改被拒（400 提示）
- 倒排项目存在两条独立分支 → 链尾均对齐完成日期
- 正推项目行为与改动前一致（回归）

- [ ] **Step 4: 提交**

```bash
git add -A
git commit -m "验证:排程方向功能浏览器回归通过"
```

（若验证发现问题，按 bug 流程先修再提交，并记录 .wolf/buglog.json）
