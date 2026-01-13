package telegram

import (
	"context"
	"encoding/json"
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
	bot         *tgbotapi.BotAPI
	chatID      int64
	db          *database.Database
	services    *services.ServiceManager
	handlers    map[string]func(*tgbotapi.Message)
	skipReasons map[string]string
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
		skipReasons: map[string]string{
			"no_energy":  "🔋 Не было энергии",
			"no_time":    "⏰ Не хватило времени",
			"irrelevant": "🎯 Задача неактуальна",
			"illness":    "Болел",
		},
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
	b.handlers["/time"] = b.handleChangeTime
	b.handlers["/date"] = b.handleChangeDate
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

	keyboard := b.createTaskKeyboard(task.ID)
	actionMsg := tgbotapi.NewMessage(b.chatID, "Выполнено?")
	actionMsg.ReplyMarkup = keyboard
	actionMsg.ParseMode = "HTML"

	_, err := b.bot.Send(actionMsg)
	return err
}

// createTaskKeyboard создает клавиатуру для взаимодействия с задачей
func (b *Bot) createTaskKeyboard(taskID int) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Выполнил", fmt.Sprintf("complete_%d", taskID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔄 Отложить", fmt.Sprintf("snooze_%d", taskID)),
			tgbotapi.NewInlineKeyboardButtonData("❌ Закрыть", fmt.Sprintf("skip_%d", taskID)),
		),
	)
}

// createSkipReasonKeyboard создает клавиатуру для выбора причины пропуска
func (b *Bot) createSkipReasonKeyboard(taskID int) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

	for code, text := range b.skipReasons {
		callbackData := fmt.Sprintf("skip_reason_%d_%s", taskID, code)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(text, callbackData),
		))
	}

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// SendCombinedMissedNotification отправляет объединенное сообщение о пропущенных задачах
func (b *Bot) SendCombinedMissedNotification(missedTasks []database.TaskNotification) error {
	if len(missedTasks) == 0 {
		return nil
	}

	var message strings.Builder
	message.WriteString(fmt.Sprintf("⏰ <b>ПРОПУЩЕННЫЕ ЗАДАЧИ (%d)</b>\n\n", len(missedTasks)))
	message.WriteString("<i>Найдены задачи, которые должны были быть выполнены ранее:</i>\n\n")

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

	b.SendMessage(message.String())

	var keyboardRows [][]tgbotapi.InlineKeyboardButton

	for _, task := range missedTasks {
		buttonText := fmt.Sprintf("✅ Выполнена? %s", utils.GetPillarName(task.Pillar))
		callbackData := fmt.Sprintf("missed_complete_%d", task.ID)

		keyboardRows = append(keyboardRows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(buttonText, callbackData),
		))
	}

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

	b.handleMessage(update.Message)
}

// handleMessage обрабатывает текстовые сообщения
func (b *Bot) handleMessage(msg *tgbotapi.Message) {
	text := msg.Text
	if text == "" {
		return
	}

	// Обработка команд с префиксами
	switch {
	case strings.HasPrefix(text, "/add "):
		b.handleAddTask(msg)
	case strings.HasPrefix(text, "/feelings "):
		b.handleFeelingsCommand(msg)
	case strings.HasPrefix(text, "/time "):
		b.handleChangeTime(msg)
	case strings.HasPrefix(text, "/date "):
		b.handleChangeDate(msg)
	default:
		if strings.HasPrefix(text, "/") {
			parts := strings.Fields(text)
			command := parts[0]

			if handler, exists := b.handlers[command]; exists {
				handler(msg)
			} else {
				b.SendMessageOrLogError("❌ Неизвестная команда. Используйте /help")
			}
		}
	}
}

func (b *Bot) handleCallbackQuery(callback *tgbotapi.CallbackQuery) {
	defer func(bot *tgbotapi.BotAPI, c tgbotapi.Chattable) {
		_, err := bot.Request(c)
		if err != nil {
			fmt.Printf("Telegram Bot request error: %s\n", err.Error())
		}
	}(b.bot, tgbotapi.NewCallback(callback.ID, "✅"))

	if callback.Message.Chat.ID != b.chatID {
		return
	}

	data := callback.Data
	log.Printf("Received callback: %s", data)

	switch {
	case strings.HasPrefix(data, "complete_"):
		b.handleCompleteTask(data)
	case strings.HasPrefix(data, "snooze_"):
		b.handleSnoozeTask(data)
	case strings.HasPrefix(data, "skip_reason_"):
		b.handleSkipReason(data)
	case strings.HasPrefix(data, "skip_"):
		b.handleSkipTask(data, callback.Message.MessageID)
	case strings.HasPrefix(data, "missed_complete_"):
		b.handleMissedCompleteTask(data, callback.Message.MessageID)
	}
}

