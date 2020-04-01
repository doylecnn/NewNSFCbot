package chatbot

import (
	"context"
	"strconv"
	"strings"

	"github.com/doylecnn/new-nsfc-bot/storage"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	log "github.com/sirupsen/logrus"
)

func cmdImportData(message *tgbotapi.Message) (replyMessage *tgbotapi.MessageConfig, err error) {
	if message.From.ID != botAdminID {
		return
	}
	args := strings.Split(strings.TrimSpace(message.CommandArguments()), ";")
	ctx := context.Background()
	for _, arg := range args {
		var parts = strings.Split(arg, ",")
		if len(parts) != 3 {
			continue
		}
		tgidstr, fcstr, name := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), strings.TrimSpace(parts[2])
		tgid, err := strconv.ParseInt(tgidstr, 10, 64)
		if err != nil {
			log.Warnf("user name:%s, id:%s, err:%v", name, tgidstr, err)
		}
		fc, err := strconv.ParseInt(fcstr, 10, 64)
		if err != nil {
			log.Warnf("user name:%s, id:%s, fc:%s, err:%v", name, tgidstr, fcstr, err)
		}
		u := storage.User{ID: int(tgid), Name: name, NSAccounts: []storage.NSAccount{storage.NSAccount{Name: name, FC: storage.FriendCode(fc)}}}
		u.Create(ctx)
	}
	return &tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: "导入完毕。"},
		nil
}
