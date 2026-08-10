package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"followitup/internal/auth"
	"followitup/internal/models"

	"github.com/go-chi/chi/v5"
)

func TestSplitOwnerNames(t *testing.T) {
	cases := []struct{ in string; want []string }{
		{"张三;李四", []string{"张三", "李四"}},
		{"张三; 李四", []string{"张三", "李四"}},
		{"张三,李四", []string{"张三", "李四"}},
		{"张三;张三;李四", []string{"张三", "李四"}},   // 去重
		{" 张三 ; 李四 ", []string{"张三", "李四"}},   // trim
		{"", nil},
		{"张三", []string{"张三"}},
	}
	for _, c := range cases {
		got := splitOwnerNames(c.in)
		if len(got) != len(c.want) {
			t.Errorf("splitOwnerNames(%q) = %v, want %v", c.in, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("splitOwnerNames(%q) = %v, want %v", c.in, got, c.want)
				break
			}
		}
	}
}

// resolveUserIDs + saveTaskAssignees + loadTaskAssignees 全链路
func TestResolveAndSaveAssignees(t *testing.T) {
	conn, _ := testTaskHandler(t) // 复用已有 helper:db.Open(TempDir) 已跑全部迁移(含 v9)
	conn.Exec(`INSERT INTO users (login, email, display_name, password_hash, auth_source, is_active) VALUES ('a','a@x.com','张三','x','local',1), ('b','b@x.com','李四','x','local',1), ('c','c@x.com','王五','x','local',0)`)
	pid := setupProject(t, conn)
	tid := setupTask(t, conn, pid, "任务X", "", "")

	ids, missing := resolveUserIDs(conn, []string{"张三", "王五", "不存在"})
	if len(ids) != 1 || len(missing) != 2 {
		t.Fatalf("resolveUserIDs = (%v, %v), want 1 成功 2 失败(停用+不存在)", ids, missing)
	}
	// 重复 id 写入去重
	snap := saveTaskAssignees(conn, tid, append(ids, ids[0]))
	if snap != "张三" {
		t.Errorf("快照 = %q, want 张三", snap)
	}
	gotIDs, gotSnap := loadTaskAssignees(conn, tid)
	if len(gotIDs) != 1 || gotIDs[0] != ids[0] || gotSnap != "张三" {
		t.Errorf("loadTaskAssignees = (%v, %q)", gotIDs, gotSnap)
	}
	// 覆盖写:换成李四
	snap = saveTaskAssignees(conn, tid, []int64{})
	if snap != "" {
		t.Errorf("清空后快照 = %q, want 空", snap)
	}
	gotIDs, _ = loadTaskAssignees(conn, tid)
	if len(gotIDs) != 0 {
		t.Errorf("清空后 ids = %v, want 空", gotIDs)
	}
}

// saveProjectOwners + loadProjectOwners
func TestResolveAndSaveProjectOwners(t *testing.T) {
	conn, _ := testTaskHandler(t)
	conn.Exec(`INSERT INTO users (login, email, display_name, password_hash, auth_source, is_active) VALUES ('a','a@x.com','张三','x','local',1), ('b','b@x.com','李四','x','local',1)`)
	pid := setupProject(t, conn)

	ids, _ := resolveUserIDs(conn, []string{"张三", "李四"})
	snap := saveProjectOwners(conn, pid, ids)
	if snap != "张三; 李四" {
		t.Errorf("项目快照 = %q, want %q", snap, "张三; 李四")
	}
	got, _ := loadProjectOwners(conn, pid)
	if len(got) != 2 {
		t.Errorf("owners = %v, want 2 个", got)
	}
}

// resolveUserIDs email 精确匹配优先于 display_name(反例:B.display_name 撞 email 文本且 B.id 更小)
func TestResolveUserIDsEmailPriority(t *testing.T) {
	conn, _ := testTaskHandler(t)
	// B 先插(id 更小):display_name='foo@x.com';A 后插:id 更大,email='foo@x.com'
	conn.Exec(`INSERT INTO users (login, email, display_name, password_hash, auth_source, is_active) VALUES ('b','bar@x.com','foo@x.com','x','local',1), ('a','foo@x.com','Bar','x','local',1)`)
	ids, missing := resolveUserIDs(conn, []string{"foo@x.com"})
	if len(ids) != 1 || len(missing) != 0 {
		t.Fatalf("resolveUserIDs = (%v, %v), want 1 成功 0 失败", ids, missing)
	}
	var wantID int64
	conn.QueryRow(`SELECT id FROM users WHERE email = 'foo@x.com'`).Scan(&wantID)
	if ids[0] != wantID {
		t.Errorf("email 优先解析 = %d, want %d(A),否则被 display_name 撞车的 B 抢先", ids[0], wantID)
	}
}

