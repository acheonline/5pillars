package app

import (
	"context"
	"log"
	"time"

	"five-pillars/internal/config"
	"five-pillars/internal/database"
	"five-pillars/internal/services"
	"five-pillars/internal/telegram"

	"github.com/robfig/cron/v3"
)

type Application struct {
	config     *config.Config
	db         *database.Database
	bot        *telegram.Bot
	services   *services.ServiceManager
	cron       *cron.Cron
	cancelFunc context.CancelFunc
	ctx        context.Context
}

func New(cfg *config.Config) (*Application, error) {
	db, err := database.New(cfg.Database.Path)
	if err != nil {
		return nil, err
	}

	serviceManager := services.NewServiceManager(db)
	bot, err := telegram.NewBot(cfg.Telegram.Token, cfg.Telegram.ChatID, db, serviceManager)
	if err != nil {
		db.Close()
		return nil, err
	}

	serviceManager.SetNotificationSender(bot)
	ctx, cancel := context.WithCancel(context.Background())

	app := &Application{
		config:     cfg,
		db:         db,
		bot:        bot,
		services:   serviceManager,
		cron:       cron.New(),
		cancelFunc: cancel,
		ctx:        ctx,
	}

	app.setupCronJobs()

	return app, nil
}

func (a *Application) Start() error {
	log.Println("🚀 Запуск приложения...")

	go a.bot.Start(a.ctx)

	a.cron.Start()
	time.Sleep(3 * time.Second)

	log.Println("🔍 Проверка пропущенных уведомлений...")
	a.services.Notification.SendMissedNotifications()

	time.Sleep(1 * time.Second)

	a.sendWelcomeMessage()

	today := time.Now().UTC().Format("2006-01-02")
	if err := a.services.Task.CreateDefaultTasksToday(today); err != nil {
		log.Printf("⚠️ Ошибка создания задач: %v", err)
	}

	log.Printf("✅ Приложение запущено. Бот: @%s", a.bot.GetUsername())
	log.Printf("🌐 API доступен на порту: %s", a.config.Server.Port)

	return nil
}

func (a *Application) Stop() error {
	log.Println("🛑 Остановка приложения...")

	a.cancelFunc()
	a.cron.Stop()

	if err := a.db.Close(); err != nil {
		log.Printf("⚠️ Ошибка закрытия БД: %v", err)
	}

	log.Println("✅ Приложение остановлено")
	return nil
}

func (a *Application) setupCronJobs() {
	// Проверка уведомлений каждую минуту
	_, err := a.cron.AddFunc("* * * * *", func() {
		a.services.Notification.CheckAndSendNotifications()
	})
	if err != nil {
		panic(err)
	}

	// Напоминание о задачах на день с 6 утра до 18 каждые 2 часа
	_, err = a.cron.AddFunc("0 3-18/2 * * *", func() {
		a.services.Notification.SendAllTodayTaskNotification()
	})
	if err != nil {
		panic(err)
	}

	// Сводка дня в 21:55 UTC+3
	a.cron.AddFunc("55 18 * * *", func() {
		a.services.Notification.SendDailySummary()
	})

	// Создание задач на следующий день в 22:00 UTC+3
	a.cron.AddFunc("0 19 * * *", func() {
		tomorrow := time.Now().UTC().AddDate(0, 0, 1).Format("2006-01-02")
		if err := a.services.Task.CreateDefaultTasksNextDay(tomorrow); err != nil {
			log.Printf("⚠️ Ошибка создания задач: %v", err)
		}
	})

	// Напоминание о внесении ощущений в 18:00 UTC
	a.cron.AddFunc("0 18 * * *", func() {
		a.bot.SendMessage(
			"📝 Не забудьте оценить свои ощущения за день!\n" +
				"Используйте команду: /feelings энергия=... контроль=... сон=...",
		)
	})
}

func (a *Application) sendWelcomeMessage() {
	message := `🎯 <b>5 Столпов 2026</b>

Ваш трекер успешно запущен!

Сегодня: ` + time.Now().UTC().Format("2006-01-02") + `

Используйте команды:
/today - задачи на сегодня
/summary - итоги дня
/week - аналитика за неделю
/feelings - оценить ощущения
/add - доабвить задачу
/all - список всех задач на сегодня
/time - изменить время выполнения задачи
/date - изменить дату выполнения задачи
/help - справка по командам`

	a.bot.SendMessage(message)
}
