package services

import (
	"time"

	"five-pillars/internal/database"
)

type TaskService struct {
	repository *database.Repository
}

func NewTaskService(repo *database.Repository) *TaskService {
	return &TaskService{
		repository: repo,
	}
}

func (ts *TaskService) CreateDefaultTasksToday(date string) error {
	tasks, err := ts.repository.GetTasksByDate(date)
	if err != nil || len(tasks) > 0 {
		return err
	}

	err2 := execute(date, err, ts)
	if err2 != nil {
		return err2
	}

	return nil
}

func (ts *TaskService) CreateDefaultTasksNextDay(date string) error {
	_, err := ts.repository.GetTasksByDate(date)
	if err != nil {
		return err
	}

	err2 := execute(date, err, ts)
	if err2 != nil {
		return err2
	}

	return nil
}

func execute(date string, err error, ts *TaskService) error {
	taskDate, err := time.Parse("2006-01-02", date)
	if err != nil {
		return err
	}
	weekday := taskDate.Weekday()

	baseTasks := []database.DailyTask{
		{
			Pillar:      database.Energy,
			Description: "День без алкоголя",
			Completed:   false,
			TimeUTC:     "15:00",
			Date:        date,
			Notes:       "Вечерний ритуал: кроссовки → активность → контрастный душ",
		},
	}

	switch weekday {
	case time.Monday, time.Wednesday, time.Friday:
		desc := "Беговая тренировка в 18:30"
		if weekday == time.Wednesday {
			desc = "Силовая тренировка 18:30"
		}
		baseTasks = append(baseTasks, database.DailyTask{
			Pillar:      database.Body,
			Description: desc,
			Completed:   false,
			TimeUTC:     "18:00",
			Date:        date,
			Notes:       "Ритм 2+1 - инвестиция в энергию",
		})
	}

	if weekday >= time.Monday && weekday <= time.Friday {
		baseTasks = append(baseTasks, database.DailyTask{
			Pillar:      database.Focus,
			Description: "Утренний блок 90 мин",
			Completed:   false,
			TimeUTC:     "06:00",
			Date:        date,
			Notes:       "Самая сложная задача дня",
		})

		baseTasks = append(baseTasks, database.DailyTask{
			Pillar:      database.Focus,
			Description: "Вечерний урок",
			Completed:   false,
			TimeUTC:     "18:00",
			Date:        date,
			Notes:       "вечерний урок 15 мин",
		})
	}

	if weekday == time.Saturday {
		baseTasks = append(baseTasks, database.DailyTask{
			Pillar:      database.Life,
			Description: "Проверяй смету по кваритре, ищи деньги, подбивай таймлайн конца проекта (2 часа)",
			Completed:   false,
			TimeUTC:     "08:00",
			Date:        date,
			Notes:       "Одно конкретное действие: замер, выбор, упаковка",
		})
	}

	if weekday == time.Sunday {
		baseTasks = append(baseTasks, database.DailyTask{
			Pillar:      database.Balance,
			Description: "Ревью недели + план",
			Completed:   false,
			TimeUTC:     "11:00",
			Date:        date,
			Notes:       "30 мин: анализ + корректировка плана",
		})
	}

	for _, task := range baseTasks {
		if err := ts.repository.AddTask(task); err != nil {
			return err
		}
	}
	return nil
}
