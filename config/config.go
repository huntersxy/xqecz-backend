package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	DefaultMySQLHost     = "localhost"
	DefaultMySQLPort     = 3306
	DefaultMySQLUser     = "root"
	DefaultMySQLPassword = "root"
	DefaultMySQLDatabase = "xiaoquan"
	DefaultMySQLCharset  = "utf8mb4"
	DefaultMaxOpenConns  = 30
	DefaultMaxIdleConns  = 10
	DefaultConnMaxLifetime = 3600
	DefaultConnMaxIdleTime = 1800

	DefaultServerPort     = 8080
	DefaultUploadDir     = "./uploads"
	DefaultThumbnailDir  = "./thumbnails"
	DefaultMaxUploadSize = 1073741824

	DefaultRedisHost    = "localhost"
	DefaultRedisPort    = 6379
	DefaultRedisPassword = ""
	DefaultRedisDB      = 0
	DefaultRedisTimeout = 5
	DefaultRedisPrefix  = "xiaoquan:"

	DefaultSpamAPIURL = ""
)

type Config struct {
	MySQL   MySQLConfig   `yaml:"mysql"`
	Server  ServerConfig  `yaml:"server"`
	SpamAPI SpamAPIConfig `yaml:"spam_api"`
	Redis   RedisConfig    `yaml:"redis"`
}

type SpamAPIConfig struct {
	URL string `yaml:"url"`
}

type RedisConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
	Timeout  int    `yaml:"timeout"`
	Prefix   string `yaml:"prefix"`
}

type MySQLConfig struct {
	Host            string `yaml:"host"`
	Port            int    `yaml:"port"`
	User            string `yaml:"user"`
	Password        string `yaml:"password"`
	Database        string `yaml:"database"`
	Charset         string `yaml:"charset"`
	MaxOpenConns    int    `yaml:"max_open_conns"`
	MaxIdleConns    int    `yaml:"max_idle_conns"`
	ConnMaxLifetime int    `yaml:"conn_max_lifetime"`
	ConnMaxIdleTime int    `yaml:"conn_max_idle_time"`
}

type ServerConfig struct {
	Port          int    `yaml:"port"`
	UploadDir     string `yaml:"upload_dir"`
	ThumbnailDir  string `yaml:"thumbnail_dir"`
	MaxUploadSize int64  `yaml:"max_upload_size"`
}

var AppConfig *Config

