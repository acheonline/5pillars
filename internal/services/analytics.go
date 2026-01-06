package services

import (
	"fmt"
	"strings"
	"time"

	"five-pillars/internal/database"
)

type AnalyticsService struct {
	repository *database.Repository
}

func NewAnalyticsService(repo *database.Repository) *AnalyticsService {
	return &AnalyticsService{
		repository: repo,
	}
}

func (as *AnalyticsService) GetWeeklyAnalytics() (*database.WeeklyAnalytics, error) {
	now := time.Now()
	year, week := now.ISOWeek()
	startDate := as.firstDayOfISOWeek(year, week)
	endDate := startDate.AddDate(0, 0, 6)

	analytics, err := as.repository.GetWeeklyAnalytics(
		startDate.Format("2006-01-02"),
		endDate.Format("2006-01-02"),
	)
	if err != nil {
		return nil, err
	}

	analytics.WeekNumber = week
	analytics.Insights = as.generateInsights(analytics)

	return analytics, nil
}

func (as *AnalyticsService) generateInsights(analytics *database.WeeklyAnalytics) string {
	var insights []string

	completionRate := float64(analytics.TotalDone) / float64(analytics.TotalTasks) * 100

	if completionRate < 50 {
		insights = append(insights, "💪 Нужно больше фокуса на выполнении задач")
	} else if completionRate > 80 {
		insights = append(insights, "🎯 Отличная неделя! Продолжайте в том же духе")
	} else {
		insights = append(insights, "📈 Хороший прогресс, есть куда расти")
	}

	for pillar, stats := range analytics.PillarStats {
		rate := float64(stats.Completed) / float64(stats.Total) * 100
		p := database.Pillar(pillar)

		if rate < 40 {
			insights = append(insights, fmt.Sprintf(
				"⚠️ %s требует внимания: %.0f%% выполнено",
				database.PillarNames[p], rate,
			))
		}
	}

	if avgEnergy, ok := analytics.AvgFeelings["energy"]; ok {
		if avgEnergy < 5 {
			insights = append(insights, "🔋 Уровень энергии низкий. Проверьте сон и нагрузку")
		} else if avgEnergy > 8 {
			insights = append(insights, "⚡ Отличный уровень энергии!")
		}
	}

	if len(insights) == 0 {
		return "📊 Данных для анализа недостаточно. Продолжайте заполнять трекер!"
	}

	return strings.Join(insights, "\n")
}

func (as *AnalyticsService) firstDayOfISOWeek(year, week int) time.Time {
	date := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	isoYear, isoWeek := date.ISOWeek()

	for date.Weekday() != time.Monday {
		date = date.AddDate(0, 0, -1)
		isoYear, isoWeek = date.ISOWeek()
	}

	for isoYear < year {
		date = date.AddDate(0, 0, 7)
		isoYear, isoWeek = date.ISOWeek()
	}

	for isoWeek < week {
		date = date.AddDate(0, 0, 7)
		isoYear, isoWeek = date.ISOWeek()
	}

	return date
}
