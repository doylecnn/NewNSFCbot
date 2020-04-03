package chatbot

import (
	"encoding/json"
	"net/url"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// BotCommand BotCommand
type BotCommand struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

func setMyCommands(commands []BotCommand) (response tgbotapi.APIResponse, err error) {
	v := url.Values{}
	var data []byte
	if data, err = json.Marshal(commands); err == nil {
		v.Add("commands", string(data))
	} else {
		return
	}

	return tgbot.MakeRequest("setMyCommands", v)
}

func getMyCommands() (commands []BotCommand, err error) {
	v := url.Values{}
	resp, err := tgbot.MakeRequest("getMyCommands", v)
	if err != nil {
		return
	}

	err = json.Unmarshal(resp.Result, &commands)
	return
}
