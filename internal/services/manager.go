package services

import (
	"five-pillars/internal/database"
)

type ServiceManager struct {
	Notification *NotificationService
	Analytics    *AnalyticsService
	Task         *TaskService
	repository   *database.Repository
}

func NewServiceManager(db *database.Database) *ServiceManager {
	repo := database.NewRepository(db)

	return &ServiceManager{
		Notification: nil,
		Analytics:    NewAnalyticsService(repo),
		Task:         NewTaskService(repo),
		repository:   repo,
	}
}

func (sm *ServiceManager) SetNotificationSender(sender NotificationSender) {
	sm.Notification = NewNotificationService(sender, sm.repository)
}
