package chatbot

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/doylecnn/new-nsfc-bot/storage"
	log "github.com/sirupsen/logrus"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

func cmdAddMyIsland(message *tgbotapi.Message) (replyMessage *tgbotapi.MessageConfig, err error) {
	argstr := strings.TrimSpace(message.CommandArguments())
	if len(argstr) <= 1 {
		return
	}
	var username = message.From.UserName
	if len(username) == 0 {
		username = message.From.FirstName + " " + message.From.LastName
	}
	args := strings.Split(argstr, " ")
	if len(args) < 2 {
		return
	}
	islandName := args[0]
	var hemisphere int
	if args[1] == "N" {
		hemisphere = 0
	} else if args[1] == "S" {
		hemisphere = 1
	} else {
		return nil, Error{InnerError: err,
			ReplyText: "请使用 N 表示北半球，S 表示南半球",
		}
	}
	fruits := args[2:]

	var island *storage.Island
	ctx := context.Background()
	u, err := storage.GetUser(ctx, message.From.ID)
	if err != nil || u == nil {
		return nil, Error{InnerError: err,
			ReplyText: fmt.Sprintf("请先添加FC。error info: %v", err),
		}
	}
	if island, err = u.GetAnimalCrossingIsland(ctx); err != nil {
		return nil, Error{InnerError: err,
			ReplyText: fmt.Sprintf("请先添加FC。error info: %v", err),
		}
	} else if err == nil && island != nil {
		island, err = u.GetAnimalCrossingIsland(ctx)
		if err != nil {
			return
		}
		island.Name = islandName
		island.Hemisphere = hemisphere
		island.Fruits = fruits
		if err = u.SetAirportStatus(ctx, *island); err != nil {
			return nil, Error{InnerError: err,
				ReplyText: fmt.Sprintf("更新岛屿信息时出错"),
			}
		}
	} else {
		island = &storage.Island{Name: islandName, Hemisphere: hemisphere, AirportIsOpen: false, AirportPassword: "", Fruits: fruits}
		if err = u.AddAnimalCrossingIsland(ctx, *island); err != nil {
			return nil, Error{InnerError: err,
				ReplyText: fmt.Sprintf("记录岛屿时出错"),
			}
		}
	}

	return &tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: fmt.Sprintf("完成。添加了岛屿 %s 的信息。", islandName)},
		nil
}

func cmdMyIsland(message *tgbotapi.Message) (replyMessage *tgbotapi.MessageConfig, err error) {
	ctx := context.Background()
	u, err := storage.GetUser(ctx, message.From.ID)
	if err != nil {
		return nil, Error{InnerError: err,
			ReplyText: "查询记录时出错了",
		}
	}
	if u == nil {
		log.Debug("没有找到用户记录")
		return &tgbotapi.MessageConfig{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true},
				Text: "没有找到您的记录，请先使用 addisland 命令添加岛屿记录"},
			nil
	}
	log.Debugf("user:%s", u.Name)
	island, err := u.GetAnimalCrossingIsland(ctx)
	if err != nil {
		return nil, Error{InnerError: err,
			ReplyText: "查询记录时出错了",
		}
	}
	if island == nil {
		return &tgbotapi.MessageConfig{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true},
				Text: "没有找到您的记录，请先使用 addisland 命令添加岛屿记录"},
			nil
	}
	return &tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: island.String()},
		nil
}

func cmdOpenIsland(message *tgbotapi.Message) (replyMessage *tgbotapi.MessageConfig, err error) {
	password := strings.TrimSpace(message.CommandArguments())

	ctx := context.Background()
	u, err := storage.GetUser(ctx, message.From.ID)
	if err != nil {
		return nil, Error{InnerError: err,
			ReplyText: "查询记录时出错了",
		}
	}
	if u == nil {
		log.Debug("没有找到用户记录")
		return &tgbotapi.MessageConfig{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true},
				Text: "没有找到您的记录，请先使用 addisland 命令添加岛屿记录"},
			nil
	}
	log.Debugf("user:%s", u.Name)
	island, err := u.GetAnimalCrossingIsland(ctx)
	if err != nil {
		return nil, Error{InnerError: err,
			ReplyText: "查询记录时出错了",
		}
	}
	if island == nil {
		return &tgbotapi.MessageConfig{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true},
				Text: "没有找到您的记录，请先使用 addisland 命令添加岛屿记录"},
			nil
	}
	if !island.AirportIsOpen || island.AirportPassword != password {
		island.AirportIsOpen = true
		island.AirportPassword = password
		u.SetAirportStatus(ctx, *island)
	}
	return &tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: island.String()},
		nil
}

