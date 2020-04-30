package chatbot

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/doylecnn/new-nsfc-bot/storage"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

func cmdImportData(message *tgbotapi.Message) (replyMessage []*tgbotapi.MessageConfig, err error) {
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
			_logger.Warn().Err(err).Str("user", name).Int64("uid", tgid).Send()
		}
		fc, err := strconv.ParseInt(fcstr, 10, 64)
		if err != nil {
			_logger.Warn().Err(err).Str("user", name).Int64("id", tgid).Str("FC", fcstr).Send()
		}
		u := storage.User{ID: int(tgid), Name: name, NSAccounts: []storage.NSAccount{{Name: name, FC: storage.FriendCode(fc)}}}
		_, err = storage.GetUser(ctx, u.ID, 0)
		if err != nil {
			if status.Code(err) != codes.NotFound {
				continue
			}
			u.Set(ctx)
		} else {
			u.Update(ctx)
		}
	}
	return []*tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: "导入完毕。"}},
		nil
}

// cmdUpgradeData 数据结构更新
func cmdUpgradeData(message *tgbotapi.Message) (replyMessage []*tgbotapi.MessageConfig, err error) {
	if message.From.ID != botAdminID {
		return
	}
	ctx := context.Background()
	users, err := storage.GetAllUsers(ctx)
	if err != nil {
		err = Error{InnerError: err,
			ReplyText: "查询用户时出错",
		}
		return
	}
	client, err := firestore.NewClient(ctx, _projectID)
	if err != nil {
		return
	}
	defer client.Close()
	for _, u := range users {
		_, _, err := u.GetAnimalCrossingIsland(ctx)
		if err != nil {
			_logger.Error().Err(err).Msg("GetAnimalCrossingIsland")
			continue
		}
	}

	return []*tgbotapi.MessageConfig{{
		BaseChat: tgbotapi.BaseChat{
			ChatID:              message.Chat.ID,
			ReplyToMessageID:    message.MessageID,
			DisableNotification: true},
		Text: "done"}}, nil
}

func cmdListAllFriendCodes(message *tgbotapi.Message) (replyMessages []*tgbotapi.MessageConfig, err error) {
	if message.From.ID != botAdminID {
		return
	}
	ctx := context.Background()
	users, err := storage.GetAllUsers(ctx)
	if err != nil {
		err = Error{InnerError: err,
			ReplyText: "查询时出错",
		}
		return
	}
	var rst []string
	for i, u := range users {
		userinfo := fmt.Sprintf("%s: %d", u.Name, u.ID)
		for _, a := range u.NSAccounts {
			userinfo += fmt.Sprintf("\n +- %s", a.String())
		}
		rst = append(rst, userinfo)
		if i != 0 && i%50 == 0 {
			replyMessages = append(replyMessages, &tgbotapi.MessageConfig{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true},
				Text: strings.Join(rst, "\n")})
			rst = rst[:0]
		}
	}
	if len(rst) > 0 {
		replyMessages = append(replyMessages, &tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: strings.Join(rst, "\n")})
		rst = rst[:0]
	}

	return replyMessages, nil
}

func cmdToggleDebugMode(message *tgbotapi.Message) (replyMessages []*tgbotapi.MessageConfig, err error) {
	tgbot.Debug = !tgbot.Debug
	var debugInfo = "debug off"
	if tgbot.Debug {
		debugInfo = "debug on"
	}
	replyMessages = append(replyMessages, &tgbotapi.MessageConfig{
		BaseChat: tgbotapi.BaseChat{
			ChatID:              message.Chat.ID,
			ReplyToMessageID:    message.MessageID,
			DisableNotification: true},
		Text: debugInfo})
	return replyMessages, nil
}

func (c ChatBot) cmdClearMessages(message *tgbotapi.Message) (replyMessages []*tgbotapi.MessageConfig, err error) {
	if len(sentMsgs) > 0 {
		c.logger.Info().Int("sentMsgs len:", len(sentMsgs)).Send()
		sort.Slice(sentMsgs, func(i, j int) bool {
			return sentMsgs[i].Time.After(sentMsgs[j].Time)
		})
		var i = 0
		var foundOutDateMsg = false
		for j, sentMsg := range sentMsgs {
			if time.Since(sentMsg.Time).Minutes() > 1 {
				foundOutDateMsg = true
				i = j
				break
			}
		}
		if foundOutDateMsg {
			for _, sentMsg := range sentMsgs[i:] {
				c.TgBotClient.DeleteMessage(tgbotapi.NewDeleteMessage(sentMsg.ChatID, sentMsg.MsgID))
			}
		}
		sentMsgs = sentMsgs[0:0]
	}
	cacheForEdit.Purge()
	c.logger.Info().Int("sentMsgs len", len(sentMsgs)).
		Int("cacheForEdit len", cacheForEdit.Len()).
		Msg("clear")
	replyMessages = append(replyMessages, &tgbotapi.MessageConfig{
		BaseChat: tgbotapi.BaseChat{
			ChatID:              message.Chat.ID,
			ReplyToMessageID:    message.MessageID,
			DisableNotification: true},
		Text: "done"})
	return replyMessages, nil
}
