package telegram

import "log"

func (b *Bot) SendMessageOrLogError(message string) {
	err := b.SendMessage(message)
	if err != nil {
		log.Fatalf(err.Error())
	}
}
