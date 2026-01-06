package services

import (
	"fmt"
	"log"
	"time"

	"five-pillars/internal/database"
)

// NotificationSender интерфейс для отправки уведомлений
type NotificationSender interface {
	SendMessage(text string) error
	SendTaskNotification(task database.TaskNotification) error
	SendCombinedMissedNotification(missedTasks []database.TaskNotification) error
}

type NotificationService struct {
	sender     NotificationSender
	repository *database.Repository
}

func NewNotificationService(sender NotificationSender, repo *database.Repository) *NotificationService {
	return &NotificationService{
		sender:     sender,
		repository: repo,
	}
}

func (ns *NotificationService) CheckAndSendNotifications() {
	now := time.Now().UTC()
	currentTime := now.Format("15:04")
	today := now.Format("2006-01-02")

	log.Printf("🔔 Проверка уведомлений: %s %s", today, currentTime)

	// 1. Проверяем задачи на текущее время
	tasks, err := ns.repository.GetTasksForNotification(currentTime, today)
	if err != nil {
		log.Printf("⚠️ Ошибка получения задач: %v", err)
		return
	}

	log.Printf("📋 Найдено задач для текущего времени: %d", len(tasks))

	for _, task := range tasks {
		log.Printf("📨 Отправляю уведомление: %s - %s", task.Pillar, task.Description)

		if err := ns.sender.SendTaskNotification(task); err != nil {
			log.Printf("❌ Ошибка отправки: %v", err)
		} else {
			log.Printf("✅ Уведомление отправлено: ID=%d", task.ID)
		}
	}
}

// SendMissedNotifications отправляет ОБЪЕДИНЕННОЕ сообщение о пропущенных задачах
func (ns *NotificationService) SendMissedNotifications() {
	now := time.Now().UTC()
	currentTime := now.Format("15:04")
	today := now.Format("2006-01-02")

	log.Printf("⏰ Проверка пропущенных уведомлений за %s (текущее время: %s)", today, currentTime)

	tasks, err := ns.repository.GetMissedTasks(today, currentTime)
	if err != nil {
		log.Printf("⚠️ Ошибка получения пропущенных задач: %v", err)
		return
	}

	if len(tasks) == 0 {
		log.Println("✅ Пропущенных уведомлений нет")
		return
	}

	log.Printf("📨 Найдено пропущенных задач: %d", len(tasks))

	// Отправляем ОДНО объединенное сообщение вместо нескольких
	if len(tasks) > 0 {
		if err := ns.sender.SendCombinedMissedNotification(tasks); err != nil {
			log.Printf("❌ Ошибка отправки объединенного уведомления: %v", err)
			// Fallback: отправляем обычные уведомления (старый способ)
			for _, task := range tasks {
				err := ns.sender.SendTaskNotification(task)
				if err != nil {
					log.Fatal(err)
				}
			}
		}
	}
}

// SendDailySummary отправляет итоги дня
func (ns *NotificationService) SendDailySummary() {
	today := time.Now().UTC().Format("2006-01-02")
	summary, err := ns.repository.GetDailySummary(today)
	if err != nil {
		log.Printf("⚠️ Ошибка получения сводки дня: %v", err)
		return
	}

	completed := summary["completed"].(int)
	total := summary["total"].(int)
	percentage := summary["percentage"].(float64)

	message := fmt.Sprintf(
		"📊 <b>Итоги дня %s</b>\n\n"+
			"✅ Выполнено: %d/%d (%.0f%%)\n\n"+
			"Завтра будет новый день! 🌅",
		today,
		completed,
		total,
		percentage,
	)

	ns.sender.SendMessage(message)
}
