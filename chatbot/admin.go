package chatbot

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/doylecnn/new-nsfc-bot/storage"
	"github.com/sirupsen/logrus"
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
			logrus.Warnf("user name:%s, id:%s, err:%v", name, tgidstr, err)
		}
		fc, err := strconv.ParseInt(fcstr, 10, 64)
		if err != nil {
			logrus.Warnf("user name:%s, id:%s, fc:%s, err:%v", name, tgidstr, fcstr, err)
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
		island, err := u.GetAnimalCrossingIsland(ctx)
		if err != nil {
			logrus.WithError(err).Error("GetAnimalCrossingIsland")
			continue
		}
		if int(island.Timezone) == 0 {
			island.Timezone = storage.Timezone(8 * 3600)
			island.LastPrice = storage.PriceHistory{}
		}
		var weekStartDateUTC = time.Now().AddDate(0, 0, 0-int(time.Now().Weekday())).Truncate(24 * time.Hour)
		var weekStartDateLoc = time.Date(weekStartDateUTC.Year(), weekStartDateUTC.Month(), weekStartDateUTC.Day(), 0, 0, 0, 0, island.Timezone.Location())
		var weekStartDate = weekStartDateLoc.UTC()
		var weekEndDate = weekStartDate.AddDate(0, 0, 7)
		wph, err := storage.GetWeeklyDTCPriceHistory(ctx, u.ID, weekStartDate, weekEndDate)
		if err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{"uid": u.ID}).Error("GetWeeklyDTCPriceHistory")
			continue
		}
		if err = storage.DeleteCollection(ctx, client, client.Collection(fmt.Sprintf("users/%d/games/animal_crossing/price_history", u.ID)), 10); err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{"uid": u.ID}).Error("delete priceHistory")
			continue
		}
		var lastph *storage.PriceHistory
		if l := len(wph); l > 3 {
			wph = wph[l-3 : 3]
		}
		for i, ph := range wph {
			if i >= 3 {
				break
			}
			ph.Timezone = island.Timezone
			if i == 0 {
				ph.Date = weekStartDate
				weekStartDate = weekStartDate.AddDate(0, 0, 1)
			} else if i%2 == 1 {
				weekStartDate = weekStartDate.Add(8 * time.Hour)
				ph.Date = weekStartDate
			} else {
				weekStartDate = weekStartDate.Add(4 * time.Hour)
				ph.Date = weekStartDate
				weekStartDate = weekStartDate.Add(12 * time.Hour)
			}
			lastph = ph
			if err = ph.Set(ctx, u.ID); err != nil {
				logrus.WithError(err).WithFields(logrus.Fields{"uid": u.ID}).Error("update default island")
				continue
			}
		}
		if lastph != nil {
			island.LastPrice = *lastph
		}
		if err = island.Update(ctx); err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{"uid": u.ID}).Error("update default island")
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
