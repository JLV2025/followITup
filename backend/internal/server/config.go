package server

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config 应用配置
type Config struct {
	Server ServerConfig  `yaml:"server"`
	Auth   AuthConfig    `yaml:"auth"`
	LDAP   LDAPConfig    `yaml:"ldap"`
	Fiscal FiscalConfig  `yaml:"fiscal"`
}

// FiscalConfig 财年配置
type FiscalConfig struct {
	YearStartMonth int `yaml:"year_start_month"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Port    int    `yaml:"port"`
	DataDir string `yaml:"data_dir"`
}

// AuthConfig 认证配置
type AuthConfig struct {
	JWTSecret   string `yaml:"jwt_secret"`
	SessionHours int   `yaml:"session_hours"`
	BcryptCost  int    `yaml:"bcrypt_cost"`
}

// LDAPConfig LDAP 配置
type LDAPConfig struct {
	Enabled      bool     `yaml:"enabled"`
	Host         string   `yaml:"host"`
	Port         int      `yaml:"port"`
	BindDN       string   `yaml:"bind_dn"`
	BindPassword string   `yaml:"bind_password"`
	BaseDN       string   `yaml:"base_dn"`
	UserFilter   string   `yaml:"user_filter"`
	Attributes   []string `yaml:"attributes"`
}

// LoadConfig 从 YAML 文件加载配置
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	cfg := &Config{}
	// 设置默认值
	cfg.Server.Port = 8080
	cfg.Server.DataDir = "./data"
	cfg.Auth.SessionHours = 8
	cfg.Auth.BcryptCost = 12
	cfg.Fiscal.YearStartMonth = 4

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 确保数据目录存在
	if err := os.MkdirAll(cfg.Server.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}

	return cfg, nil
}
