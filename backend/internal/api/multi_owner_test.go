package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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
		t.Fatalf("覆盖写状态码 = %d, body=%s", w2.Code, w2.Body.String())
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

// 创建项目:owner_ids 数组 + 文本兼容 + 校验失败 400
func TestCreateProjectWithOwnerIDs(t *testing.T) {
	conn, _ := testTaskHandler(t)
	ph := &ProjectHandler{db: conn}
	conn.Exec(`INSERT INTO users (login, email, display_name, password_hash, auth_source, is_active) VALUES ('a@x.com','a@x.com','张三','x','local',1), ('b@x.com','b@x.com','李四','x','local',1)`)
	var uid1 int64
	conn.QueryRow(`SELECT id FROM users WHERE email='a@x.com'`).Scan(&uid1)

	r := chi.NewRouter()
	r.Post("/api/projects", ph.CreateProject)

	// 正常:owner_ids 双值(含无效 999 → 400)
	body, _ := json.Marshal(map[string]interface{}{
		"name": "项目A", "start_date": "2026-08-01", "end_date": "2026-08-31",
		"owner_ids": []int64{uid1, 999},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("无效 owner_ids 状态码 = %d, want 400", w.Code)
	}
	// 文本兼容
	body2, _ := json.Marshal(map[string]interface{}{
		"name": "项目A", "start_date": "2026-08-01", "end_date": "2026-08-31",
		"owner": "张三;李四",
	})
	req2 := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader(body2))
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusCreated {
		t.Fatalf("文本创建状态码 = %d, body=%s", w2.Code, w2.Body.String())
	}
	var pid int64
	conn.QueryRow(`SELECT id FROM projects WHERE name='项目A'`).Scan(&pid)
	var n int
	conn.QueryRow(`SELECT COUNT(*) FROM project_owners WHERE project_id=?`, pid).Scan(&n)
	if n != 2 {
		t.Errorf("project_owners 行数 = %d, want 2", n)
	}
}

// UpdateProject 改 owner:open 任务快照列与关联表同步,version 递增
func TestUpdateProjectReassignsTasks(t *testing.T) {
	conn, _ := testTaskHandler(t)
	ph := &ProjectHandler{db: conn}
	conn.Exec(`INSERT INTO users (login, email, display_name, password_hash, auth_source, is_active) VALUES ('a@x.com','a@x.com','张三','x','local',1), ('b@x.com','b@x.com','李四','x','local',1)`)
	var uid1, uid2 int64
	conn.QueryRow(`SELECT id FROM users WHERE email='a@x.com'`).Scan(&uid1)
	conn.QueryRow(`SELECT id FROM users WHERE email='b@x.com'`).Scan(&uid2)

	// 建项目:owner=张三
	r1, _ := conn.Exec(`INSERT INTO projects (name, start_date, end_date, status, owner) VALUES ('改派测试','2026-08-01','2026-08-31','active','张三')`)
	pid, _ := r1.LastInsertId()
	saveProjectOwners(conn, pid, []int64{uid1})
	// 建 open 任务:assignee=张三
	r2, _ := conn.Exec(`INSERT INTO tasks (project_id, name, task_type, status, start_date, end_date, duration_days, progress_pct, sort_order, version) VALUES (?, 'open任务','task','open','2026-08-03','2026-08-10',5,0,0,1)`, pid)
	tid, _ := r2.LastInsertId()
	saveTaskAssignees(conn, tid, []int64{uid1})

	r := chi.NewRouter()
	r.Put("/api/projects/{id}", ph.UpdateProject)
	// 改 owner 为李四(owner_ids 数组)
	body, _ := json.Marshal(map[string]interface{}{
		"name": "改派测试", "start_date": "2026-08-01", "end_date": "2026-08-31",
		"owner_ids": []int64{uid2}, "status": "active",
	})
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/projects/%d", pid), bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("改派状态码 = %d, body=%s", w.Code, w.Body.String())
	}

	// 快照列已更新
	var snap string
	var ver int
	conn.QueryRow(`SELECT assignee, version FROM tasks WHERE id=?`, tid).Scan(&snap, &ver)
	if snap != "李四" {
		t.Errorf("改派后快照 = %q, want 李四", snap)
	}
	if ver != 2 {
		t.Errorf("改派后 version = %d, want 2(递增)", ver)
	}
	// 关联表同步
	var n int
	conn.QueryRow(`SELECT COUNT(*) FROM task_assignees WHERE task_id=?`, tid).Scan(&n)
	if n != 1 {
		t.Errorf("改派后关联表行数 = %d, want 1", n)
	}
}

