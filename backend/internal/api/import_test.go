package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"followitup/internal/db"

	"github.com/go-chi/chi/v5"
)

// testTaskHandler 构造测试用 TaskHandler（hub 为 nil，broadcastChange 内部判空跳过）
func testTaskHandler(t *testing.T) (*sql.DB, *TaskHandler) {
	t.Helper()
	d, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d.Conn, &TaskHandler{db: d.Conn}
}

// setupProject 建测试项目，返回项目 ID
func setupProject(t *testing.T, conn *sql.DB) int64 {
	t.Helper()
	res, err := conn.Exec(`INSERT INTO projects (name, start_date, end_date, status, owner) VALUES ('测试项目', '2026-08-01', '2026-08-31', 'active', 'owner@test.local')`)
	if err != nil {
		t.Fatalf("建项目: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

// setupTask 建任务，返回任务 ID（可选 actual 日期）
func setupTask(t *testing.T, conn *sql.DB, pid int64, name string, actualStart, actualEnd string) int64 {
	t.Helper()
	res, err := conn.Exec(
		`INSERT INTO tasks (project_id, name, task_type, status, start_date, end_date, duration_days, progress_pct, actual_start, actual_end, version)
		 VALUES (?, ?, 'task', 'open', '2026-08-01', '2026-08-10', 5, 0, ?, ?, 1)`,
		pid, name, actualStart, actualEnd)
	if err != nil {
		t.Fatalf("建任务: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

// CSV 导入：UI 展示词（待开始/已延期）与英文/其他中文词映射正确；未知状态/重复 WBS 跳过并提示；
// 进度带 % 合法；导入任务 actual 默认跟随计划
func TestImportTasksStatusWordsAndGuards(t *testing.T) {
	conn, h := testTaskHandler(t)
	pid := setupProject(t, conn)

	csv := strings.Join([]string{
		"任务名,WBS,工期,开始日期,负责人,进度,状态",
		"需求分析,1,5,2026-08-03,张三,50%,进行中",
		"编码,1.1,3,2026-08-10,李四,60%,已延期",   // UI 词"已延期" → delayed
		"测试,2,2,2026-08-15,王五,30%,待开始",    // UI 词"待开始" → open
		"坏状态,3,2,2026-08-15,张三,20%,完成啦",  // 未知状态 → 跳过
		"重复,1,2,2026-08-20,张三,10%,",        // WBS 1 重复 → 跳过
		"文档,4,2,2026-08-21,张三,80%,",        // 进度 80% 带 % → 80
	}, "\n")

	body, _ := json.Marshal(map[string]string{"csv": csv})
	r := chi.NewRouter()
	r.Post("/api/projects/{id}/tasks/import", h.ImportTasks)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/projects/%d/tasks/import", pid), bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("ImportTasks 状态码 = %d, body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			Imported int      `json:"imported"`
			Skipped  int      `json:"skipped"`
			Errors   []string `json:"errors"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应: %v", err)
	}
	if resp.Data.Imported != 4 {
		t.Errorf("imported = %d, want 4（响应 %s）", resp.Data.Imported, w.Body.String())
	}
	if resp.Data.Skipped != 2 {
		t.Errorf("skipped = %d, want 2（响应 %s）", resp.Data.Skipped, w.Body.String())
	}

	// 状态词映射
	var delayed, pending string
	conn.QueryRow(`SELECT status FROM tasks WHERE project_id=? AND name='编码'`, pid).Scan(&delayed)
	conn.QueryRow(`SELECT status FROM tasks WHERE project_id=? AND name='测试'`, pid).Scan(&pending)
	if delayed != "delayed" {
		t.Errorf("'已延期' → %q, want delayed", delayed)
	}
	if pending != "open" {
		t.Errorf("'待开始' → %q, want open", pending)
	}
	// actual 默认跟随计划
	var actualStart, actualEnd string
	conn.QueryRow(`SELECT actual_start, actual_end FROM tasks WHERE project_id=? AND name='需求分析'`, pid).Scan(&actualStart, &actualEnd)
	if actualStart != "2026-08-03" || actualEnd == "" {
		t.Errorf("导入任务 actual = (%q,%q), want 跟随计划 (2026-08-03, 非空)", actualStart, actualEnd)
	}
	// 重复 WBS 未插入
	var dupCnt int
	conn.QueryRow(`SELECT COUNT(*) FROM tasks WHERE project_id=? AND name='重复'`, pid).Scan(&dupCnt)
	if dupCnt != 0 {
		t.Error("重复 WBS 的行被插入，应跳过")
	}
	// 进度 80% 解析为 80
	var prog float64
	conn.QueryRow(`SELECT progress_pct FROM tasks WHERE project_id=? AND name='文档'`, pid).Scan(&prog)
	if prog != 80 {
		t.Errorf("'80%%' 进度 = %v, want 80", prog)
	}
}

// UpdateTask 部分更新（请求体不含 actual_*）：必须保留 DB 中用户手填的实际日期，不静默覆盖
func TestUpdateTaskPartialUpdateKeepsActual(t *testing.T) {
	conn, h := testTaskHandler(t)
	pid := setupProject(t, conn)
	tid := setupTask(t, conn, pid, "任务A", "2026-07-20", "2026-07-25")

	// 只改名称（带全字段除 actual_*）——模拟脚本部分更新
	payload := map[string]interface{}{
		"name": "任务A改名", "task_type": "task", "status": "in_progress",
		"start_date": "2026-08-01", "end_date": "2026-08-10", "duration_days": 5,
		"progress_pct": 30, "manual_scheduled": false, "version": 1,
	}
	body, _ := json.Marshal(payload)
	r := chi.NewRouter()
	r.Put("/api/projects/{id}/tasks/{taskID}", h.UpdateTask)
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/projects/%d/tasks/%d", pid, tid), bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("UpdateTask 状态码 = %d, body=%s", w.Code, w.Body.String())
	}
	var as, ae string
	conn.QueryRow(`SELECT actual_start, actual_end FROM tasks WHERE id=?`, tid).Scan(&as, &ae)
	if as != "2026-07-20" || ae != "2026-07-25" {
		t.Errorf("部分更新后 actual = (%q,%q), want 保留 (2026-07-20,2026-07-25)", as, ae)
	}
}

// UpdateTask 校验：实际开始晚于（计划兜底出的）实际结束 → 400 INVALID_ACTUAL
func TestUpdateTaskInvalidActual(t *testing.T) {
	conn, h := testTaskHandler(t)
	pid := setupProject(t, conn)
	tid := setupTask(t, conn, pid, "任务B", "", "")

	payload := map[string]interface{}{
		"name": "任务B", "task_type": "task", "status": "in_progress",
		"start_date": "2026-08-01", "end_date": "2026-08-10", "duration_days": 5,
		"progress_pct": 30, "manual_scheduled": false, "version": 1,
		"actual_start": "2026-08-15", // 晚于计划结束；actual_end 未传 → 兜底为计划结束 08-10
	}
	body, _ := json.Marshal(payload)
	r := chi.NewRouter()
	r.Put("/api/projects/{id}/tasks/{taskID}", h.UpdateTask)
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/projects/%d/tasks/%d", pid, tid), bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("UpdateTask 状态码 = %d, want 400", w.Code)
	}
	if !strings.Contains(w.Body.String(), "INVALID_ACTUAL") {
		t.Errorf("错误码 = %s, want INVALID_ACTUAL", w.Body.String())
	}
}
