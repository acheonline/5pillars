package database

import (
	"database/sql"
)

type Repository struct {
	Db *Database
}

func NewRepository(db *Database) *Repository {
	return &Repository{Db: db}
}

// UpdateTaskDate обновляет дату задачи по ID
func (r *Repository) UpdateTaskDate(taskID int, newTime string) error {
	query := `
		UPDATE tasks 
		SET time_utc = ?
		WHERE id = ?
	`

	_, err := r.Db.db.Exec(query, newTime, taskID)
	if err != nil {
		return err
	}

	return nil
}

// Task repository methods
func (r *Repository) GetTasksByDate(date string) ([]DailyTask, error) {
	rows, err := r.Db.db.Query(`
		SELECT id, pillar, description, completed, time_utc, date, notes, created_at
		FROM tasks 
		WHERE date = ?
		ORDER BY time_utc
	`, date)
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			panic(err)
		}
	}(rows)

	var tasks []DailyTask
	for rows.Next() {
		var task DailyTask
		err := rows.Scan(
			&task.ID,
			&task.Pillar,
			&task.Description,
			&task.Completed,
			&task.TimeUTC,
			&task.Date,
			&task.Notes,
			&task.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}

func (r *Repository) AddTask(task DailyTask) error {
	_, err := r.Db.db.Exec(`
		INSERT INTO tasks (pillar, description, completed, time_utc, date, notes)
		VALUES (?, ?, ?, ?, ?, ?)
	`, task.Pillar, task.Description, task.Completed, task.TimeUTC, task.Date, task.Notes)
	return err
}

func (r *Repository) UpdateTaskCompletion(taskID int, completed bool) error {
	_, err := r.Db.db.Exec("UPDATE tasks SET completed = ? WHERE id = ?", completed, taskID)
	return err
}

func (r *Repository) DeleteTask(taskID int) error {
	_, err := r.Db.db.Exec("DELETE FROM tasks WHERE id = ?", taskID)
	return err
}

func (r *Repository) GetTasksForNotification(currentTime, today string) ([]TaskNotification, error) {
	rows, err := r.Db.db.Query(`
		SELECT id, pillar, description, time_utc, notes, date
		FROM tasks 
		WHERE date = ? AND time_utc <= ? AND completed = 0
	`, today, currentTime)

	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			panic(err)
		}
	}(rows)

	var tasks []TaskNotification
	for rows.Next() {
		var task TaskNotification
		err := rows.Scan(
			&task.ID,
			&task.Pillar,
			&task.Description,
			&task.TimeUTC,
			&task.Notes,
			&task.Date,
		)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}

// GetMissedTasks возвращает задачи, время которых уже прошло, но они не выполнены
func (r *Repository) GetMissedTasks(date, currentTime string) ([]TaskNotification, error) {
	rows, err := r.Db.db.Query(`
		SELECT id, pillar, description, time_utc, notes, date
		FROM tasks 
		WHERE date = ? 
		AND time_utc <= ? 
		AND completed = 0
		ORDER BY time_utc
	`, date, currentTime)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []TaskNotification
	for rows.Next() {
		var task TaskNotification
		err := rows.Scan(
			&task.ID,
			&task.Pillar,
			&task.Description,
			&task.TimeUTC,
			&task.Notes,
			&task.Date,
		)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}

// Feelings repository methods
func (r *Repository) SaveFeelings(feelings DailyFeelings) error {
	_, err := r.Db.db.Exec(`
		INSERT OR REPLACE INTO feelings 
		(date, energy_level, control_level, sleep_hours, mood, notes)
		VALUES (?, ?, ?, ?, ?, ?)
	`, feelings.Date, feelings.EnergyLevel, feelings.ControlLevel, feelings.SleepHours, feelings.Mood, feelings.Notes)
	return err
}

func (r *Repository) GetFeelings(date string) (*DailyFeelings, error) {
	var feelings DailyFeelings
	err := r.Db.db.QueryRow(`
		SELECT id, date, energy_level, control_level, sleep_hours, mood, notes, created_at
		FROM feelings 
		WHERE date = ?
	`, date).Scan(
		&feelings.ID,
		&feelings.Date,
		&feelings.EnergyLevel,
		&feelings.ControlLevel,
		&feelings.SleepHours,
		&feelings.Mood,
		&feelings.Notes,
		&feelings.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &feelings, nil
}

// Analytics repository methods
func (r *Repository) GetDailySummary(date string) (map[string]interface{}, error) {
	summary := make(map[string]interface{})

	var total, completed int
	err := r.Db.db.QueryRow(`
		SELECT 
			COUNT(*) as total,
			SUM(CASE WHEN completed = 1 THEN 1 ELSE 0 END) as completed
		FROM tasks 
		WHERE date = ?
	`, date).Scan(&total, &completed)

	if err != nil {
		return nil, err
	}

	summary["date"] = date
	summary["total"] = total
	summary["completed"] = completed

	percentage := 0.0
	if total > 0 {
		percentage = float64(completed) / float64(total) * 100
	}
	summary["percentage"] = percentage

	pillarStats := make(map[string]int)
	rows, err := r.Db.db.Query(`
		SELECT pillar, COUNT(*) as count
		FROM tasks 
		WHERE date = ? AND completed = 1
		GROUP BY pillar
	`, date)

	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var pillar string
			var count int
			rows.Scan(&pillar, &count)
			pillarStats[pillar] = count
		}
	}
	summary["pillar_stats"] = pillarStats

	return summary, nil
}

func (r *Repository) GetWeeklyAnalytics(startDate, endDate string) (*WeeklyAnalytics, error) {
	analytics := &WeeklyAnalytics{
		StartDate:   startDate,
		EndDate:     endDate,
		PillarStats: make(map[string]PillarStat),
		AvgFeelings: make(map[string]float64),
	}

	rows, err := r.Db.db.Query(`
		SELECT 
			pillar,
			COUNT(*) as total,
			SUM(CASE WHEN completed = 1 THEN 1 ELSE 0 END) as completed
		FROM tasks 
		WHERE date BETWEEN ? AND ?
		GROUP BY pillar
	`, startDate, endDate)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var pillar string
		var stats PillarStat
		rows.Scan(&pillar, &stats.Total, &stats.Completed)
		analytics.PillarStats[pillar] = stats
		analytics.TotalTasks += stats.Total
		analytics.TotalDone += stats.Completed
	}

	metricRows, err := r.Db.db.Query(`
		SELECT 
			AVG(energy_level) as avg_energy,
			AVG(control_level) as avg_control
		FROM feelings 
		WHERE date BETWEEN ? AND ?
	`, startDate, endDate)

	if err == nil {
		defer metricRows.Close()
		if metricRows.Next() {
			var avgEnergy, avgControl sql.NullFloat64
			metricRows.Scan(&avgEnergy, &avgControl)

			if avgEnergy.Valid {
				analytics.AvgFeelings["energy"] = avgEnergy.Float64
			}
			if avgControl.Valid {
				analytics.AvgFeelings["control"] = avgControl.Float64
			}
		}
	}

	return analytics, nil
}
