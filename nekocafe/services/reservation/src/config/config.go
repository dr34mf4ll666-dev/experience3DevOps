package config

import (
	"fmt"
	"os"
)

// Config 集中所有运行期配置。
// 严禁在代码中硬编码任何密钥；本地用 .env，CI/CD 用 GitHub Secrets，
// K8s 中通过 ExternalSecret → Vault 注入。
type Config struct {
	ServiceName      string
	DBDsn            string
	RedisAddr        string
	MemberServiceURL string
	OTLPEndpoint     string
	LogLevel         string
}

// Load 读取环境变量并做最小校验。
func Load() (*Config, error) {
	cfg := &Config{
		ServiceName:      env("SERVICE_NAME", "reservation"),
		DBDsn:            os.Getenv("DB_DSN"),
		RedisAddr:        env("REDIS_ADDR", "localhost:6379"),
		MemberServiceURL: env("MEMBER_SERVICE_URL", "http://member:8080"),
		OTLPEndpoint:     env("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
		LogLevel:         env("LOG_LEVEL", "info"),
	}
	if cfg.DBDsn == "" {
		return nil, fmt.Errorf("DB_DSN 不能为空")
	}
	return cfg, nil
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
