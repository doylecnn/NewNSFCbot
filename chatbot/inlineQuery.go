package chatbot

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/doylecnn/new-nsfc-bot/storage"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// HandleInlineQuery handle all inline query
func (c ChatBot) HandleInlineQuery(inlineQuery *tgbotapi.InlineQuery) {
	if inlineQuery.Query == "myfc" {
		if result, err := inlineQueryMyFC(inlineQuery); err != nil {
			_logger.Warn().Err(err).Send()
		} else {
			c.TgBotClient.AnswerInlineQuery(*result)
		}
	} else if inlineQuery.Query == "myisland" {
		if result, err := inlineQueryMyIsland(inlineQuery); err != nil {
			_logger.Warn().Err(err).Send()
		} else {
			c.TgBotClient.AnswerInlineQuery(*result)
		}
	} else if strings.HasPrefix(inlineQuery.Query, "/share_") {
		if result, err := inlineQueryShareQueue(inlineQuery); err != nil {
			_logger.Warn().Err(err).Send()
		} else {
			_, err := c.TgBotClient.AnswerInlineQuery(*result)
			if err != nil {
				_logger.Error().Err(err).Msg("answer share queue inline query failed")
			}
		}
	}
}

func inlineQueryShareQueue(query *tgbotapi.InlineQuery) (rst *tgbotapi.InlineConfig, err error) {
	uid := query.From.ID
	queueID := query.Query[7:]
	ctx := context.Background()
	island, _, err := storage.GetAnimalCrossingIslandByUserID(ctx, uid)
	if err != nil {
		return
	}
	if island.OnBoardQueueID != queueID {
		return nil, errors.New("not island owner")
	}
	r := tgbotapi.NewInlineQueryResultArticle(query.ID, fmt.Sprintf("分享前往您的岛屿 %s 的队列", island.Name), fmt.Sprintf("邀请您加入前往 %s 的队列\n本次信息：%s\n点击“加入队列”按钮后，请再点击“start”按钮", island.Name, island.Info))
	var joinBtn = tgbotapi.NewInlineKeyboardButtonURL("加入队列", "https://t.me/NS_FC_bot?start=join_"+queueID)
	var replyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(joinBtn))
	r.ReplyMarkup = &replyMarkup
	return &tgbotapi.InlineConfig{
		InlineQueryID: query.ID,
		Results:       []interface{}{r},
		IsPersonal:    true,
	}, nil
}
