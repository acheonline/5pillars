package utils

// Вспомогательные функции для получения названий и эмодзи столпов
func GetPillarName(pillarStr string) string {
	switch pillarStr {
	case "energy":
		return "⚖️ Энергия"
	case "body":
		return "🏃 Тело"
	case "focus":
		return "🧠 Фокус"
	case "life":
		return "🏠 Быт"
	case "balance":
		return "🔄 Баланс"
	default:
		return pillarStr
	}
}

func GetPillarEmoji(pillarStr string) string {
	switch pillarStr {
	case "energy":
		return "⚖️"
	case "body":
		return "🏃"
	case "focus":
		return "🧠"
	case "life":
		return "🏠"
	case "balance":
		return "🔄"
	default:
		return "📌"
	}
}
