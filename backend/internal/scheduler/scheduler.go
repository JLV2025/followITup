package scheduler

import (
	"database/sql"
	"fmt"
	"log"
	"sort"
	"time"
)

type DepType string

const (
	FS DepType = "FS"
	SS DepType = "SS"
	FF DepType = "FF"
	SF DepType = "SF"
)

const (
	ConstraintNone              = ""
	ConstraintStartNoEarlierThan = "start_no_earlier_than"
	ConstraintFinishNoLaterThan  = "finish_no_later_than"
)

type Dep struct {
	ID            int64
	PredecessorID int64
	SuccessorID   int64
	Type          DepType
	LagDays       int
}

type TaskInfo struct {
	ID              int64
	ParentID        *int64
	StartDate       string
	EndDate         string
	DurationDays    int
	ManualScheduled bool
	ConstraintType  string
	ConstraintDate  string
	LateStart       string
	LateFinish      string
	TotalFloat      int
	SortOrder       int // 项目内排序序号（隐式顺序依赖依据）
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

func recalc(db *sql.DB, projectID int64, startQueue []int64) (map[int64]map[string]string, error) {
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

	// 迭代收敛：父任务作为前驱时，其汇总值（rollup）参与下一轮传播。
	// 每轮从起点重新前向传播（内存值已更新），直至无变化（一般 2-3 轮，上限 5 轮）
	changes := make(map[int64]map[string]string)
	for round := 0; round < 5; round++ {
		ch := forwardPass(tasks, deps, startQueue, parentSet, cal)
		rollupParentDates(tasks, parentSet, ch, cal)
		for id, fields := range ch {
			changes[id] = fields
		}
		if len(ch) == 0 {
			break // 收敛：无新变化
		}
	}
	backwardPass(tasks, deps, parentSet, cal)

	for i := range tasks {
		t := &tasks[i]
		if t.ConstraintType == ConstraintFinishNoLaterThan && t.EndDate > t.ConstraintDate {
			if changes[t.ID] == nil {
				changes[t.ID] = make(map[string]string)
			}
			changes[t.ID]["constraint_conflict"] = "1"
		}
	}
	if len(changes) == 0 {
		return nil, nil
	}
	for id, fields := range changes {
		// 写冲突（SQLITE_BUSY，启动期/并发）时重试一次
		for attempt := 0; attempt < 2; attempt++ {
			_, err := db.Exec(
				"UPDATE tasks SET start_date=?, end_date=?, duration_days=?, updated_at=datetime('now') WHERE id=?",
				fields["start_date"], fields["end_date"], fields["duration_days"], id,
			)
			if err == nil {
				break
			}
			if attempt == 1 {
				log.Printf("[Scheduler] 更新任务 %d 失败: %v", id, err)
			} else {
				time.Sleep(200 * time.Millisecond)
			}
		}
	}
	return changes, nil
}

func loadTasks(db *sql.DB, projectID int64) ([]TaskInfo, error) {
	rows, err := db.Query(
		`SELECT id, start_date, end_date, duration_days, manual_scheduled,
		        constraint_type, constraint_date, parent_id, sort_order
		 FROM tasks WHERE project_id = ? AND deleted_at IS NULL`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []TaskInfo
	for rows.Next() {
		var t TaskInfo
		var manual int
		if err := rows.Scan(&t.ID, &t.StartDate, &t.EndDate, &t.DurationDays, &manual,
			&t.ConstraintType, &t.ConstraintDate, &t.ParentID, &t.SortOrder); err != nil {
			continue
		}
		t.ManualScheduled = manual != 0
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func loadDeps(db *sql.DB, projectID int64) ([]Dep, error) {
	rows, err := db.Query(
		`SELECT d.id, d.predecessor_id, d.successor_id, d.dep_type, d.lag_days
		 FROM dependencies d JOIN tasks t ON t.id = d.predecessor_id
		 WHERE t.project_id = ?`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var deps []Dep
	for rows.Next() {
		var d Dep
		if err := rows.Scan(&d.ID, &d.PredecessorID, &d.SuccessorID, &d.Type, &d.LagDays); err == nil {
			deps = append(deps, d)
		}
	}
	return deps, nil
}

func detectCycle(tasks []TaskInfo, deps []Dep) []int64 {
	taskSet := make(map[int64]bool)
	for _, t := range tasks {
		taskSet[t.ID] = true
	}
	adj := make(map[int64][]int64)
	for _, d := range deps {
		adj[d.PredecessorID] = append(adj[d.PredecessorID], d.SuccessorID)
	}
	visited := make(map[int64]bool)
	recStack := make(map[int64]bool)
	var path []int64
	var dfs func(id int64) bool
	dfs = func(id int64) bool {
		visited[id] = true
		recStack[id] = true
		path = append(path, id)
		for _, succ := range adj[id] {
			if recStack[succ] {
				path = append(path, succ)
				return true
			}
			if !visited[succ] && dfs(succ) {
				return true
			}
		}
		recStack[id] = false
		path = path[:len(path)-1]
		return false
	}
	for _, t := range tasks {
		if !visited[t.ID] {
			path = nil
			if dfs(t.ID) {
				return path
			}
		}
	}
	return nil
}

func forwardPass(tasks []TaskInfo, deps []Dep, startQueue []int64, parentSet map[int64]bool, cal map[string]string) map[int64]map[string]string {
	taskMap := make(map[int64]*TaskInfo)
	for i := range tasks {
		taskMap[tasks[i].ID] = &tasks[i]
	}
	successors := make(map[int64][]Dep)  // 前置 id → 显式依赖（正向传播）
	predDeps := make(map[int64][]Dep)    // 后继 id → 显式依赖（候选综合取 max）
	for _, d := range deps {
		successors[d.PredecessorID] = append(successors[d.PredecessorID], d)
		predDeps[d.SuccessorID] = append(predDeps[d.SuccessorID], d)
	}

	// 隐式顺序依赖：同分支（同 parent）内按 sort_order 相邻的任务，前者是后者的隐式 FS 前置
	// 即"任务的开始时间默认 = 前面任务的结束时间"，除非显式依赖/手动排程覆盖
	implicitPred, implicitSucc := buildImplicitPred(tasks)

	changes := make(map[int64]map[string]string)
	queue := startQueue
	visited := make(map[int64]bool)

	// applyCandidate 计算候选日期并双向更新后继（前置变化 → 后继自动调整，含提前）
	// 候选 = max(当前边候选, 该后继的全部显式前置候选, 隐式前驱结束时间)——多前置取 max 始终成立
	applyCandidate := func(pred *TaskInfo, succ *TaskInfo, dep Dep) {
		if pred == nil || succ == nil || succ.ManualScheduled || parentSet[succ.ID] {
			return
		}

		candidateStart, _ := calcDates(pred, succ, dep, cal)
		for _, pd := range predDeps[succ.ID] {
			if p := taskMap[pd.PredecessorID]; p != nil {
				cs, _ := calcDates(p, succ, pd, cal)
				if cs > candidateStart {
					candidateStart = cs
				}
			}
		}
		// 隐式前驱约束：仅当任务没有显式前置时生效——定义了前置（一个或多个）后，
		// 开始时间完全由前置完成时间决定，与顺序/前一个任务无关
		// 隐式 FS 与显式 FS 同语义：end 为独占式，后继直接从前置 end 衔接
		if len(predDeps[succ.ID]) == 0 {
			if pid, ok := implicitPred[succ.ID]; ok {
				if p := taskMap[pid]; p != nil && p.EndDate != "" && p.EndDate > candidateStart {
					candidateStart = p.EndDate
				}
			}
		}
		candidateEnd := AddWorkDays(cal, candidateStart, succ.DurationDays)

		if succ.ConstraintType == ConstraintStartNoEarlierThan && succ.ConstraintDate != "" {
			if candidateStart < succ.ConstraintDate {
				candidateStart = succ.ConstraintDate
				candidateEnd = AddWorkDays(cal, candidateStart, succ.DurationDays)
			}
		}

		// 双向跟随：前置（或前面任务）日期变化时后继自动调整（含提前）。
		// start 未变化时：仅当 duration 变更导致 end 过时（end ≠ start+duration）才重算 end，
		// 否则保持不动（避免无谓更新；日期为排程唯一来源，无手动编辑场景）
		if candidateStart == succ.StartDate {
			expectedEnd := AddWorkDays(cal, candidateStart, succ.DurationDays)
			if succ.EndDate == expectedEnd {
				return
			}
			changes[succ.ID] = map[string]string{
				"start_date":    candidateStart,
				"end_date":      expectedEnd,
				"duration_days": fmt.Sprintf("%d", succ.DurationDays),
			}
			succ.EndDate = expectedEnd
			return
		}

		changes[succ.ID] = map[string]string{
			"start_date":    candidateStart,
			"end_date":      candidateEnd,
			// duration 是用户定义的固定属性：拖动/排程只调整开始/结束时间，不改变时长
			"duration_days": fmt.Sprintf("%d", succ.DurationDays),
		}
		succ.StartDate = candidateStart
		succ.EndDate = candidateEnd
		// succ.DurationDays 保持不变

		queue = append(queue, succ.ID)
	}

	for len(queue) > 0 {
		currentID := queue[0]
		queue = queue[1:]
		if visited[currentID] {
			continue
		}
		visited[currentID] = true

		// 显式依赖后继
		for _, dep := range successors[currentID] {
			applyCandidate(taskMap[dep.PredecessorID], taskMap[dep.SuccessorID], dep)
		}

		// 隐式顺序后继（同分支内按 sort_order 的下一个任务，FS lag=0）
		if nextID, ok := implicitSucc[currentID]; ok {
			applyCandidate(taskMap[currentID], taskMap[nextID], Dep{Type: FS, LagDays: 0})
		}
	}
	return changes
}

// buildImplicitPred 构建隐式顺序依赖映射：同分支（同 parent_id）内按 sort_order 排序，
// 相邻任务前者是后者的隐式前驱。返回 id→前驱 与 id→后继 两个方向
func buildImplicitPred(tasks []TaskInfo) (map[int64]int64, map[int64]int64) {
	implicitPred := make(map[int64]int64)
	implicitSucc := make(map[int64]int64)
	groups := make(map[int64][]*TaskInfo) // parentID（0=顶级）→ 该分支的任务
	for i := range tasks {
		pid := int64(0)
		if tasks[i].ParentID != nil {
			pid = *tasks[i].ParentID
		}
		groups[pid] = append(groups[pid], &tasks[i])
	}
	for _, g := range groups {
		// (SortOrder, ID) 双键排序保证确定性（SortOrder 相同时按 id，避免不稳定排序）
		sort.Slice(g, func(a, b int) bool {
			if g[a].SortOrder != g[b].SortOrder {
				return g[a].SortOrder < g[b].SortOrder
			}
			return g[a].ID < g[b].ID
		})
		for i := 1; i < len(g); i++ {
			implicitPred[g[i].ID] = g[i-1].ID
			implicitSucc[g[i-1].ID] = g[i].ID
		}
	}
	return implicitPred, implicitSucc
}

func backwardPass(tasks []TaskInfo, deps []Dep, parentSet map[int64]bool, cal map[string]string) {
	taskMap := make(map[int64]*TaskInfo)
	for i := range tasks {
		t := &tasks[i]
		taskMap[t.ID] = t
		t.LateStart = ""
		t.LateFinish = "9999-12-31"
	}

	latestFinish := ""
	for _, t := range tasks {
		if t.ConstraintType == ConstraintFinishNoLaterThan && t.ConstraintDate != "" {
			if latestFinish == "" || t.ConstraintDate > latestFinish {
				latestFinish = t.ConstraintDate
			}
		}
		if t.EndDate > latestFinish {
			latestFinish = t.EndDate
		}
	}
	if latestFinish == "" {
		return
	}

	predecessors := make(map[int64][]Dep)
	for _, d := range deps {
		predecessors[d.SuccessorID] = append(predecessors[d.SuccessorID], d)
	}
	// 隐式顺序依赖参与倒推（关键路径/浮动计算）
	implicitPred, _ := buildImplicitPred(tasks)
	for succID, predID := range implicitPred {
		predecessors[succID] = append(predecessors[succID], Dep{PredecessorID: predID, SuccessorID: succID, Type: FS, LagDays: 0})
	}
	hasSuccessor := make(map[int64]bool)
	for _, d := range deps {
		hasSuccessor[d.PredecessorID] = true
	}
	for predID := range implicitPred {
		hasSuccessor[predID] = true
	}

	type qi struct {
		id int64
		ls string
		lf string
	}
	queue := []qi{}
	queued := make(map[int64]bool)

	for _, t := range tasks {
		if !hasSuccessor[t.ID] {
			queue = append(queue, qi{id: t.ID, lf: latestFinish})
			queued[t.ID] = true
		}
	}
	for _, t := range tasks {
		if t.ConstraintType == ConstraintFinishNoLaterThan && t.ConstraintDate != "" && !queued[t.ID] {
			queue = append(queue, qi{id: t.ID, lf: t.ConstraintDate})
			queued[t.ID] = true
		}
	}

	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]

		succ := taskMap[item.id]
		if succ == nil || parentSet[succ.ID] {
			continue
		}

		ls := shiftDate(item.lf, -succ.DurationDays)

		if succ.LateFinish == "" || item.lf < succ.LateFinish {
			succ.LateFinish = item.lf
			succ.LateStart = ls
		}

		if succ.ConstraintType == ConstraintFinishNoLaterThan && succ.ConstraintDate != "" {
			if succ.LateFinish == "" || succ.ConstraintDate < succ.LateFinish {
				succ.LateFinish = succ.ConstraintDate
				succ.LateStart = shiftDate(succ.ConstraintDate, -succ.DurationDays)
			}
		}

		for _, dep := range predecessors[item.id] {
			pred := taskMap[dep.PredecessorID]
			if pred == nil || pred.ManualScheduled {
				continue
			}
			predLF := calcPredecessorLFFwd(succ, dep)
			if pred.LateFinish == "" || predLF < pred.LateFinish {
				pred.LateFinish = predLF
				pred.LateStart = shiftDate(predLF, -pred.DurationDays)
			}
			if !queued[pred.ID] {
				queue = append(queue, qi{id: pred.ID, lf: pred.LateFinish})
				queued[pred.ID] = true
			}
		}
	}

	for i := range tasks {
		t := &tasks[i]
		if t.LateStart != "" && t.StartDate != "" {
			t.TotalFloat = julianDayFromStr(t.LateStart) - julianDayFromStr(t.StartDate)
		}
	}
}

func rollupParentDates(tasks []TaskInfo, parentSet map[int64]bool, changes map[int64]map[string]string, cal map[string]string) {
	if len(parentSet) == 0 {
		return
	}
	children := make(map[int64][]*TaskInfo)
	for i := range tasks {
		t := &tasks[i]
		if t.ParentID != nil {
			children[*t.ParentID] = append(children[*t.ParentID], t)
		}
	}
	for parentID, kids := range children {
		if len(kids) == 0 {
			continue
		}
		minStart, maxEnd := "", ""
		for _, c := range kids {
			if c.StartDate != "" && (minStart == "" || c.StartDate < minStart) {
				minStart = c.StartDate
			}
			if c.EndDate != "" && (maxEnd == "" || c.EndDate > maxEnd) {
				maxEnd = c.EndDate
			}
		}
		if minStart == "" || maxEnd == "" {
			continue
		}
		duration := CountWorkDays(cal, minStart, maxEnd)
		if duration < 1 {
			duration = 1
		}
		if changes[parentID] == nil {
			changes[parentID] = make(map[string]string)
		}
		changes[parentID]["start_date"] = minStart
		changes[parentID]["end_date"] = maxEnd
		changes[parentID]["duration_days"] = fmt.Sprintf("%d", duration)
		for i := range tasks {
			if tasks[i].ID == parentID {
				tasks[i].StartDate = minStart
				tasks[i].EndDate = maxEnd
				tasks[i].DurationDays = duration
				break
			}
		}
	}
}

func calcPredecessorLFFwd(succ *TaskInfo, dep Dep) string {
	switch dep.Type {
	case FS:
		return shiftDate(succ.LateStart, -dep.LagDays-1)
	case SS:
		return shiftDate(succ.LateStart, -dep.LagDays)
	case FF:
		return shiftDate(succ.LateFinish, -dep.LagDays)
	case SF:
		return shiftDate(succ.LateFinish, -dep.LagDays)
	default:
		return succ.LateStart
	}
}

func calcDates(pred, succ *TaskInfo, dep Dep, cal map[string]string) (string, string) {
	lag := dep.LagDays
	switch dep.Type {
	case FS:
		// FS：前置"结束后"后继开始。end 为独占式（结束日 = 开始 + 工期），
		// 直接衔接：lag=0 → succStart = pred.EndDate；lag=N → 再隔 N 个工作日
		succStart := AddWorkDays(cal, pred.EndDate, lag)
		succEnd := AddWorkDays(cal, succStart, succ.DurationDays)
		return succStart, succEnd
	case SS:
		// SS：开始对齐（同日开始）——lag 个工作日
		succStart := AddWorkDays(cal, pred.StartDate, lag)
		succEnd := AddWorkDays(cal, succStart, succ.DurationDays)
		return succStart, succEnd
	case FF:
		// FF：结束对齐——后继 end = 前置 end + lag；start 从 end 倒推 duration 个工作日
		succEnd := AddWorkDays(cal, pred.EndDate, lag)
		succStart := SubWorkDays(cal, succEnd, succ.DurationDays)
		return succStart, succEnd
	case SF:
		// SF：前置开始 → 后继结束
		succEnd := AddWorkDays(cal, pred.StartDate, lag)
		succStart := SubWorkDays(cal, succEnd, succ.DurationDays)
		return succStart, succEnd
	default:
		return succ.StartDate, succ.EndDate
	}
}

func shiftDate(date string, days int) string {
	if date == "" {
		return date
	}
	var y, m, d int
	fmt.Sscanf(date, "%d-%d-%d", &y, &m, &d)
	d += days
	for {
		dim := daysInMonth(y, m)
		if d > dim {
			d -= dim
			m++
			if m > 12 {
				m = 1
				y++
			}
		} else if d < 1 {
			m--
			if m < 1 {
				m = 12
				y--
			}
			d += daysInMonth(y, m)
		} else {
			break
		}
	}
	return fmt.Sprintf("%04d-%02d-%02d", y, m, d)
}

func calcDurationCal(start, end string) int { return julianDayFromStr(end) - julianDayFromStr(start) }

func julianDayFromStr(date string) int {
	var y, m, d int
	fmt.Sscanf(date, "%d-%d-%d", &y, &m, &d)
	return julianDay(y, m, d)
}

func daysInMonth(y, m int) int {
	switch m {
	case 2:
		if (y%4 == 0 && y%100 != 0) || y%400 == 0 {
			return 29
		}
		return 28
	case 4, 6, 9, 11:
		return 30
	default:
		return 31
	}
}

func julianDay(y, m, d int) int {
	if m <= 2 {
		y--
		m += 12
	}
	a := y / 100
	b := 2 - a + a/4
	return (36525*(y+4716))/100 + (306001*(m+1))/10000 + d + b - 1524
}

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
		// 写冲突（SQLITE_BUSY，启动期/并发）时重试一次
		for attempt := 0; attempt < 2; attempt++ {
			_, err := db.Exec(
				"UPDATE tasks SET start_date=?, end_date=?, duration_days=?, updated_at=datetime('now') WHERE id=?",
				fields["start_date"], fields["end_date"], fields["duration_days"], id,
			)
			if err == nil {
				break
			}
			if attempt == 1 {
				log.Printf("[Scheduler] 更新任务 %d 失败: %v", id, err)
			} else {
				time.Sleep(200 * time.Millisecond)
			}
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
	// 注意：必须用索引取指针（for i := range），range 副本更新不会影响 taskMap 指向的原始元素
	for i := range tasks {
		t := &tasks[i]
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
		if len(preds) == 0 {
			if pid, ok := implicitPred[succID]; ok {
				preds = append(preds, Dep{PredecessorID: pid, SuccessorID: succID, Type: FS, LagDays: 0})
			}
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