// UpdateProject 相同 owner 集合 + 改日期:不触发改派,任务 assignee/version 不变(回归 C1)
func TestUpdateProjectSameOwnerDoesNotReassign(t *testing.T) {
	conn, _ := testTaskHandler(t)
	ph := &ProjectHandler{db: conn}
	conn.Exec(`INSERT INTO users (login, email, display_name, password_hash, auth_source, is_active) VALUES ('a@x.com','a@x.com','张三','x','local',1), ('b@x.com','b@x.com','李四','x','local',1)`)
	var uid1, uid2 int64
	conn.QueryRow(`SELECT id FROM users WHERE email='a@x.com'`).Scan(&uid1)
	conn.QueryRow(`SELECT id FROM users WHERE email='b@x.com'`).Scan(&uid2)

	// 建项目:owner=张三+李四
	r1, _ := conn.Exec(`INSERT INTO projects (name, start_date, end_date, status, owner) VALUES ('相同集合测试','2026-08-01','2026-08-31','active','张三; 李四')`)
	pid, _ := r1.LastInsertId()
	saveProjectOwners(conn, pid, []int64{uid1, uid2})
	// 建 open 任务:assignee=张三(任务级特意设置,不应被覆盖)
	r2, _ := conn.Exec(`INSERT INTO tasks (project_id, name, task_type, status, assignee, start_date, end_date, duration_days, progress_pct, sort_order, version) VALUES (?, 'open任务','task','open','张三','2026-08-03','2026-08-10',5,0,0,1)`, pid)
	tid, _ := r2.LastInsertId()
	saveTaskAssignees(conn, tid, []int64{uid1})

	r := chi.NewRouter()
	r.Put("/api/projects/{id}", ph.UpdateProject)
	// 发送相同 owner_ids + 修改 start_date(触发前端 ProjectDetail saveDate 场景)
	body, _ := json.Marshal(map[string]interface{}{
		"name": "相同集合测试", "start_date": "2026-08-05", "end_date": "2026-08-31",
		"owner_ids": []int64{uid1, uid2}, "status": "active",
	})
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/projects/%d", pid), bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("更新状态码 = %d, body=%s", w.Code, w.Body.String())
	}

	// 任务 assignee 应保持张三(不被项目 owner 集合覆盖)
	var snap string
	var ver int
	conn.QueryRow(`SELECT assignee, version FROM tasks WHERE id=?`, tid).Scan(&snap, &ver)
	if snap != "张三" {
		t.Errorf("相同owner集合改日期后快照 = %q, want 张三(不应被覆盖)", snap)
	}
	if ver != 1 {
		t.Errorf("相同owner集合改日期后 version = %d, want 1(不应递增)", ver)
	}
}

// UpdateProject 清空 owner_ids:空数组清空项目负责人 + open 任务改派为空
func TestUpdateProjectClearOwnerIDs(t *testing.T) {
	conn, _ := testTaskHandler(t)
	ph := &ProjectHandler{db: conn}
	conn.Exec(`INSERT INTO users (login, email, display_name, password_hash, auth_source, is_active) VALUES ('a@x.com','a@x.com','张三','x','local',1), ('b@x.com','b@x.com','李四','x','local',1)`)
	var uid1, uid2 int64
	conn.QueryRow(`SELECT id FROM users WHERE email='a@x.com'`).Scan(&uid1)
	conn.QueryRow(`SELECT id FROM users WHERE email='b@x.com'`).Scan(&uid2)

	// 建项目:owner=张三
	r1, _ := conn.Exec(`INSERT INTO projects (name, start_date, end_date, status, owner) VALUES ('清空测试','2026-08-01','2026-08-31','active','张三')`)
	pid, _ := r1.LastInsertId()
	saveProjectOwners(conn, pid, []int64{uid1})
	// 建 open 任务:assignee=张三
	r2, _ := conn.Exec(`INSERT INTO tasks (project_id, name, task_type, status, start_date, end_date, duration_days, progress_pct, sort_order, version) VALUES (?, 'open任务','task','open','2026-08-03','2026-08-10',5,0,0,1)`, pid)
	tid, _ := r2.LastInsertId()
	saveTaskAssignees(conn, tid, []int64{uid1})

	// 已有一行 project_owners
	var n int
	conn.QueryRow(`SELECT COUNT(*) FROM project_owners WHERE project_id=?`, pid).Scan(&n)
	if n != 1 {
		t.Fatalf("初始 project_owners = %d, want 1", n)
	}

	r := chi.NewRouter()
	r.Put("/api/projects/{id}", ph.UpdateProject)
	// 清空:传 owner_ids 空数组
	body, _ := json.Marshal(map[string]interface{}{
		"name": "清空测试", "start_date": "2026-08-01", "end_date": "2026-08-31",
		"owner_ids": []int64{}, "status": "active",
	})
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/projects/%d", pid), bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("清空状态码 = %d, body=%s", w.Code, w.Body.String())
	}

	// project_owners 已清空
	conn.QueryRow(`SELECT COUNT(*) FROM project_owners WHERE project_id=?`, pid).Scan(&n)
	if n != 0 {
		t.Errorf("清空后 project_owners = %d, want 0", n)
	}
	// open 任务 assignee 改派为空(快照列清空)
	var snap string
	conn.QueryRow(`SELECT assignee FROM tasks WHERE id=?`, tid).Scan(&snap)
	if snap != "" {
		t.Errorf("清空后任务快照 = %q, want 空", snap)
	}
	// 关联表清空
	conn.QueryRow(`SELECT COUNT(*) FROM task_assignees WHERE task_id=?`, tid).Scan(&n)
	if n != 0 {
		t.Errorf("清空后 task_assignees = %d, want 0", n)
	}
}