// handleCompleteTask обрабатывает завершение задачи
func (b *Bot) handleCompleteTask(data string) {
	taskID, err := strconv.Atoi(strings.TrimPrefix(data, "complete_"))
	if err != nil {
		b.SendMessageOrLogError("❌ Ошибка обработки запроса")
		return
	}
	if err := database.NewRepository(b.db).UpdateTaskCompletion(taskID, true); err != nil {
		b.SendMessageOrLogError("❌ Ошибка обновления задачи")
		return
	}
	b.SendMessageOrLogError("✅ Задача выполнена!")
}

// handleSnoozeTask обрабатывает откладывание задачи
func (b *Bot) handleSnoozeTask(data string) {
	taskID, err := strconv.Atoi(strings.TrimPrefix(data, "snooze_"))
	if err != nil {
		b.SendMessageOrLogError("❌ Ошибка обработки запроса")
		return
	}
	var currentTime string
	err = b.db.GetDB().QueryRow("SELECT time_utc FROM tasks WHERE id = ?", taskID).Scan(&currentTime)
	if err != nil {
		b.SendMessageOrLogError("❌ Ошибка получения времени задачи")
		return
	}

	t, err := time.Parse("15:04", currentTime)
	if err != nil {
		b.SendMessageOrLogError("❌ Ошибка парсинга времени")
		return
	}

	newTime := t.Add(60 * time.Minute).Format("15:04")

	_, err = b.db.GetDB().Exec("UPDATE tasks SET time_utc = ? WHERE id = ?", newTime, taskID)
	if err != nil {
		b.SendMessageOrLogError("❌ Ошибка откладывания задачи")
		return
	}

	b.SendMessageOrLogError(fmt.Sprintf("⏰ Задача отложена до %s UTC", newTime))
}

// handleSkipTask обрабатывает начало процесса пропуска задачи
func (b *Bot) handleSkipTask(data string, messageID int) {
	taskID, err := strconv.Atoi(strings.TrimPrefix(data, "skip_"))
	if err != nil {
		b.SendMessageOrLogError("❌ Ошибка обработки запроса")
		return
	}

	b.safeDeleteMessage(messageID)

	reasonMsg := tgbotapi.NewMessage(b.chatID, "📝 Почему задача не выполнена?\n(Это поможет аналитике)")
	reasonMsg.ReplyMarkup = b.createSkipReasonKeyboard(taskID)
	_, err = b.bot.Send(reasonMsg)
	if err != nil {
		return
	}
}

func (b *Bot) handleSkipReason(data string) {
	parts := strings.Split(strings.TrimPrefix(data, "skip_reason_"), "_")
	if len(parts) != 2 {
		b.SendMessageOrLogError("❌ Ошибка обработки запроса")
		return
	}

	taskID, _ := strconv.Atoi(parts[0])
	reasonCode := parts[1]
	reasonText := b.skipReasons[reasonCode]

	repo := database.NewRepository(b.db)
	if err := repo.MarkTaskAsSkipped(taskID, reasonCode, reasonText); err != nil {
		b.SendMessageOrLogError("❌ Ошибка сохранения пропуска")
		log.Printf("Ошибка MarkTaskAsSkipped: %v", err)
		return
	}

	b.SendMessageOrLogError(fmt.Sprintf("➖ Задача пропущена\n📝 Причина: %s\n\n💡 Эта информация будет учтена в еженедельном анализе.", reasonText))
}

// handleMissedCompleteTask обрабатывает завершение пропущенной задачи
func (b *Bot) handleMissedCompleteTask(data string, messageID int) {
	taskID, err := strconv.Atoi(strings.TrimPrefix(data, "missed_complete_"))
	if err != nil {
		b.SendMessageOrLogError("❌ Ошибка обработки запроса")
		return
	}

	if err := database.NewRepository(b.db).UpdateTaskCompletion(taskID, true); err != nil {
		b.SendMessageOrLogError("❌ Ошибка обновления задачи")
		return
	}

	b.safeDeleteMessage(messageID)

	b.SendMessageOrLogError("✅ Задача отмечена выполненной!")
}

// safeDeleteMessage вспомогательная функция для безопасного удаления сообщений
func (b *Bot) safeDeleteMessage(messageID int) {
	deleteConfig := tgbotapi.NewDeleteMessage(b.chatID, messageID)

	resp, err := b.bot.Request(deleteConfig)
	if err != nil {
		log.Printf("⚠️ Ошибка при удалении сообщения %d: %v", messageID, err)
		return
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		log.Printf("⚠️ Не удалось декодировать ответ при удалении сообщения %d: %v", messageID, err)
	}

	if ok, exists := result["ok"]; exists {
		if isOk, okBool := ok.(bool); okBool && isOk {
			log.Printf("✅ Сообщение %d успешно удалено", messageID)
		}
	}
}
