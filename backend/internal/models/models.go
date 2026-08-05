package models

// User 用户模型
type User struct {
	ID          int64     `json:"id"`
	Login       string    `json:"login"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	AuthSource  string    `json:"auth_source"` // "local" 或 "ldap"
	IsAdmin     bool      `json:"is_admin"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   string    `json:"created_at"`
	UpdatedAt   string    `json:"updated_at"`
}

// Project 项目模型
type Project struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	StartDate   string    `json:"start_date"`
	EndDate     string    `json:"end_date"`
	ScheduleDirection string `json:"schedule_direction"` // forward=正推 backward=倒推
	Status      string    `json:"status"` // active | completed | archived
	BaselineCreatedAt string `json:"baseline_created_at"`
	BaselineCreatedBy string `json:"baseline_created_by"`
	IsPublic    bool      `json:"is_public"`
	DeletedAt   *string   `json:"deleted_at,omitempty"`
	CreatedAt   string    `json:"created_at"`
	UpdatedAt   string    `json:"updated_at"`
}

// Task 任务模型，支持 WBS 层级
type Task struct {
	ID              int64     `json:"id"`
	ProjectID       int64     `json:"project_id"`
	ParentID        *int64    `json:"parent_id"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	TaskType        string    `json:"task_type"`        // task | milestone | project_summary
	Status          string    `json:"status"`           // open | in_progress | completed | delayed
	Priority        string    `json:"priority"`         // low | medium | high | critical
	Assignee        string    `json:"assignee"`
	StartDate       string    `json:"start_date"`
	EndDate         string    `json:"end_date"`
	DurationDays    int       `json:"duration_days"`
	ProgressPct     float64   `json:"progress_pct"`     // 0.0 ~ 100.0
	BaselineStartDate    string    `json:"baseline_start_date"`
	BaselineEndDate      string    `json:"baseline_end_date"`
	BaselineDurationDays int       `json:"baseline_duration_days"`
	BaselineProgressPct  float64   `json:"baseline_progress_pct"`
	ActualStart     string    `json:"actual_start"`
	ActualEnd       string    `json:"actual_end"`
	ManualScheduled bool      `json:"manual_scheduled"` // false=自动排程 true=手动锁定
	ConstraintType  string    `json:"constraint_type"`  // '' | start_no_earlier_than | finish_no_later_than
	ConstraintDate  string    `json:"constraint_date"`  // YYYY-MM-DD 约束日期
	SortOrder       int       `json:"sort_order"`
	Version         int       `json:"version"`          // 乐观锁版本号
	DeletedAt       *string   `json:"deleted_at,omitempty"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// Dependency 任务依赖关系
type Dependency struct {
	ID             int64     `json:"id"`
	PredecessorID  int64     `json:"predecessor_id"`
	SuccessorID    int64     `json:"successor_id"`
	DepType        string    `json:"dep_type"` // FS | SS | FF | SF
	LagDays        int       `json:"lag_days"` // 延迟天数（可为负=lead）
	CreatedAt   string `json:"created_at"`
}

// ProjectMember 项目成员
type ProjectMember struct {
	ProjectID int64  `json:"project_id"`
	UserID    int64  `json:"user_id"`
	Role      string `json:"role"` // owner | editor | viewer
}

// ActivityLog 操作日志
type ActivityLog struct {
	ID          int64     `json:"id"`
	ProjectID   int64     `json:"project_id"`
	TaskID      *int64    `json:"task_id"`
	UserID      int64     `json:"user_id"`
	Action      string    `json:"action"`
	DetailsJSON string    `json:"details_json"`
	CreatedAt   string `json:"created_at"`
}

// --- API 请求/响应 DTO ---

// LoginRequest 登录请求
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	Token              string `json:"token"`
	User               User   `json:"user"`
	MustChangePassword bool   `json:"must_change_password"`
}

// APIResponse 统一响应信封
type APIResponse struct {
	Data  interface{} `json:"data,omitempty"`
	Error *APIError   `json:"error,omitempty"`
}

// APIError 错误响应
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ChangePasswordRequest 修改密码请求
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}