// UpdateProject 复合场景:owner_ids 为空数组但 owner 文本与旧值不同 → 以 owner_ids 为准,不回退文本
func TestUpdateProjectClearOwnerIDsCompound(t *testing.T) {
	conn, _ := testTaskHandler(t)
	ph := &ProjectHandler{db: conn}
	conn.Exec(`INSERT INTO users (login, email, display_name, password_hash, auth_source, is_active) VALUES ('a@x.com','a@x.com','张三','x','local',1), ('b@x.com','b@x.com','李四','x','local',1)`)
	var uid1, uid2 int64
	conn.QueryRow(`SELECT id FROM users WHERE email='a@x.com'`).Scan(&uid1)
	conn.QueryRow(`SELECT id FROM users WHERE email='b@x.com'`).Scan(&uid2)

	// 建项目:owner=张三
	r1, _ := conn.Exec(`INSERT INTO projects (name, start_date, end_date, status, owner) VALUES ('复合测试','2026-08-01','2026-08-31','active','张三')`)
	pid, _ := r1.LastInsertId()
	saveProjectOwners(conn, pid, []int64{uid1})

	// 先改为李四(第一次 PUT)
	r := chi.NewRouter()
	r.Put("/api/projects/{id}", ph.UpdateProject)
	body1, _ := json.Marshal(map[string]interface{}{
		"name": "复合测试", "start_date": "2026-08-01", "end_date": "2026-08-31",
		"owner_ids": []int64{uid2}, "status": "active",
	})
	req1 := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/projects/%d", pid), bytes.NewReader(body1))
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("第一次改派状态码 = %d", w1.Code)
	}
	// 确认已改为李四
	got, _ := loadProjectOwners(conn, pid)
	if len(got) != 1 || got[0] != uid2 {
		t.Fatalf("第一次改派后 owners = %v, want [%d]", got, uid2)
	}

	// 复合场景:owner_ids 空数组,但 owner 文本是旧值"张三"≠ DB 旧值"李四"
	// → 必须以 owner_ids 为准(清空),不能因为文本不同就回退到文本"张三"
	body2, _ := json.Marshal(map[string]interface{}{
		"name": "复合测试", "start_date": "2026-08-01", "end_date": "2026-08-31",
		"owner_ids": []int64{}, "owner": "张三", "status": "active",
	})
	req2 := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/projects/%d", pid), bytes.NewReader(body2))
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("复合清空状态码 = %d, body=%s", w2.Code, w2.Body.String())
	}

	// 检查:project_owners 必须为空(以 owner_ids 空数组为准,不回退)
	var n int
	conn.QueryRow(`SELECT COUNT(*) FROM project_owners WHERE project_id=?`, pid).Scan(&n)
	if n != 0 {
		t.Errorf("复合清空后 project_owners = %d, want 0(以 owner_ids 为准,不回退)", n)
	}
	// 快照列也必须为空
	var snap string
	conn.QueryRow(`SELECT owner FROM projects WHERE id=?`, pid).Scan(&snap)
	if snap != "" {
		t.Errorf("复合清空后快照列 = %q, want 空", snap)
	}
}

