package telegram

import (
	"context"
	"five-pillars/internal/utils"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"five-pillars/internal/database"
	"five-pillars/internal/services"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Bot struct {
	bot      *tgbotapi.BotAPI
	chatID   int64
	db       *database.Database
	services *services.ServiceManager
	handlers map[string]func(*tgbotapi.Message)
}

func NewBot(token string, chatID int64, db *database.Database, serviceManager *services.ServiceManager) (*Bot, error) {
	botAPI, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания бота: %v", err)
	}

	bot := &Bot{
		bot:      botAPI,
		chatID:   chatID,
		db:       db,
		services: serviceManager,
		handlers: make(map[string]func(*tgbotapi.Message)),
	}

	bot.registerHandlers()
	log.Printf("🤖 Бот инициализирован: %s", botAPI.Self.UserName)
	return bot, nil
}

func (b *Bot) registerHandlers() {
	b.handlers["/start"] = b.handleStart
	b.handlers["/today"] = b.handleToday
	b.handlers["/summary"] = b.handleSummary
	b.handlers["/week"] = b.handleWeek
	b.handlers["/all"] = b.handleAll
	b.handlers["/change"] = b.handleChangeDate
	b.handlers["/feelings"] = b.handleFeelings
	b.handlers["/help"] = b.handleHelp
}

func (b *Bot) SendMessage(text string) error {
	msg := tgbotapi.NewMessage(b.chatID, text)
	msg.ParseMode = "HTML"
	_, err := b.bot.Send(msg)
	return err
}

func (b *Bot) SendTaskNotification(task database.TaskNotification) error {
	pillarName := utils.GetPillarName(task.Pillar)
	pillarEmoji := utils.GetPillarEmoji(task.Pillar)

	// Форматируем время для МСК
	formattedTime := utils.FormatTimeForDisplay(task.TimeUTC)

	message := fmt.Sprintf(
		"🔔 <b>%s %s</b>\n\n"+
			"<i>%s</i>\n\n"+
			"⏰ Время: %s\n"+
			"📝 %s",
		pillarEmoji, pillarName,
		task.Description,
		formattedTime,
		task.Notes,
	)

	b.SendMessage(message)

	// Отправляем кнопки
	msg := tgbotapi.NewMessage(b.chatID, "Выполнено?")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Да", fmt.Sprintf("complete_%d", task.ID)),
			tgbotapi.NewInlineKeyboardButtonData("⏰ Отложить", fmt.Sprintf("snooze_%d", task.ID)),
		),
	)
	msg.ParseMode = "HTML"

	_, err := b.bot.Send(msg)
	return err
}

// SendCombinedMissedNotification отправляет объединенное сообщение о пропущенных задачах
func (b *Bot) SendCombinedMissedNotification(missedTasks []database.TaskNotification) error {
	if len(missedTasks) == 0 {
		return nil
	}

	// Создаем заголовок сообщения
	var message strings.Builder
	message.WriteString(fmt.Sprintf("⏰ <b>ПРОПУЩЕННЫЕ ЗАДАЧИ (%d)</b>\n\n", len(missedTasks)))
	message.WriteString("<i>Найдены задачи, которые должны были быть выполнены ранее:</i>\n\n")

	// Добавляем каждую задачу в список
	for i, task := range missedTasks {
		pillarName := utils.GetPillarName(task.Pillar)
		pillarEmoji := utils.GetPillarEmoji(task.Pillar)

		message.WriteString(fmt.Sprintf(
			"%d. <b>%s %s</b>\n",
			i+1, pillarEmoji, pillarName,
		))
		message.WriteString(fmt.Sprintf(
			"   <i>%s</i>\n",
			task.Description,
		))
		message.WriteString(fmt.Sprintf(
			"   ⏱ Должно было быть: %s UTC\n\n",
			task.TimeUTC,
		))
	}

	message.WriteString("👇 <i>Выполнили какие-то из них?</i>")

	// Отправляем сообщение
	b.SendMessage(message.String())

	// Создаем клавиатуру с кнопками для каждой задачи
	var keyboardRows [][]tgbotapi.InlineKeyboardButton

	// Кнопки для каждой задачи
	for _, task := range missedTasks {
		buttonText := fmt.Sprintf("✅ Выполнена? %s", utils.GetPillarName(task.Pillar))
		callbackData := fmt.Sprintf("missed_complete_%d", task.ID)

		keyboardRows = append(keyboardRows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(buttonText, callbackData),
		))
	}

	// Отправляем клавиатуру
	keyboardMsg := tgbotapi.NewMessage(b.chatID, "Выберите действие:")
	keyboardMsg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboardRows...)
	keyboardMsg.ParseMode = "HTML"

	_, err := b.bot.Send(keyboardMsg)
	return err
}