// ownerNamesOf 按传入 ids 顺序返回显示名,缺失跳过
func TestOwnerNamesOf(t *testing.T) {
	conn, _ := testTaskHandler(t)
	conn.Exec(`INSERT INTO users (login, email, display_name, password_hash, auth_source, is_active) VALUES ('a','a@x.com','张三','x','local',1), ('b','b@x.com','李四','x','local',1)`)
	var uid1, uid2 int64
	conn.QueryRow(`SELECT id FROM users WHERE login = 'a'`).Scan(&uid1)
	conn.QueryRow(`SELECT id FROM users WHERE login = 'b'`).Scan(&uid2)

	names := ownerNamesOf(conn, []int64{uid2, uid1})
	if len(names) != 2 || names[0] != "李四" || names[1] != "张三" {
		t.Errorf("ownerNamesOf 顺序 = %v, want [李四 张三]", names)
	}
	// 含不存在 id:跳过
	names = ownerNamesOf(conn, []int64{uid1, 99999, uid2})
	if len(names) != 2 || names[0] != "张三" || names[1] != "李四" {
		t.Errorf("ownerNamesOf 缺失跳过 = %v, want [张三 李四]", names)
	}
}

// CreateTask 传 assignee_ids 数组 → 关联表写入;assignee 列同步快照
func TestCreateTaskWithAssigneeIDs(t *testing.T) {
	conn, h := testTaskHandler(t)
	conn.Exec(`INSERT INTO users (login, email, display_name, password_hash, auth_source, is_active) VALUES ('a','a@x.com','张三','x','local',1), ('b','b@x.com','李四','x','local',1)`)
	pid := setupProject(t, conn)
	var uid1, uid2 int64
	conn.QueryRow(`SELECT id FROM users WHERE email='a@x.com'`).Scan(&uid1)
	conn.QueryRow(`SELECT id FROM users WHERE email='b@x.com'`).Scan(&uid2)

	body, _ := json.Marshal(map[string]interface{}{
		"name": "任务A", "task_type": "task", "status": "open",
		"start_date": "2026-08-03", "end_date": "2026-08-10", "duration_days": 5,
		"assignee_ids": []int64{uid1, uid2, uid1}, // 重复 id 去重
	})
	r := chi.NewRouter()
	r.Post("/api/projects/{id}/tasks", h.CreateTask)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/projects/%d/tasks", pid), bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateTask 状态码 = %d, body=%s", w.Code, w.Body.String())
	}
	var tid int64
	conn.QueryRow(`SELECT id FROM tasks WHERE project_id=? AND name='任务A'`, pid).Scan(&tid)
	var n int
	conn.QueryRow(`SELECT COUNT(*) FROM task_assignees WHERE task_id=?`, tid).Scan(&n)
	if n != 2 {
		t.Errorf("task_assignees 行数 = %d, want 2(去重)", n)
	}
	var snap string
	conn.QueryRow(`SELECT assignee FROM tasks WHERE id=?`, tid).Scan(&snap)
	if snap != "张三; 李四" {
		t.Errorf("快照列 = %q, want %q", snap, "张三; 李四")
	}
	// 响应体含 assignee_ids
	var resp struct {
		Data models.Task `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Data.AssigneeIDs) != 2 {
		t.Errorf("响应 assignee_ids = %v, want 2 个", resp.Data.AssigneeIDs)
	}
}

// UpdateTask:未携带负责人字段 → 保留 DB 旧关联;携带 assignee_ids → 覆盖写
func TestUpdateTaskPreservesOrOverwritesAssignees(t *testing.T) {
	conn, h := testTaskHandler(t)
	conn.Exec(`INSERT INTO users (login, email, display_name, password_hash, auth_source, is_active) VALUES ('a','a@x.com','张三','x','local',1), ('b','b@x.com','李四','x','local',1)`)
	pid := setupProject(t, conn)
	tid := setupTask(t, conn, pid, "任务B", "", "")
	var uid1 int64
	conn.QueryRow(`SELECT id FROM users WHERE email='a@x.com'`).Scan(&uid1)
	saveTaskAssignees(conn, tid, []int64{uid1})

	// 部分更新(不含 assignee_ids/assignee):旧关联保留
	body, _ := json.Marshal(map[string]interface{}{
		"name": "任务B改名", "task_type": "task", "status": "open",
		"start_date": "2026-08-03", "end_date": "2026-08-10", "duration_days": 5,
		"version": 1,
	})
	r := chi.NewRouter()
	r.Put("/api/projects/{id}/tasks/{taskID}", h.UpdateTask)
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/projects/%d/tasks/%d", pid, tid), bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("部分更新状态码 = %d, body=%s", w.Code, w.Body.String())
	}
	var n int
	conn.QueryRow(`SELECT COUNT(*) FROM task_assignees WHERE task_id=?`, tid).Scan(&n)
	if n != 1 {
		t.Errorf("部分更新后关联行数 = %d, want 1(保留旧值)", n)
	}
	// 携带 assignee_ids:[] → 清空
	body2, _ := json.Marshal(map[string]interface{}{
		"name": "任务B改名", "task_type": "task", "status": "open",
		"start_date": "2026-08-03", "end_date": "2026-08-10", "duration_days": 5,
		"assignee_ids": []int64{}, "version": 2,
	})
	req2 := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/projects/%d/tasks/%d", pid, tid), bytes.NewReader(body2))
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("覆盖写状态码 = %d, body=%s", w2.Code, w.Body.String())
	}
	conn.QueryRow(`SELECT COUNT(*) FROM task_assignees WHERE task_id=?`, tid).Scan(&n)
	if n != 0 {
		t.Errorf("清空后关联行数 = %d, want 0", n)
	}
}

// UpdateTask 旧文本 assignee 兼容:分号分隔
func TestUpdateTaskLegacyAssigneeText(t *testing.T) {
	conn, h := testTaskHandler(t)
	conn.Exec(`INSERT INTO users (login, email, display_name, password_hash, auth_source, is_active) VALUES ('a','a@x.com','张三','x','local',1), ('b','b@x.com','李四','x','local',1)`)
	pid := setupProject(t, conn)
	tid := setupTask(t, conn, pid, "任务C", "", "")

	body, _ := json.Marshal(map[string]interface{}{
		"name": "任务C", "task_type": "task", "status": "open",
		"start_date": "2026-08-03", "end_date": "2026-08-10", "duration_days": 5,
		"assignee": "张三;李四", "version": 1,
	})
	r := chi.NewRouter()
	r.Put("/api/projects/{id}/tasks/{taskID}", h.UpdateTask)
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/projects/%d/tasks/%d", pid, tid), bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, body=%s", w.Code, w.Body.String())
	}
	var n int
	conn.QueryRow(`SELECT COUNT(*) FROM task_assignees WHERE task_id=?`, tid).Scan(&n)
	if n != 2 {
		t.Errorf("文本解析后关联行数 = %d, want 2", n)
	}
}

// CreateTask 未指定负责人 → 默认取项目全部 owner
func TestCreateTaskDefaultsToProjectOwners(t *testing.T) {
	conn, h := testTaskHandler(t)
	conn.Exec(`INSERT INTO users (login, email, display_name, password_hash, auth_source, is_active) VALUES ('a','a@x.com','张三','x','local',1), ('b','b@x.com','李四','x','local',1)`)
	pid := setupProject(t, conn)
	ids, _ := resolveUserIDs(conn, []string{"张三", "李四"})
	saveProjectOwners(conn, pid, ids)

	body, _ := json.Marshal(map[string]interface{}{
		"name": "任务D", "task_type": "task", "status": "open",
		"start_date": "2026-08-03", "end_date": "2026-08-10", "duration_days": 5,
	})
	r := chi.NewRouter()
	r.Post("/api/projects/{id}/tasks", h.CreateTask)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/projects/%d/tasks", pid), bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("状态码 = %d, body=%s", w.Code, w.Body.String())
	}
	var n int
	conn.QueryRow(`SELECT COUNT(*) FROM task_assignees WHERE task_id IN (SELECT id FROM tasks WHERE project_id=? AND name='任务D')`, pid).Scan(&n)
	if n != 2 {
		t.Errorf("默认项目 owner 后关联行数 = %d, want 2", n)
	}
}

// UpdateTask 传 assignee_ids 含重复 → 去重后写入,响应与快照无重复
func TestUpdateTaskAssigneeIDsDedup(t *testing.T) {
	conn, h := testTaskHandler(t)
	conn.Exec(`INSERT INTO users (login, email, display_name, password_hash, auth_source, is_active) VALUES ('a','a@x.com','张三','x','local',1), ('b','b@x.com','李四','x','local',1)`)
	pid := setupProject(t, conn)
	tid := setupTask(t, conn, pid, "任务E", "", "")
	var uid1, uid2 int64
	conn.QueryRow(`SELECT id FROM users WHERE email='a@x.com'`).Scan(&uid1)
	conn.QueryRow(`SELECT id FROM users WHERE email='b@x.com'`).Scan(&uid2)

	body, _ := json.Marshal(map[string]interface{}{
		"name": "任务E", "task_type": "task", "status": "open",
		"start_date": "2026-08-03", "end_date": "2026-08-10", "duration_days": 5,
		"assignee_ids": []int64{uid1, uid2, uid1}, "version": 1, // 含重复 uid1
	})
	r := chi.NewRouter()
	r.Put("/api/projects/{id}/tasks/{taskID}", h.UpdateTask)
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/projects/%d/tasks/%d", pid, tid), bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, body=%s", w.Code, w.Body.String())
	}
	// 响应 assignee_ids 去重
	var resp struct {
		Data models.Task `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Data.AssigneeIDs) != 2 {
		t.Errorf("响应 assignee_ids = %v, want 2 个(去重)", resp.Data.AssigneeIDs)
	}
	if resp.Data.Assignee != "张三; 李四" {
		t.Errorf("响应快照 = %q, want %q", resp.Data.Assignee, "张三; 李四")
	}
	// DB 快照列无重复
	var snap string
	conn.QueryRow(`SELECT assignee FROM tasks WHERE id=?`, tid).Scan(&snap)
	if snap != "张三; 李四" {
		t.Errorf("DB 快照列 = %q, want %q", snap, "张三; 李四")
	}
	// 关联表去重
	var n int
	conn.QueryRow(`SELECT COUNT(*) FROM task_assignees WHERE task_id=?`, tid).Scan(&n)
	if n != 2 {
		t.Errorf("关联表行数 = %d, want 2(去重)", n)
	}
}

