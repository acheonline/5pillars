package database

import "time"

type Pillar string

const (
	Energy  Pillar = "energy"
	Body    Pillar = "body"
	Focus   Pillar = "focus"
	Life    Pillar = "life"
	Balance Pillar = "balance"
)

var PillarNames = map[Pillar]string{
	Energy:  "⚖️ Энергия",
	Body:    "🏃 Тело",
	Focus:   "🧠 Фокус",
	Life:    "🏠 Быт",
	Balance: "🔄 Баланс",
}

var PillarEmojis = map[Pillar]string{
	Energy:  "⚖️",
	Body:    "🏃",
	Focus:   "🧠",
	Life:    "🏠",
	Balance: "🔄",
}

type DailyTask struct {
	ID          int       `json:"id"`
	Pillar      Pillar    `json:"pillar"`
	Description string    `json:"description"`
	Completed   bool      `json:"completed"`
	TimeUTC     string    `json:"time_utc"`
	Date        string    `json:"date"`
	Notes       string    `json:"notes,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type DailyFeelings struct {
	ID           int       `json:"id"`
	Date         string    `json:"date"`
	EnergyLevel  int       `json:"energy_level"`  // 1-10
	ControlLevel int       `json:"control_level"` // 1-10
	SleepHours   float64   `json:"sleep_hours,omitempty"`
	Mood         string    `json:"mood,omitempty"`
	Notes        string    `json:"notes,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

type WeeklyAnalytics struct {
	WeekNumber   int                   `json:"week_number"`
	StartDate    string                `json:"start_date"`
	EndDate      string                `json:"end_date"`
	TotalDone    int                   `json:"total_done"`
	TotalTasks   int                   `json:"total_tasks"`
	TotalSkipped int                   `json:"total_skipped"`
	PillarStats  map[string]PillarStat `json:"pillar_stats"`
	AvgFeelings  map[string]float64    `json:"avg_feelings"`
	Insights     string                `json:"insights"`
}

type PillarStat struct {
	Completed int `json:"completed"`
	Total     int `json:"total"`
}

type TaskNotification struct {
	ID          int    `json:"id"`
	Pillar      string `json:"pillar"`
	Description string `json:"description"`
	TimeUTC     string `json:"time_utc"`
	Notes       string `json:"notes"`
	Date        string `json:"date"`
}
