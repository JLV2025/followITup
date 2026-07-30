package scheduler

import (
	"database/sql"
	"fmt"
	"log"
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
}

func Recalculate(db *sql.DB, projectID int64, triggerTaskID int64) (map[int64]map[string]string, error) {
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

	changes := forwardPass(tasks, deps, triggerTaskID, parentSet, cal)
	rollupParentDates(tasks, parentSet, changes, cal)
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
		if _, err := db.Exec(
			"UPDATE tasks SET start_date=?, end_date=?, duration_days=?, updated_at=datetime('now') WHERE id=?",
			fields["start_date"], fields["end_date"], fields["duration_days"], id,
		); err != nil {
			log.Printf("[Scheduler] 更新任务 %d 失败: %v", id, err)
		}
	}
	return changes, nil
}

func loadTasks(db *sql.DB, projectID int64) ([]TaskInfo, error) {
	rows, err := db.Query(
		`SELECT id, start_date, end_date, duration_days, manual_scheduled,
		        constraint_type, constraint_date, parent_id
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
			&t.ConstraintType, &t.ConstraintDate, &t.ParentID); err != nil {
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

func forwardPass(tasks []TaskInfo, deps []Dep, triggerTaskID int64, parentSet map[int64]bool, cal map[string]string) map[int64]map[string]string {
	taskMap := make(map[int64]*TaskInfo)
	for i := range tasks {
		taskMap[tasks[i].ID] = &tasks[i]
	}
	successors := make(map[int64][]Dep)
	for _, d := range deps {
		successors[d.PredecessorID] = append(successors[d.PredecessorID], d)
	}

	changes := make(map[int64]map[string]string)
	queue := []int64{triggerTaskID}
	visited := make(map[int64]bool)

	for len(queue) > 0 {
		currentID := queue[0]
		queue = queue[1:]
		if visited[currentID] {
			continue
		}
		visited[currentID] = true

		for _, dep := range successors[currentID] {
			pred := taskMap[dep.PredecessorID]
			succ := taskMap[dep.SuccessorID]
			if pred == nil || succ == nil || succ.ManualScheduled || parentSet[succ.ID] {
				continue
			}

			candidateStart, candidateEnd := calcDates(pred, succ, dep, cal)

			if succ.ConstraintType == ConstraintStartNoEarlierThan && succ.ConstraintDate != "" {
				if candidateStart < succ.ConstraintDate {
					candidateStart = succ.ConstraintDate
					candidateEnd = AddWorkDays(cal, candidateStart, succ.DurationDays)
				}
			}

			if candidateStart <= succ.StartDate {
				continue
			}

			duration := CountWorkDays(cal, candidateStart, candidateEnd)
			if duration < 1 {
				duration = 1
			}
			changes[succ.ID] = map[string]string{
				"start_date":    candidateStart,
				"end_date":      candidateEnd,
				"duration_days": fmt.Sprintf("%d", duration),
			}
			succ.StartDate = candidateStart
			succ.EndDate = candidateEnd
			succ.DurationDays = duration

			queue = append(queue, succ.ID)
		}
	}
	return changes
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
	hasSuccessor := make(map[int64]bool)
	for _, d := range deps {
		hasSuccessor[d.PredecessorID] = true
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
		succStart := shiftDate(pred.EndDate, lag)
		succEnd := AddWorkDays(cal, succStart, succ.DurationDays)
		return succStart, succEnd
	case SS:
		succStart := shiftDate(pred.StartDate, lag)
		succEnd := AddWorkDays(cal, succStart, succ.DurationDays)
		return succStart, succEnd
	case FF:
		succEnd := shiftDate(pred.EndDate, lag)
		succStart := shiftDate(succEnd, -succ.DurationDays)
		return succStart, succEnd
	case SF:
		succEnd := shiftDate(pred.StartDate, lag)
		succStart := shiftDate(succEnd, -succ.DurationDays)
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