// ListTasks 返回 assignee_ids 与分号分隔 assignee
func TestListTasksReturnsAssigneeIDs(t *testing.T) {
	conn, h := testTaskHandler(t)
	conn.Exec(`INSERT INTO users (login, email, display_name, password_hash, auth_source, is_active) VALUES ('a','a@x.com','张三','x','local',1), ('b','b@x.com','李四','x','local',1)`)
	pid := setupProject(t, conn)
	tid := setupTask(t, conn, pid, "任务E", "", "")
	ids, _ := resolveUserIDs(conn, []string{"张三", "李四"})
	saveTaskAssignees(conn, tid, ids)

	r := chi.NewRouter()
	r.Get("/api/projects/{id}/tasks", h.ListTasks)
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/projects/%d/tasks", pid), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d", w.Code)
	}
	var resp struct {
		Data struct {
			Tasks []models.Task `json:"tasks"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Data.Tasks) != 1 {
		t.Fatalf("任务数 = %d", len(resp.Data.Tasks))
	}
	task := resp.Data.Tasks[0]
	if len(task.AssigneeIDs) != 2 {
		t.Errorf("assignee_ids = %v, want 2 个", task.AssigneeIDs)
	}
	if task.Assignee != "张三; 李四" {
		t.Errorf("assignee = %q, want %q", task.Assignee, "张三; 李四")
	}
}

// GetMyTasks 双视角:view=task(默认)我名下任务;view=project 我名下项目的任务
func TestGetMyTasksViews(t *testing.T) {
	conn, h := testTaskHandler(t)
	conn.Exec(`INSERT INTO users (login, email, display_name, password_hash, auth_source, is_active) VALUES ('me@x.com','me@x.com','Me User','x','local',1), ('a@x.com','a@x.com','Alpha','x','local',1), ('b@x.com','b@x.com','Beta','x','local',1)`)
	var me, alpha int64
	conn.QueryRow(`SELECT id FROM users WHERE email='me@x.com'`).Scan(&me)
	conn.QueryRow(`SELECT id FROM users WHERE email='a@x.com'`).Scan(&alpha)

	// 项目1:owner = Me User;项目2:owner = Alpha
	r1, _ := conn.Exec(`INSERT INTO projects (name, start_date, end_date, status, owner) VALUES ('我的项目','2026-08-01','2026-08-31','active','Me User')`)
	pid1, _ := r1.LastInsertId()
	r2, _ := conn.Exec(`INSERT INTO projects (name, start_date, end_date, status, owner) VALUES ('他人项目','2026-08-01','2026-08-31','active','Alpha')`)
	pid2, _ := r2.LastInsertId()

	// 建立 project_owners:pid1→me,pid2→alpha
	saveProjectOwners(conn, pid1, []int64{me})
	saveProjectOwners(conn, pid2, []int64{alpha})

	// 我的项目里:任务1 进行中、assignee=Beta(不是我);任务2 open、明天开始
	conn.Exec(`INSERT INTO tasks (project_id, name, task_type, status, start_date, end_date, duration_days, progress_pct, sort_order) VALUES (?, '项目任务', 'task', 'in_progress', '2026-08-05', '2026-08-15', 5, 30, 0)`, pid1)
	var t1 int64
	conn.QueryRow(`SELECT id FROM tasks WHERE project_id=? AND name='项目任务'`, pid1).Scan(&t1)
	var beta int64
	conn.QueryRow(`SELECT id FROM users WHERE email='b@x.com'`).Scan(&beta)
	saveTaskAssignees(conn, t1, []int64{beta})
	conn.Exec(`INSERT INTO tasks (project_id, name, task_type, status, start_date, end_date, duration_days, progress_pct, sort_order) VALUES (?, '项目任务2', 'task', 'open', '2026-08-12', '2026-08-16', 5, 0, 1)`, pid1)
	var t2 int64
	conn.QueryRow(`SELECT id FROM tasks WHERE project_id=? AND name='项目任务2'`, pid1).Scan(&t2)
	saveTaskAssignees(conn, t2, []int64{me}) // 我名下

	// 他人项目里:任务3 进行中、assignee=我
	conn.Exec(`INSERT INTO tasks (project_id, name, task_type, status, start_date, end_date, duration_days, progress_pct, sort_order) VALUES (?, '他人项目任务', 'task', 'in_progress', '2026-08-05', '2026-08-15', 5, 30, 0)`, pid2)
	var t3 int64
	conn.QueryRow(`SELECT id FROM tasks WHERE project_id=? AND name='他人项目任务'`, pid2).Scan(&t3)
	saveTaskAssignees(conn, t3, []int64{me})

	do := func(view string) ([]string, []string) {
		r := chi.NewRouter()
		r.Get("/api/tasks/mine", h.GetMyTasks)
		req := httptest.NewRequest(http.MethodGet, "/api/tasks/mine?view="+view, nil)
		ctx := context.WithValue(req.Context(), auth.UserIDKey, me)
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("view=%s 状态码 = %d, body=%s", view, w.Code, w.Body.String())
		}
		var resp struct {
			Data struct {
				Mine     []MyTaskItem `json:"mine"`
				Starting []MyTaskItem `json:"starting"`
			} `json:"data"`
		}
		json.Unmarshal(w.Body.Bytes(), &resp)
		var m, s []string
		for _, x := range resp.Data.Mine {
			m = append(m, x.Name)
		}
		for _, x := range resp.Data.Starting {
			s = append(s, x.Name)
		}
		return m, s
	}

	// task 视角:mine 只含我名下的(他人项目任务 in_progress);starting(12 天内)= 项目任务2
	mineT, startingT := do("task")
	if len(mineT) != 1 || mineT[0] != "他人项目任务" {
		t.Errorf("task视角 mine = %v, want [他人项目任务]", mineT)
	}
	if len(startingT) != 1 || startingT[0] != "项目任务2" {
		t.Errorf("task视角 starting = %v, want [项目任务2]", startingT)
	}
	// project 视角:mine = 我名下项目全部 in_progress(项目任务);starting = 我名下项目 open 即将开始(项目任务2)
	mineP, startingP := do("project")
	if len(mineP) != 1 || mineP[0] != "项目任务" {
		t.Errorf("project视角 mine = %v, want [项目任务]", mineP)
	}
	if len(startingP) != 1 || startingP[0] != "项目任务2" {
		t.Errorf("project视角 starting = %v, want [项目任务2]", startingP)
	}
}