// 项目列表返回 owner_ids;CopyProject 复制两套关联表
func TestProjectListAndCopyWithOwners(t *testing.T) {
	conn, _ := testTaskHandler(t)
	ph := &ProjectHandler{db: conn}
	conn.Exec(`INSERT INTO users (login, email, display_name, password_hash, auth_source, is_active) VALUES ('a@x.com','a@x.com','张三','x','local',1), ('b@x.com','b@x.com','李四','x','local',1)`)
	ids, _ := resolveUserIDs(conn, []string{"张三", "李四"})
	r1, _ := conn.Exec(`INSERT INTO projects (name, start_date, end_date, status, owner) VALUES ('源项目','2026-08-01','2026-08-31','active','张三; 李四')`)
	pid, _ := r1.LastInsertId()
	saveProjectOwners(conn, pid, ids)
	// 任务+负责人
	r2, _ := conn.Exec(`INSERT INTO tasks (project_id, name, task_type, status, start_date, end_date, duration_days, progress_pct, sort_order) VALUES (?, '任务A','task','open','2026-08-03','2026-08-10',5,0,0)`, pid)
	tid, _ := r2.LastInsertId()
	saveTaskAssignees(conn, tid, ids)

	// 项目列表含 owner_ids
	r := chi.NewRouter()
	r.Get("/api/projects", ph.ProjectList)
	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("列表状态码 = %d", w.Code)
	}
	var resp struct {
		Data []struct {
			ID       int64   `json:"id"`
			Owner    string  `json:"owner"`
			OwnerIDs []int64 `json:"owner_ids"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	list := resp.Data
	found := false
	for _, p := range list {
		if p.ID == pid {
			found = true
			if len(p.OwnerIDs) != 2 || p.Owner != "张三; 李四" {
				t.Errorf("项目 owner = (%q, %v)", p.Owner, p.OwnerIDs)
			}
		}
	}
	if !found {
		t.Fatal("列表中找不到源项目")
	}

	// 复制项目
	r2r := chi.NewRouter()
	r2r.Post("/api/projects/{id}/copy", ph.CopyProject)
	req2 := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/projects/%d/copy", pid), nil)
	ctx := context.WithValue(req2.Context(), auth.UserIDKey, int64(1))
	req2 = req2.WithContext(ctx)
	w2 := httptest.NewRecorder()
	r2r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusCreated {
		t.Fatalf("复制状态码 = %d, body=%s", w2.Code, w2.Body.String())
	}
	var newID int64
	conn.QueryRow(`SELECT id FROM projects WHERE name='源项目(副本)'`).Scan(&newID)
	var n int
	conn.QueryRow(`SELECT COUNT(*) FROM project_owners WHERE project_id=?`, newID).Scan(&n)
	if n != 2 {
		t.Errorf("副本 project_owners = %d, want 2", n)
	}
	conn.QueryRow(`SELECT COUNT(*) FROM task_assignees ta JOIN tasks t ON t.id = ta.task_id WHERE t.project_id=?`, newID).Scan(&n)
	if n != 2 {
		t.Errorf("副本 task_assignees = %d, want 2", n)
	}
}

// CSV 导入:负责人列分号多值,解析失败归未分配+提示
func TestImportTasksMultiAssignee(t *testing.T) {
	conn, h := testTaskHandler(t)
	conn.Exec(`INSERT INTO users (login, email, display_name, password_hash, auth_source, is_active) VALUES ('a@x.com','a@x.com','张三','x','local',1), ('b@x.com','b@x.com','李四','x','local',1)`)
	pid := setupProject(t, conn)
	// 种子:给项目补真实 project_owners(与 setupProject 文本 owner 对应)。
	// 否则旧代码「全解析失败兜底项目 owner」查询返回 0 行,「测试负责人数 = 0」断言无回归判别力。
	res, err := conn.Exec(`INSERT INTO users (login, email, display_name, password_hash, auth_source, is_active) VALUES ('owner@test.local','owner@test.local','项目负责人','x','local',1)`)
	if err != nil {
		t.Fatalf("建 owner 用户: %v", err)
	}
	ownerID, _ := res.LastInsertId()
	saveProjectOwners(conn, pid, []int64{ownerID})

	csv := strings.Join([]string{
		"任务名,WBS,工期,开始日期,负责人,进度,状态",
		"需求,1,3,2026-08-03,张三;李四,50%,进行中",   // 双负责人
		"编码,2,3,2026-08-10,张三;王五,0%,",          // 王五不存在 → 只留张三 + 提示
		"测试,3,1,2026-08-17,王五;赵六,0%,",          // 全解析失败 → 归未分配(不兜底项目 owner)+ 双提示
	}, "\n")
	body, _ := json.Marshal(map[string]string{"csv": csv})
	r := chi.NewRouter()
	r.Post("/api/projects/{id}/tasks/import", h.ImportTasks)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/projects/%d/tasks/import", pid), bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			Imported int      `json:"imported"`
			Errors   []string `json:"errors"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Data.Imported != 3 {
		t.Fatalf("imported = %d, want 3", resp.Data.Imported)
	}
	// 需求任务:2 个负责人
	var tid1 int64
	conn.QueryRow(`SELECT id FROM tasks WHERE project_id=? AND name='需求'`, pid).Scan(&tid1)
	var n int
	conn.QueryRow(`SELECT COUNT(*) FROM task_assignees WHERE task_id=?`, tid1).Scan(&n)
	if n != 2 {
		t.Errorf("需求负责人数 = %d, want 2", n)
	}
	// 编码任务:1 个负责人(王五被跳过)
	var tid2 int64
	conn.QueryRow(`SELECT id FROM tasks WHERE project_id=? AND name='编码'`, pid).Scan(&tid2)
	conn.QueryRow(`SELECT COUNT(*) FROM task_assignees WHERE task_id=?`, tid2).Scan(&n)
	if n != 1 {
		t.Errorf("编码负责人数 = %d, want 1", n)
	}
	// 测试任务:全部解析失败 → 归未分配(0 个负责人,不兜底项目 owner)
	var tid3 int64
	conn.QueryRow(`SELECT id FROM tasks WHERE project_id=? AND name='测试'`, pid).Scan(&tid3)
	conn.QueryRow(`SELECT COUNT(*) FROM task_assignees WHERE task_id=?`, tid3).Scan(&n)
	if n != 0 {
		t.Errorf("测试负责人数 = %d, want 0(归未分配)", n)
	}
	// 提示含王五与赵六(各一条)
	found := false
	foundZhao := false
	for _, e := range resp.Data.Errors {
		if strings.Contains(e, "王五") {
			found = true
		}
		if strings.Contains(e, "赵六") {
			foundZhao = true
		}
	}
	if !found {
		t.Errorf("errors = %v, want 含王五提示", resp.Data.Errors)
	}
	if !foundZhao {
		t.Errorf("errors = %v, want 含赵六提示", resp.Data.Errors)
	}
}

// 项目无负责人(project_owners 空):GetProject 200 且 owner_ids 空;ProjectList 仍含该项目
// (COALESCE 防 NULL scan 报错导致项目从列表剔除/详情 404——bug-237 根因回归)
func TestGetProjectAndListWithNoOwners(t *testing.T) {
	conn, _ := testTaskHandler(t)
	ph := &ProjectHandler{db: conn}
	pid := setupProject(t, conn) // 不写 project_owners

	// GetProject:无 owner 项目必须 200(不能 404)
	r := chi.NewRouter()
	r.Get("/api/projects/{id}", ph.GetProject)
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/projects/%d", pid), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("无owner项目 GetProject 状态码 = %d, body=%s", w.Code, w.Body.String())
	}
	var pResp struct {
		Data models.Project `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &pResp)
	if len(pResp.Data.OwnerIDs) != 0 {
		t.Errorf("无owner项目 owner_ids = %v, want []", pResp.Data.OwnerIDs)
	}

	// ProjectList:无 owner 项目仍出现在列表(不因 NULL scan 被剔除)
	r2 := chi.NewRouter()
	r2.Get("/api/projects", ph.ProjectList)
	req2 := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	w2 := httptest.NewRecorder()
	r2.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("ProjectList 状态码 = %d", w2.Code)
	}
	var listResp struct {
		Data []struct {
			ID       int64   `json:"id"`
			OwnerIDs []int64 `json:"owner_ids"`
		} `json:"data"`
	}
	json.Unmarshal(w2.Body.Bytes(), &listResp)
	found := false
	for _, p := range listResp.Data {
		if p.ID == pid {
			found = true
			if len(p.OwnerIDs) != 0 {
				t.Errorf("列表中无owner项目 owner_ids = %v, want []", p.OwnerIDs)
			}
		}
	}
	if !found {
		t.Error("无owner项目从 ProjectList 消失(应保留)")
	}
}
