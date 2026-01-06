package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"five-pillars/internal/app"
	"five-pillars/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("❌ Ошибка загрузки конфигурации: %v", err)
	}

	application, err := app.New(cfg)
	if err != nil {
		log.Fatalf("❌ Ошибка создания приложения: %v", err)
	}

	if err := application.Start(); err != nil {
		log.Fatalf("❌ Ошибка запуска приложения: %v", err)
	}
	defer application.Stop()

	waitForShutdown()
	log.Println("👋 Приложение завершает работу")
}

func waitForShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
}
