package app

import (
	"os"
	"strconv"
)

// このファイルは、実行環境の環境変数を Config へ集約し、型付きの設定として提供する。
// 起動処理や handler が環境変数を直接参照せず、環境差分と既定値を一か所で管理するために分離している。

type Config struct {
	AppEnv              string
	DatabaseURL         string
	FrontendOrigin      string
	SessionStore        string
	SessionCookieSecure bool
	RedisAddr           string
	RedisPassword       string
	RedisDB             int
}

func LoadConfig() Config {
	return Config{
		AppEnv:              envOrDefault("APP_ENV", "local"),
		DatabaseURL:         os.Getenv("DATABASE_URL"),
		FrontendOrigin:      envOrDefault("FRONTEND_ORIGIN", "http://localhost:30101"),
		SessionStore:        envOrDefault("SESSION_STORE", "postgres"),
		SessionCookieSecure: envBool("SESSION_COOKIE_SECURE", false),
		RedisAddr:           envOrDefault("REDIS_ADDR", "localhost:6379"),
		RedisPassword:       os.Getenv("REDIS_PASSWORD"),
		RedisDB:             envInt("REDIS_DB", 0),
	}
}

func envOrDefault(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func envBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
