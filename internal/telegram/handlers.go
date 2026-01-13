package telegram

import (
	"five-pillars/internal/utils"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"five-pillars/internal/database"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// handlers.go - обработчики команд Telegram бота

func (b *Bot) handleStart(msg *tgbotapi.Message) {
	message := `🎯 <b>5 Столпов 2026 - Трекер</b>

Доступные команды:
/today - Задачи на сегодня
/summary - Итоги дня
/week - Сводка за неделю
/add [задача] - Добавить задачу
/all - все задачи на сегодня
/time - изменить время выполнения задачи
/date - изменить время выполнения задачи
/feelings - Оценить свои ощущения
/help - Помощь

Пример:
/add energy Вечерний ритуал в 20:00
/feelings энергия=8 контроль=7 сон=7.5 настроение=Сосредоточен`

	b.SendMessageOrLogError(message)
}

func (b *Bot) handleToday(msg *tgbotapi.Message) {
	today := time.Now().UTC().Format("2006-01-02")
	repo := database.NewRepository(b.db)
	tasks, err := repo.GetTasksByDate(today)
	if err != nil {
		b.SendMessageOrLogError("❌ Ошибка получения задач")
		return
	}

	if len(tasks) == 0 {
		b.SendMessageOrLogError("📭 На сегодня задач нет")
		return
	}

	var message strings.Builder
	message.WriteString(fmt.Sprintf("📅 <b>Задачи на %s</b>\n\n", utils.GetCurrentMSKDate()))
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
				"⏰ %s\n"+
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

	b.SendMessageOrLogError(message.String())
}

func (b *Bot) handleSummary(msg *tgbotapi.Message) {
	today := time.Now().UTC().Format("2006-01-02")
	repo := database.NewRepository(b.db)
	summary, err := repo.GetDailySummary(today)
	if err != nil {
		b.SendMessageOrLogError("❌ Ошибка получения сводки")
		return
	}

	message := fmt.Sprintf(
		"📊 <b>Итоги дня %s</b>\n\n"+
			"%s\n\n"+
			"✅ Выполнено: %d/%d (%.0f%%)\n\n"+
			"<b>По столпам:</b>\n",
		utils.GetCurrentMSKDate(),
		utils.GetTimezoneInfo(),
		summary["completed"].(int),
		summary["total"].(int),
		summary["percentage"].(float64),
	)

	if stats, ok := summary["pillar_stats"].(map[string]int); ok {
		for pillar, count := range stats {
			emoji := utils.GetPillarEmoji(pillar)
			pillarName := utils.GetPillarName(pillar)
			message += fmt.Sprintf("%s %s: %d\n", emoji, pillarName, count)
		}
	}

	feelings, err := repo.GetFeelings(today)
	if err == nil {
		message += fmt.Sprintf(
			"\n<b>Ощущения:</b>\n"+
				"⚡ Энергия: %d/10\n"+
				"🎯 Контроль: %d/10\n",
			feelings.EnergyLevel,
			feelings.ControlLevel,
		)
		if feelings.SleepHours > 0 {
			message += fmt.Sprintf("😴 Сон: %.1f ч\n", feelings.SleepHours)
		}
		if feelings.Mood != "" {
			message += fmt.Sprintf("😊 Настроение: %s\n", feelings.Mood)
		}
	}

	b.SendMessageOrLogError(message)
}

func (b *Bot) handleAll(msg *tgbotapi.Message) {
	today := time.Now().UTC().Format("2006-01-02")
	repo := database.NewRepository(b.db)
	all, err := repo.GetTasksByDate(today)
	if err != nil {
		b.SendMessageOrLogError("❌ Ошибка получения сводки")
		return
	}

	message := "📅 <b>Сегодня:</b>\n\n"
	for _, t := range all {
		status := "❌"
		if t.Completed {
			status = "✅"
		} else if t.Skipped {
			status = "💤"
		}
		message += fmt.Sprintf("id: %d, %s %s\n", t.ID, t.Description, status)
	}
	b.SendMessageOrLogError(message)
}

func (b *Bot) handleChangeTime(msg *tgbotapi.Message) {
	text := strings.TrimPrefix(msg.Text, "/time ")
	parts := strings.SplitN(text, " ", 2)
	if len(parts) < 2 {
		b.SendMessageOrLogError("❌ Формат: /time [id] [новое время в UTC]")
		return
	}

	id, err := strconv.Atoi(parts[0])
	if err != nil {
		b.SendMessageOrLogError("❌ id должен быть числовой")
	}
	time2do := parts[1]

	re := regexp.MustCompile(`^([01][0-9]|2[0-3]):([0-5][0-9])$`)
	if !re.MatchString(time2do) {
		b.SendMessageOrLogError("❌ Время в HH:mm в UTC")
		return
	}

	repo := database.NewRepository(b.db)
	if err := repo.UpdateTaskTime(id, time2do); err != nil {
		b.SendMessageOrLogError("❌ Ошибка изменения времени задачи")
		return
	}
	b.SendMessageOrLogError(fmt.Sprintf(
		"✅ Время задачи id: %v обновлено на ⏰ %s UTC",
		id, time2do))
}

func (b *Bot) handleChangeDate(msg *tgbotapi.Message) {
	text := strings.TrimPrefix(msg.Text, "/date ")
	parts := strings.SplitN(text, " ", 2)
	if len(parts) < 2 {
		b.SendMessageOrLogError("❌ Формат: /date [id] [новое дата в формате YYYY-MM-DD]")
		return
	}

	id, err := strconv.Atoi(parts[0])
	if err != nil {
		b.SendMessageOrLogError("❌ id должен быть числовой")
	}
	date2do := parts[1]

	re := regexp.MustCompile(`^\d{4}-(0[1-9]|1[0-2])-(0[1-9]|[12]\d|3[01])$`)
	if !re.MatchString(date2do) {
		b.SendMessageOrLogError("❌ Время должно быть в YYYY-MM-DD")
		return
	}

	repo := database.NewRepository(b.db)
	if err := repo.UpdateTaskDate(id, date2do); err != nil {
		b.SendMessageOrLogError("❌ Ошибка изменения даты задачи")
		return
	}
	b.SendMessageOrLogError(fmt.Sprintf(
		"✅ Дата задачи #%v обновлена. 📅 %s ", id, date2do))
}

func (b *Bot) handleWeek(msg *tgbotapi.Message) {
	analytics, err := b.services.Analytics.GetWeeklyAnalytics()
	if err != nil {
		b.SendMessageOrLogError("❌ Ошибка получения сводки за неделю")
		return
	}

	message := fmt.Sprintf(
		"📈 <b>Аналитика за неделю %d</b>\n\n"+
			"📅 %s - %s\n\n"+
			"✅ Выполнено: %d/%d (%.0f%%)\n\n"+
			"<b>Эффективность по столпам:</b>\n",
		analytics.WeekNumber,
		analytics.StartDate,
		analytics.EndDate,
		analytics.TotalDone,
		analytics.TotalTasks,
		float64(analytics.TotalDone)/float64(analytics.TotalTasks)*100,
	)

	for pillar, stats := range analytics.PillarStats {
		p := database.Pillar(pillar)
		percentage := 0.0
		if stats.Total > 0 {
			percentage = float64(stats.Completed) / float64(stats.Total) * 100
		}
		message += fmt.Sprintf(
			"%s %s: %d/%d (%.0f%%)\n",
			database.PillarEmojis[p],
			database.PillarNames[p],
			stats.Completed,
			stats.Total,
			percentage,
		)
	}

	if len(analytics.AvgFeelings) > 0 {
		message += "\n<b>Средние ощущения:</b>\n"
		for pillar, avg := range analytics.AvgFeelings {
			p := database.Pillar(pillar)
			message += fmt.Sprintf("%s %s: %.1f/10\n", database.PillarEmojis[p], database.PillarNames[p], avg)
		}
	}

	if analytics.Insights != "" {
		message += fmt.Sprintf("\n<b>💡 Инсайты:</b>\n%s", analytics.Insights)
	}

	b.SendMessageOrLogError(message)
}

func (b *Bot) handleAddTask(msg *tgbotapi.Message) {
	text := strings.TrimPrefix(msg.Text, "/add ")
	parts := strings.SplitN(text, " ", 2)
	if len(parts) < 2 {
		b.SendMessageOrLogError("❌ Формат: /add [столп] [описание и время в UTC]")
		return
	}

	pillarStr := parts[0]
	description := parts[1]
	time2do := description[len(description)-5:]

	re := regexp.MustCompile(`^([01][0-9]|2[0-3]):([0-5][0-9])$`)
	if !re.MatchString(time2do) {
		b.SendMessageOrLogError("❌ Время в HH:mm в UTC")
		return
	}

	var pillar database.Pillar
	switch strings.ToLower(pillarStr) {
	case "энергия", "energy":
		pillar = database.Energy
	case "тело", "body":
		pillar = database.Body
	case "фокус", "focus":
		pillar = database.Focus
	case "быт", "life":
		pillar = database.Life
	case "баланс", "balance":
		pillar = database.Balance
	default:
		b.SendMessageOrLogError("❌ Неизвестный столп. Используйте: энергия, тело, фокус, быт, баланс")
		return
	}

	repo := database.NewRepository(b.db)
	task := database.DailyTask{
		Pillar:      pillar,
		Description: description,
		Completed:   false,
		TimeUTC:     time2do,
		Date:        time.Now().UTC().Format("2006-01-02"),
		Notes:       "Добавлено через Telegram",
	}

	if err := repo.AddTask(task); err != nil {
		b.SendMessageOrLogError("❌ Ошибка добавления задачи")
		return
	}

	b.SendMessageOrLogError(fmt.Sprintf(
		"✅ Добавлена задача:\n%s %s\n%s\n⏰ %s UTC",
		database.PillarEmojis[pillar],
		database.PillarNames[pillar],
		description,
		task.TimeUTC,
	))
}

func (b *Bot) handleFeelings(msg *tgbotapi.Message) {
	message := `📊 <b>Оцените свои ощущения за день</b>

Формат:
/feelings энергия=[1-10] контроль=[1-10] сон=[часы] настроение=[текст]

Пример:
/feelings энергия=8 контроль=7 сон=7.5 настроение=Сосредоточен`

	b.SendMessageOrLogError(message)
}

func (b *Bot) handleFeelingsCommand(msg *tgbotapi.Message) {
	text := strings.TrimPrefix(msg.Text, "/feelings ")
	metrics := make(map[string]string)
	pairs := strings.Fields(text)

	for _, pair := range pairs {
		parts := strings.Split(pair, "=")
		if len(parts) == 2 {
			key := strings.ToLower(strings.TrimSpace(parts[0]))
			value := strings.TrimSpace(parts[1])
			metrics[key] = value
		}
	}

	var energy, control int
	var sleep float64
	var mood string
	var err error

	if val, ok := metrics["энергия"]; ok {
		energy, err = strconv.Atoi(val)
		if err != nil || energy < 1 || energy > 10 {
			b.SendMessageOrLogError("❌ Энергия должна быть от 1 до 10")
			return
		}
	}

	if val, ok := metrics["контроль"]; ok {
		control, err = strconv.Atoi(val)
		if err != nil || control < 1 || control > 10 {
			b.SendMessageOrLogError("❌ Контроль должен быть от 1 до 10")
			return
		}
	}

	if val, ok := metrics["сон"]; ok {
		sleep, err = strconv.ParseFloat(val, 64)
		if err != nil || sleep <= 0 {
			b.SendMessageOrLogError("❌ Сон должен быть положительным числом")
			return
		}
	}

	if val, ok := metrics["настроение"]; ok {
		mood = val
	}

	date := time.Now().UTC().Format("2006-01-02")
	repo := database.NewRepository(b.db)
	feelings := database.DailyFeelings{
		Date:         date,
		EnergyLevel:  energy,
		ControlLevel: control,
		SleepHours:   sleep,
		Mood:         mood,
	}

	if err := repo.SaveFeelings(feelings); err != nil {
		b.SendMessageOrLogError("❌ Ошибка сохранения ощущений")
		return
	}

	message := fmt.Sprintf(
		"✅ Ощущения сохранены:\n\n"+
			"⚡ Энергия: %d/10\n"+
			"🎯 Контроль: %d/10\n",
		energy, control,
	)

	if sleep > 0 {
		message += fmt.Sprintf("😴 Сон: %.1f ч\n", sleep)
	}
	if mood != "" {
		message += fmt.Sprintf("😊 Настроение: %s\n", mood)
	}

	b.SendMessageOrLogError(message)
}

func (b *Bot) handleHelp(msg *tgbotapi.Message) {
	message := `📚 <b>Список команд</b>

<b>Основные команды:</b>
/today - Показать задачи на сегодня
/summary - Итоги дня с выполнением задач
/week - Аналитика за неделю

<b>Управление задачами:</b>
/add [столп] [описание] - Добавить задачу
Пример: /add energy Вечерний ритуал и время в UTC

/all - получить список всех задач на сегодня
Пример: /all

/time [id] [время в UTC] - Изменить время выполнения задачи
Пример: /change 3 10:00

/date [id] [YYYY-mm-DD] - Изменить дату выполнения задачи
Пример: /change 3 2026-01-10


<b>Управление задачами:</b>
/add [столп] [описание] - Добавить задачу
Пример: /add energy Вечерний ритуал


<b>Отслеживание ощущений:</b>
/feelings - Оценить свои ощущения за день
Пример: /feelings энергия=8 контроль=7 сон=7.5

<b>Столпы:</b>
⚖️ Энергия - energy, энергия
🏃 Тело - body, тело
🧠 Фокус - focus, фокус
🏠 Быт - life, быт
🔄 Баланс - balance, баланс`

	b.SendMessageOrLogError(message)
}
