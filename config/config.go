package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"

	"gopkg.in/yaml.v3"
)

const (
	DefaultMySQLHost       = "localhost"
	DefaultMySQLPort       = 3306
	DefaultMySQLUser       = "root"
	DefaultMySQLPassword   = "root"
	DefaultMySQLDatabase   = "xiaoquan"
	DefaultMySQLCharset    = "utf8mb4"
	DefaultMaxOpenConns    = 30
	DefaultMaxIdleConns    = 10
	DefaultConnMaxLifetime = 3600
	DefaultConnMaxIdleTime = 1800

	DefaultServerPort    = 8080
	DefaultUploadDir     = "./uploads"
	DefaultThumbnailDir  = "./thumbnails"
	DefaultMaxUploadSize = 1073741824

	DefaultRedisHost     = "localhost"
	DefaultRedisPort     = 6379
	DefaultRedisPassword = ""
	DefaultRedisDB       = 0
	DefaultRedisTimeout  = 5
	DefaultRedisPrefix   = "xiaoquan:"

	DefaultSpamAPIURL = ""
	DefaultBotToken   = ""
)

type Config struct {
	MySQL             MySQLConfig     `yaml:"mysql"`
	Server            ServerConfig    `yaml:"server"`
	SpamAPI           SpamAPIConfig   `yaml:"spam_api"`
	Redis             RedisConfig     `yaml:"redis"`
	Bot               BotConfig       `yaml:"bot"`
	Recommend         RecommendConfig `yaml:"recommend"`
	MigrateThumbnails bool            `yaml:"migrate_thumbnails"`
	InitAdmin         bool            `yaml:"init_admin"`
	TinifyAPIKey      string          `yaml:"tinify_api_key"`
	TinifyEnabled     bool            `yaml:"tinify_enabled"`
}

type RecommendConfig struct {
	ExcludedTags []string `yaml:"excluded_tags"`
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

type BotConfig struct {
	Token string `yaml:"token"`
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
	Port           int      `yaml:"port"`
	UploadDir      string   `yaml:"upload_dir"`
	ThumbnailDir   string   `yaml:"thumbnail_dir"`
	MaxUploadSize  int64    `yaml:"max_upload_size"`
	AllowedOrigins []string `yaml:"allowed_origins"`
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

	// 如果配置文件中不存在 migrate_thumbnails 字段，默认启用迁移
	var raw map[string]interface{}
	if yaml.Unmarshal(data, &raw) == nil {
		if _, ok := raw["migrate_thumbnails"]; !ok {
			AppConfig.MigrateThumbnails = true
		}
	}

	ApplyDefaults(AppConfig)

	if err := syncConfigFile(configPath, data); err != nil {
		log.Printf("[配置] 同步配置文件失败: %v", err)
	}

	if err := EnsureDirectories(); err != nil {
		return fmt.Errorf("failed to ensure directories: %w", err)
	}

	log.Printf("[配置] 配置文件加载成功: %s", configPath)
	return nil
}

func ApplyDefaults(cfg *Config) {
	defaultCfg := GetDefaultConfig()
	dv := reflect.ValueOf(cfg).Elem()
	sv := reflect.ValueOf(defaultCfg).Elem()
	mergeDefaults(dv, sv, "")
}

func mergeDefaults(dst, src reflect.Value, prefix string) {
	t := dst.Type()
	for i := 0; i < dst.NumField(); i++ {
		df := dst.Field(i)
		sf := src.Field(i)
		if !df.CanSet() {
			continue
		}
		fieldName := t.Field(i).Name
		path := fieldName
		if prefix != "" {
			path = prefix + "." + fieldName
		}

		switch df.Kind() {
		case reflect.String:
			setDefaultString(df, sf, path)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			setDefaultInt(df, sf, path)
		case reflect.Slice:
			setDefaultSlice(df, sf, path)
		case reflect.Struct:
			mergeDefaults(df, sf, path)
		}
	}
}

func setDefaultString(dst, src reflect.Value, path string) {
	if dst.String() == "" && src.String() != "" {
		dst.Set(src)
		log.Printf("[配置] %s 未配置，使用默认值: %s", path, src.String())
	}
}

func setDefaultInt(dst, src reflect.Value, path string) {
	if dst.Int() == 0 && src.Int() != 0 {
		dst.Set(src)
		log.Printf("[配置] %s 未配置，使用默认值: %d", path, src.Int())
	}
}

func setDefaultSlice(dst, src reflect.Value, path string) {
	if dst.IsNil() && !src.IsNil() {
		dst.Set(src)
		log.Printf("[配置] %s 未配置，使用默认值", path)
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
			Port:           DefaultServerPort,
			UploadDir:      DefaultUploadDir,
			ThumbnailDir:   DefaultThumbnailDir,
			MaxUploadSize:  DefaultMaxUploadSize,
			AllowedOrigins: []string{"http://localhost:8080", "http://localhost:3000"},
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
		Bot: BotConfig{
			Token: DefaultBotToken,
		},
		Recommend: RecommendConfig{
			ExcludedTags: []string{"bot"},
		},
		MigrateThumbnails: true,
		InitAdmin:         false,
		TinifyAPIKey:      "",
		TinifyEnabled:     false,
	}
}

func EnsureDirectories() error {
	dirs := []string{
		AppConfig.Server.UploadDir,
		AppConfig.Server.ThumbnailDir,
		"./images",
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

func SaveConfig(configPath string) error {
	data, err := yaml.Marshal(AppConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func syncConfigFile(configPath string, userData []byte) error {
	defaultCfg := GetDefaultConfig()
	defaultData, err := yaml.Marshal(defaultCfg)
	if err != nil {
		return fmt.Errorf("生成默认配置失败: %w", err)
	}

	var userMap, defaultMap map[string]interface{}
	if err := yaml.Unmarshal(userData, &userMap); err != nil {
		return fmt.Errorf("解析用户配置失败: %w", err)
	}
	if err := yaml.Unmarshal(defaultData, &defaultMap); err != nil {
		return fmt.Errorf("解析默认配置失败: %w", err)
	}

	mergeMap(defaultMap, userMap)

	merged, err := yaml.Marshal(defaultMap)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	if err := os.WriteFile(configPath, merged, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}
	return nil
}

func mergeMap(dst, src map[string]interface{}) {
	for k, srcVal := range src {
		if dstVal, ok := dst[k]; ok {
			dstMap, dstOK := dstVal.(map[string]interface{})
			srcMap, srcOK := srcVal.(map[string]interface{})
			if dstOK && srcOK {
				mergeMap(dstMap, srcMap)
				continue
			}
		}
		dst[k] = srcVal
	}
}

func (m *MySQLConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
		m.User, m.Password, m.Host, m.Port, m.Database, m.Charset)
}
