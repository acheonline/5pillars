package services

import (
	"five-pillars/internal/utils"
	"fmt"
	"log"
	"strings"
	"time"

	"five-pillars/internal/database"
)

// NotificationSender интерфейс для отправки уведомлений
type NotificationSender interface {
	SendMessage(text string) error
	SendTaskNotification(task database.TaskNotification) error
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

// SendAllTodayTaskNotification отправляет текущий статус по задачам
func (ns *NotificationService) SendAllTodayTaskNotification() {
	today := time.Now().UTC().Format("2006-01-02")
	tasks, err := ns.repository.GetTasksByDate(today)
	if err != nil {
		log.Printf("⚠️ Ошибка получения сводки дня: %v", err)
		return
	}

	if len(tasks) == 0 {
		ns.sender.SendMessage("📭 На сегодня задач нет")
		return
	}

	var message strings.Builder
	message.WriteString(fmt.Sprintf("📅 <b>!НАПОМИНАНИЕ-СВОДКА на %s</b>\n\n", utils.GetCurrentMSKDate()))
	message.WriteString(utils.GetTimezoneInfo() + "\n\n")

	for _, task := range tasks {
		pillarName := utils.GetPillarName(string(task.Pillar))
		displayTime := utils.FormatTimeForDisplay(task.TimeUTC)

		var status string
		if task.Completed {
			status = "✅"
		} else if task.Skipped {
			status = "➖"
		} else {
			taskTime, _ := time.Parse("15:04", task.TimeUTC)
			currentUTC := time.Now().UTC()
			taskUTC := time.Date(currentUTC.Year(), currentUTC.Month(), currentUTC.Day(),
				taskTime.Hour(), taskTime.Minute(), 0, 0, time.UTC)

			status = "⬜"
			if currentUTC.After(taskUTC) {
				status = "⏰"
			}
		}

		message.WriteString(fmt.Sprintf(
			"%s <b>%s</b>\n"+
				"%s\n"+
				"<i>%s</i>\n\n",
			status, pillarName,
			displayTime, task.Description,
		))

		if task.Skipped && task.Notes != "" && strings.Contains(task.Notes, "Пропущено:") {
			parts := strings.SplitN(task.Notes, "|", 2)
			if len(parts) > 1 {
				message.WriteString(fmt.Sprintf("📝 <i>%s</i>\n\n", strings.TrimSpace(parts[1])))
			}
		}
	}

	err = ns.sender.SendMessage(message.String())
	if err != nil {
		log.Printf("❌ Ошибка отправки уведомления: %v", err)
	}
}