func (b *Bot) GetUsername() string {
	return b.bot.Self.UserName
}

func (b *Bot) Start(ctx context.Context) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := b.bot.GetUpdatesChan(u)

	for {
		select {
		case <-ctx.Done():
			return
		case update := <-updates:
			b.handleUpdate(update)
		}
	}
}

func (b *Bot) handleUpdate(update tgbotapi.Update) {
	if update.CallbackQuery != nil {
		b.handleCallbackQuery(update.CallbackQuery)
		return
	}

	if update.Message == nil {
		return
	}

	if update.Message.Chat.ID != b.chatID {
		b.SendMessage("⛔ Доступ запрещен")
		return
	}

	text := update.Message.Text
	if text == "" {
		return
	}

	if strings.HasPrefix(text, "/") {
		parts := strings.Fields(text)
		command := parts[0]

		if strings.HasPrefix(text, "/add ") {
			b.handleAddTask(update.Message)
		} else if strings.HasPrefix(text, "/feelings ") {
			b.handleFeelingsCommand(update.Message)
		} else if strings.HasPrefix(text, "/all") {
			b.handleAll(update.Message)
		} else if strings.HasPrefix(text, "/change ") {
			b.handleChangeDate(update.Message)
		} else if handler, exists := b.handlers[command]; exists {
			handler(update.Message)
			return
		} else {
			b.SendMessageOrLogError("❌ Неизвестная команда. Используйте /help")
		}
	}
}

func (b *Bot) handleCallbackQuery(callback *tgbotapi.CallbackQuery) {
	data := callback.Data
	chatID := callback.Message.Chat.ID

	if chatID != b.chatID {
		return
	}

	// Обработка обычных задач
	if strings.HasPrefix(data, "complete_") {
		taskID, _ := strconv.Atoi(strings.TrimPrefix(data, "complete_"))
		b.completeTask(taskID)
	} else if strings.HasPrefix(data, "snooze_") {
		taskID, _ := strconv.Atoi(strings.TrimPrefix(data, "snooze_"))
		b.snoozeTask(taskID)
	} else if strings.HasPrefix(data, "missed_complete_") {
		taskID, _ := strconv.Atoi(strings.TrimPrefix(data, "missed_complete_"))
		b.completeMissedTask(taskID, callback.Message.MessageID)
	}

	callbackConfig := tgbotapi.NewCallback(callback.ID, "✅")
	b.bot.Request(callbackConfig)
}

func (b *Bot) completeTask(taskID int) {
	if err := database.NewRepository(b.db).UpdateTaskCompletion(taskID, true); err != nil {
		b.SendMessage("❌ Ошибка обновления задачи")
		return
	}
	b.SendMessage("✅ Задача выполнена!")
}

func (b *Bot) snoozeTask(taskID int) {
	// Получаем текущее время задачи
	var currentTime string
	err := b.db.GetDB().QueryRow("SELECT time_utc FROM tasks WHERE id = ?", taskID).Scan(&currentTime)
	if err != nil {
		b.SendMessage("❌ Ошибка получения времени задачи")
		return
	}

	// Парсим время и добавляем 15 минут
	t, err := time.Parse("15:04", currentTime)
	if err != nil {
		b.SendMessage("❌ Ошибка парсинга времени")
		return
	}

	newTime := t.Add(60 * time.Minute).Format("15:04")

	// Обновляем время
	_, err = b.db.GetDB().Exec("UPDATE tasks SET time_utc = ? WHERE id = ?", newTime, taskID)
	if err != nil {
		b.SendMessage("❌ Ошибка откладывания задачи")
		return
	}

	b.SendMessage(fmt.Sprintf("⏰ Задача отложена до %s UTC", newTime))
}

func (b *Bot) completeMissedTask(taskID int, messageID int) {
	if err := database.NewRepository(b.db).UpdateTaskCompletion(taskID, true); err != nil {
		b.SendMessage("❌ Ошибка обновления задачи")
		return
	}

	// Удаляем сообщение с кнопками
	deleteMsg := tgbotapi.NewDeleteMessage(b.chatID, messageID)
	b.bot.Send(deleteMsg)

	b.SendMessage("✅ Задача отмечена выполненной!")
}