func LoadConfig(configPath string) error {
	if err := EnsureConfigFile(configPath); err != nil {
		return fmt.Errorf("failed to ensure config file: %w", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	AppConfig = &Config{}
	if err := yaml.Unmarshal(data, AppConfig); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	ApplyDefaults(AppConfig)

	if err := EnsureDirectories(); err != nil {
		return fmt.Errorf("failed to ensure directories: %w", err)
	}

	log.Printf("[配置] 配置文件加载成功: %s", configPath)
	return nil
}

func ApplyDefaults(cfg *Config) {
	if cfg.MySQL.Host == "" {
		cfg.MySQL.Host = DefaultMySQLHost
		log.Printf("[配置] MySQL主机未配置，使用默认值: %s", DefaultMySQLHost)
	}
	if cfg.MySQL.Port == 0 {
		cfg.MySQL.Port = DefaultMySQLPort
		log.Printf("[配置] MySQL端口未配置，使用默认值: %d", DefaultMySQLPort)
	}
	if cfg.MySQL.User == "" {
		cfg.MySQL.User = DefaultMySQLUser
		log.Printf("[配置] MySQL用户未配置，使用默认值: %s", DefaultMySQLUser)
	}
	if cfg.MySQL.Password == "" {
		cfg.MySQL.Password = DefaultMySQLPassword
		log.Printf("[配置] MySQL密码未配置，使用默认值: %s", DefaultMySQLPassword)
	}
	if cfg.MySQL.Database == "" {
		cfg.MySQL.Database = DefaultMySQLDatabase
		log.Printf("[配置] MySQL数据库未配置，使用默认值: %s", DefaultMySQLDatabase)
	}
	if cfg.MySQL.Charset == "" {
		cfg.MySQL.Charset = DefaultMySQLCharset
		log.Printf("[配置] MySQL字符集未配置，使用默认值: %s", DefaultMySQLCharset)
	}
	if cfg.MySQL.MaxOpenConns == 0 {
		cfg.MySQL.MaxOpenConns = DefaultMaxOpenConns
		log.Printf("[配置] MySQL最大打开连接数未配置，使用默认值: %d", DefaultMaxOpenConns)
	}
	if cfg.MySQL.MaxIdleConns == 0 {
		cfg.MySQL.MaxIdleConns = DefaultMaxIdleConns
		log.Printf("[配置] MySQL最大空闲连接数未配置，使用默认值: %d", DefaultMaxIdleConns)
	}
	if cfg.MySQL.ConnMaxLifetime == 0 {
		cfg.MySQL.ConnMaxLifetime = DefaultConnMaxLifetime
		log.Printf("[配置] MySQL连接最大生命周期未配置，使用默认值: %d秒", DefaultConnMaxLifetime)
	}
	if cfg.MySQL.ConnMaxIdleTime == 0 {
		cfg.MySQL.ConnMaxIdleTime = DefaultConnMaxIdleTime
		log.Printf("[配置] MySQL连接最大空闲时间未配置，使用默认值: %d秒", DefaultConnMaxIdleTime)
	}

	if cfg.Server.Port == 0 {
		cfg.Server.Port = DefaultServerPort
		log.Printf("[配置] 服务器端口未配置，使用默认值: %d", DefaultServerPort)
	}
	if cfg.Server.UploadDir == "" {
		cfg.Server.UploadDir = DefaultUploadDir
		log.Printf("[配置] 上传目录未配置，使用默认值: %s", DefaultUploadDir)
	}
	if cfg.Server.ThumbnailDir == "" {
		cfg.Server.ThumbnailDir = DefaultThumbnailDir
		log.Printf("[配置] 缩略图目录未配置，使用默认值: %s", DefaultThumbnailDir)
	}
	if cfg.Server.MaxUploadSize == 0 {
		cfg.Server.MaxUploadSize = DefaultMaxUploadSize
		log.Printf("[配置] 最大上传大小未配置，使用默认值: %d字节(1GB)", DefaultMaxUploadSize)
	}

	if cfg.Redis.Host == "" {
		cfg.Redis.Host = DefaultRedisHost
		log.Printf("[配置] Redis主机未配置，使用默认值: %s", DefaultRedisHost)
	}
	if cfg.Redis.Port == 0 {
		cfg.Redis.Port = DefaultRedisPort
		log.Printf("[配置] Redis端口未配置，使用默认值: %d", DefaultRedisPort)
	}
	if cfg.Redis.Timeout == 0 {
		cfg.Redis.Timeout = DefaultRedisTimeout
		log.Printf("[配置] Redis超时未配置，使用默认值: %d秒", DefaultRedisTimeout)
	}
	if cfg.Redis.Prefix == "" {
		cfg.Redis.Prefix = DefaultRedisPrefix
		log.Printf("[配置] Redis前缀未配置，使用默认值: %s", DefaultRedisPrefix)
	}
}

func EnsureConfigFile(configPath string) error {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Printf("[配置] 配置文件不存在，正在创建: %s", configPath)

		dir := filepath.Dir(configPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}

		defaultConfig := GetDefaultConfig()
		data, err := yaml.Marshal(defaultConfig)
		if err != nil {
			return fmt.Errorf("failed to marshal default config: %w", err)
		}

		if err := os.WriteFile(configPath, data, 0644); err != nil {
			return fmt.Errorf("failed to write default config file: %w", err)
		}

		log.Printf("[配置] 默认配置文件已创建: %s", configPath)
		log.Printf("[配置] 请编辑配置文件后重新启动程序")
	}
	return nil
}

func GetDefaultConfig() *Config {
	return &Config{
		MySQL: MySQLConfig{
			Host:            DefaultMySQLHost,
			Port:            DefaultMySQLPort,
			User:            DefaultMySQLUser,
			Password:        DefaultMySQLPassword,
			Database:        DefaultMySQLDatabase,
			Charset:         DefaultMySQLCharset,
			MaxOpenConns:    DefaultMaxOpenConns,
			MaxIdleConns:    DefaultMaxIdleConns,
			ConnMaxLifetime: DefaultConnMaxLifetime,
			ConnMaxIdleTime: DefaultConnMaxIdleTime,
		},
		Server: ServerConfig{
			Port:          DefaultServerPort,
			UploadDir:     DefaultUploadDir,
			ThumbnailDir:  DefaultThumbnailDir,
			MaxUploadSize: DefaultMaxUploadSize,
		},
		SpamAPI: SpamAPIConfig{
			URL: DefaultSpamAPIURL,
		},
		Redis: RedisConfig{
			Host:     DefaultRedisHost,
			Port:     DefaultRedisPort,
			Password: DefaultRedisPassword,
			DB:       DefaultRedisDB,
			Timeout:  DefaultRedisTimeout,
			Prefix:   DefaultRedisPrefix,
		},
	}
}

func EnsureDirectories() error {
	dirs := []string{
		AppConfig.Server.UploadDir,
		AppConfig.Server.ThumbnailDir,
	}

	for _, dir := range dirs {
		if dir == "" {
			continue
		}

		absDir, err := filepath.Abs(dir)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for %s: %w", dir, err)
		}

		if _, err := os.Stat(absDir); os.IsNotExist(err) {
			log.Printf("[目录] 目录不存在，正在创建: %s", absDir)
			if err := os.MkdirAll(absDir, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", absDir, err)
			}
			log.Printf("[目录] 目录创建成功: %s", absDir)
		}
	}

	return nil
}

func (m *MySQLConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
		m.User, m.Password, m.Host, m.Port, m.Database, m.Charset)
}