func cmdCloseIsland(message *tgbotapi.Message) (replyMessage *tgbotapi.MessageConfig, err error) {
	ctx := context.Background()
	u, err := storage.GetUser(ctx, message.From.ID)
	if err != nil {
		return nil, Error{InnerError: err,
			ReplyText: "查询记录时出错了",
		}
	}
	if u == nil {
		log.Debug("没有找到用户记录")
		return &tgbotapi.MessageConfig{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true},
				Text: "没有找到您的记录，请先使用 addisland 命令添加岛屿记录"},
			nil
	}
	log.Debugf("user:%s", u.Name)
	island, err := u.GetAnimalCrossingIsland(ctx)
	if err != nil {
		return nil, Error{InnerError: err,
			ReplyText: "查询记录时出错了",
		}
	}
	if island == nil {
		return &tgbotapi.MessageConfig{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true},
				Text: "没有找到您的记录，请先使用 addisland 命令添加岛屿记录"},
			nil
	}
	if island.AirportIsOpen {
		island.AirportIsOpen = false
		island.AirportPassword = ""
		u.SetAirportStatus(ctx, *island)
	}
	return &tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: island.String()},
		nil
}

func cmdListIslands(message *tgbotapi.Message) (replyMessage *tgbotapi.MessageConfig, err error) {
	return &tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: "https://tgbot-ns-fc-wed-18-mar-2020.appspot.com/islands"},
		nil
}

func cmdDTCPriceUpdate(message *tgbotapi.Message) (replyMessage *tgbotapi.MessageConfig, err error) {
	pricestr := strings.TrimSpace(message.CommandArguments())
	if len(pricestr) == 0 {
		return
	}

	price, err := strconv.ParseInt(pricestr, 10, 64)
	if err != nil {
		return nil, Error{InnerError: err,
			ReplyText: "报价只能是数字",
		}
	}

	uid := message.From.ID

	ctx := context.Background()
	err = storage.UpdateDTCPrice(ctx, uid, int(price))
	if err != nil {
		return nil, Error{InnerError: err,
			ReplyText: "更新报价时出错",
		}
	}
	return &tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: "更新大头菜报价成功"},
		nil
}

func cmdDTCMaxPriceInGroup(message *tgbotapi.Message) (replyMessage *tgbotapi.MessageConfig, err error) {
	ctx := context.Background()
	users, err := storage.GetAllUsers(ctx)
	if err != nil {
		return nil, Error{InnerError: err,
			ReplyText: "查询时出错",
		}
	}

	var dtcPrices []string
	for _, user := range users {
		island, err := user.GetAnimalCrossingIsland(ctx)
		if err != nil || island == nil {
			continue
		}
		if time.Since(island.LastPrice.Date) > 12*time.Hour {
			continue
		}
		chatmember, err := tgbot.GetChatMember(tgbotapi.ChatConfigWithUser{ChatID: message.Chat.ID, UserID: user.ID})
		if err != nil || !(chatmember.IsMember() || chatmember.IsCreator() || chatmember.IsAdministrator()) {
			continue
		}
		if !strings.HasSuffix(island.Name, "岛") {
			island.Name += "岛"
		}
		dtcPrices = append(dtcPrices, fmt.Sprintf("%s的岛：%s 上的菜价：%d", user.Name, island.Name, island.LastPrice.Price))
	}

	return &tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: strings.Join(dtcPrices, "\n")},
		nil
}
