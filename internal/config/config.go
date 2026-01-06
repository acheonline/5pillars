package config

import (
	"log"
	"os"
	"strconv"
)

type Config struct {
	Telegram struct {
		Token  string `yaml:"token"`
		ChatID int64  `yaml:"chat_id"`
	} `yaml:"telegram"`
	Server struct {
		Port string `yaml:"port"`
	} `yaml:"server"`
	Database struct {
		Path string `yaml:"path"`
	} `yaml:"database"`
}

func Load() (*Config, error) {

	token := getEnv("TG_TOKEN", "")
	if token == "" {
		log.Fatal("❌ TG_TOKEN не установлен. Установите переменную окружения или создайте .env файл")
	}

	chatIDStr := getEnv("TG_CHAT_ID", "")
	if chatIDStr == "" {
		log.Fatal("❌ TG_CHAT_ID не установлен")
	}

	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		log.Fatalf("❌ Неверный TG_CHAT_ID: %v", err)
	}

	cfg := &Config{}
	cfg.Telegram.Token = token
	cfg.Telegram.ChatID = chatID
	cfg.Server.Port = getEnv("PORT", "8080")
	cfg.Database.Path = getEnv("DB_PATH", getEnv("DB_PATH", "/data/five-pillars.db"))

	log.Printf("✅ Конфигурация загружена: порт=%s, БД=%s", cfg.Server.Port, cfg.Database.Path)

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
