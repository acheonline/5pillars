package utils

import (
	"fmt"
	"time"
)

var (
	moscowLocation *time.Location
)

func init() {
	// Пытаемся загрузить локацию Москвы
	var err error
	moscowLocation, err = time.LoadLocation("Europe/Moscow")
	if err != nil {
		// Fallback: UTC+3
		moscowLocation = time.FixedZone("MSK", 3*60*60)
	}
}

// UTCTimeToMSK конвертирует время UTC в МСК для отображения
func UTCTimeToMSK(utcTime string) (string, error) {
	// Парсим время UTC
	t, err := time.Parse("15:04", utcTime)
	if err != nil {
		return "", err
	}

	// Создаем полную дату (сегодня) в UTC
	now := time.Now().UTC()
	dateTime := time.Date(now.Year(), now.Month(), now.Day(),
		t.Hour(), t.Minute(), 0, 0, time.UTC)

	// Конвертируем в МСК
	mskTime := dateTime.In(moscowLocation)

	return mskTime.Format("15:04"), nil
}

// FormatTimeForDisplay форматирует время для отображения (UTC → МСК)
func FormatTimeForDisplay(utcTime string) string {
	mskTime, err := UTCTimeToMSK(utcTime)
	if err != nil {
		return utcTime + " UTC" // fallback
	}

	// Вычисляем разницу
	t, _ := time.Parse("15:04", utcTime)
	mskT, _ := time.Parse("15:04", mskTime)

	diffHours := (mskT.Hour() - t.Hour() + 24) % 24

	return fmt.Sprintf("%s МСК (%s UTC, +%d)", mskTime, utcTime, diffHours)
}

// GetCurrentMSKTime возвращает текущее время в МСК
func GetCurrentMSKTime() string {
	now := time.Now().In(moscowLocation)
	return now.Format("15:04")
}

// GetCurrentMSKDate возвращает текущую дату в МСК
func GetCurrentMSKDate() string {
	now := time.Now().In(moscowLocation)
	return now.Format("2006-01-02")
}

// ParseMSKTime парсит время МСК в UTC
func ParseMSKTimeToUTC(mskTime string) (string, error) {
	// Парсим время МСК
	t, err := time.Parse("15:04", mskTime)
	if err != nil {
		return "", err
	}

	// Создаем полную дату в МСК
	now := time.Now().In(moscowLocation)
	dateTime := time.Date(now.Year(), now.Month(), now.Day(),
		t.Hour(), t.Minute(), 0, 0, moscowLocation)

	// Конвертируем в UTC
	utcTime := dateTime.UTC()

	return utcTime.Format("15:04"), nil
}

// GetTimezoneInfo возвращает информацию о временной зоне
func GetTimezoneInfo() string {
	nowUTC := time.Now().UTC()
	nowMSK := nowUTC.In(moscowLocation)

	_, offset := nowMSK.Zone()
	offsetHours := offset / 3600

	return fmt.Sprintf("🕐 Текущее время: %s МСК (UTC+%d)\n   Серверное время: %s UTC",
		nowMSK.Format("15:04"), offsetHours, nowUTC.Format("15:04"))
}
