package auth

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"followitup/internal/models"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// Service 认证服务
type Service struct {
	db          *sql.DB
	jwtSecret   []byte
	sessionHours int
	bcryptCost  int
}

// NewService 创建认证服务实例
func NewService(db *sql.DB, jwtSecret string, sessionHours int, bcryptCost int) *Service {
	return &Service{
		db:           db,
		jwtSecret:    []byte(jwtSecret),
		sessionHours:  sessionHours,
		bcryptCost:   bcryptCost,
	}
}

// InitAdmin 首次运行时创建管理员账号
func (s *Service) InitAdmin(email, password, displayName string) error {
	// 检查是否已存在用户
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		return fmt.Errorf("查询用户数失败: %w", err)
	}
	if count > 0 {
		return nil // 已有用户，跳过初始化
	}

	return s.CreateUser(email, password, displayName, "local", true)
}

// CreateUser 创建用户
func (s *Service) CreateUser(email, password, displayName, authSource string, isAdmin bool) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), s.bcryptCost)
	if err != nil {
		return fmt.Errorf("密码哈希失败: %w", err)
	}

	login := email // 邮箱即登录名
	_, err = s.db.Exec(
		`INSERT INTO users (login, email, display_name, password_hash, auth_source, is_admin, must_change_password)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		login, email, displayName, string(hash), authSource, boolToInt(isAdmin), 0,
	)
	if err != nil {
		return fmt.Errorf("创建用户失败: %w", err)
	}
	return nil
}

// Login 验证用户凭据并返回 JWT
func (s *Service) Login(email, password string) (*models.LoginResponse, error) {
	// 查找用户
	var u models.User
	var passwordHash string
	var lockedUntil sql.NullString
	var failedAttempts int
	var isAdmin int
	var isActive int
	err := s.db.QueryRow(
		`SELECT id, login, email, display_name, auth_source, is_admin, is_active,
		        password_hash, failed_attempts, locked_until
		 FROM users WHERE email = ? AND is_active = 1`,
		email,
	).Scan(&u.ID, &u.Login, &u.Email, &u.DisplayName, &u.AuthSource,
		&isAdmin, &isActive, &passwordHash, &failedAttempts, &lockedUntil)
	u.IsAdmin = isAdmin != 0
	u.IsActive = isActive != 0

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("用户名或密码错误")
		}
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}

	// 检查是否被锁定
	if lockedUntil.Valid && lockedUntil.String != "" {
		lockTime, err := time.Parse("2006-01-02T15:04:05Z07:00", lockedUntil.String)
		if err == nil && time.Now().Before(lockTime) {
			return nil, errors.New("用户名或密码错误")
		}
	}

	// 根据认证来源验证密码
	if u.AuthSource == "local" {
		if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)); err != nil {
			s.incrementFailedAttempts(u.ID)
			return nil, errors.New("用户名或密码错误")
		}
	} else {
		// LDAP 用户暂不支持，因为 MVP 阶段不启用 LDAP
		return nil, errors.New("用户名或密码错误")
	}

	// 登录成功，重置失败计数
	s.db.Exec("UPDATE users SET failed_attempts = 0, locked_until = NULL WHERE id = ?", u.ID)

	// 签发 JWT
	token, err := s.GenerateToken(&u)
	if err != nil {
		return nil, fmt.Errorf("生成 token 失败: %w", err)
	}

	return &models.LoginResponse{Token: token, User: u}, nil
}

// GenerateToken 为用户生成 JWT
func (s *Service) GenerateToken(user *models.User) (string, error) {
	claims := jwt.MapClaims{
		"user_id": user.ID,
		"email":   user.Email,
		"is_admin": user.IsAdmin,
		"exp":     time.Now().Add(time.Duration(s.sessionHours) * time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

// GetUserByID 根据 ID 查询用户
func (s *Service) GetUserByID(userID int64) (*models.User, error) {
	var u models.User
	var isAdmin, isActive int
	err := s.db.QueryRow(
		`SELECT id, login, email, display_name, auth_source, is_admin, is_active
		 FROM users WHERE id = ? AND is_active = 1`,
		userID,
	).Scan(&u.ID, &u.Login, &u.Email, &u.DisplayName, &u.AuthSource, &isAdmin, &isActive)
	if err != nil {
		return nil, err
	}
	u.IsAdmin = isAdmin != 0
	u.IsActive = isActive != 0
	return &u, nil
}

// ChangePassword 修改密码
func (s *Service) ChangePassword(userID int64, oldPassword, newPassword string) error {
	if len(newPassword) < 8 {
		return errors.New("新密码长度不能少于 8 位")
	}

	var passwordHash string
	err := s.db.QueryRow("SELECT password_hash FROM users WHERE id = ?", userID).Scan(&passwordHash)
	if err != nil {
		return errors.New("用户不存在")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(oldPassword)); err != nil {
		return errors.New("原密码错误")
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), s.bcryptCost)
	if err != nil {
		return fmt.Errorf("密码哈希失败: %w", err)
	}

	_, err = s.db.Exec("UPDATE users SET password_hash = ?, must_change_password = 0 WHERE id = ?",
		string(newHash), userID)
	return err
}

// incrementFailedAttempts 增加失败计数，超过阈值则锁定
func (s *Service) incrementFailedAttempts(userID int64) {
	var attempts int
	s.db.QueryRow("SELECT failed_attempts FROM users WHERE id = ?", userID).Scan(&attempts)
	attempts++

	if attempts >= 5 {
		lockedUntil := time.Now().Add(15 * time.Minute).Format(time.RFC3339)
		s.db.Exec("UPDATE users SET failed_attempts = ?, locked_until = ? WHERE id = ?",
			attempts, lockedUntil, userID)
	} else {
		s.db.Exec("UPDATE users SET failed_attempts = ? WHERE id = ?", attempts, userID)
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
