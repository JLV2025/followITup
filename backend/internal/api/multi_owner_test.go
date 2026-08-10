package api

import (
	"testing"
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
