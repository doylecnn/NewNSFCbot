package chatbot

import (
	"context"
	"fmt"
	"strings"

	"cloud.google.com/go/firestore"
	"github.com/doylecnn/new-nsfc-bot/storage"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

//HandleCallbackQuery handle all CallbackQuery
func (c ChatBot) HandleCallbackQuery(query *tgbotapi.CallbackQuery) {
	if query.Data == "/queue" {
		if result, err := callbackQueryStartQueue(query); err != nil {
			logrus.Warn(err)
		} else {
			c.TgBotClient.AnswerCallbackQuery(*result)
		}
	} else if strings.HasPrefix(query.Data, "/join_") {
		if result, err := callbackQueryJoinQueue(query); err != nil {
			logrus.Warn(err)
		} else {
			c.TgBotClient.AnswerCallbackQuery(*result)
		}
	} else if strings.HasPrefix(query.Data, "/leave_") {
		if result, err := callbackQueryLeaveQueue(query); err != nil {
			logrus.Warn(err)
		} else {
			c.TgBotClient.AnswerCallbackQuery(*result)
		}
	} else if strings.HasPrefix(query.Data, "/next_") {
		if result, err := callbackQueryNextQueue(query); err != nil {
			logrus.Warn(err)
		} else {
			c.TgBotClient.AnswerCallbackQuery(*result)
		}
	} else if strings.HasPrefix(query.Data, "/dismiss_") {
		if result, err := callbackQueryDismissQueue(query); err != nil {
			logrus.Warn(err)
		} else {
			c.TgBotClient.AnswerCallbackQuery(*result)
		}
	}
}

func callbackQueryStartQueue(query *tgbotapi.CallbackQuery) (callbackConfig *tgbotapi.CallbackConfig, err error) {
	_, err = tgbot.Send(&tgbotapi.MessageConfig{
		BaseChat: tgbotapi.BaseChat{
			ChatID: int64(query.From.ID),
		},
		Text: "请使用指令 /queue [密码] [同时最大客人数] 创建队列\n创建完成后请分享到其它聊天中邀请大家排队。"})
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{"uid": query.From.ID,
			"msgID": query.Message.MessageID}).Error("send message failed")
	}
	return &tgbotapi.CallbackConfig{
		CallbackQueryID: query.ID,
		Text:            "请与 @NS_FC_bot 私聊",
		ShowAlert:       true,
	}, nil
}

func callbackQueryLeaveQueue(query *tgbotapi.CallbackQuery) (callbackConfig *tgbotapi.CallbackConfig, err error) {
	queueID := query.Data[7:]
	uid := query.From.ID
	ctx := context.Background()
	client, err := firestore.NewClient(ctx, _projectID)
	if err != nil {
		logrus.WithError(err).Error("create firestore client failed")
		return &tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "failed",
			ShowAlert:       true,
		}, nil
	}
	queue, err := storage.GetOnboardQueue(ctx, client, queueID)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return &tgbotapi.CallbackConfig{
				CallbackQueryID: query.ID,
				Text:            "success",
				ShowAlert:       true,
			}, nil
		}
		logrus.WithError(err).Error("query queue failed")
		return &tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "failed",
			ShowAlert:       true,
		}, nil
	}
	if err = queue.Remove(ctx, client, int64(uid)); err != nil {
		logrus.WithError(err).Error("remove queue failed")
		return &tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "failed",
			ShowAlert:       true,
		}, nil
	}
	_, err = tgbot.Send(tgbotapi.EditMessageTextConfig{
		BaseEdit: tgbotapi.BaseEdit{
			ChatID:    int64(uid),
			MessageID: query.Message.MessageID},
		Text: "已离开队列"})
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{"uid": uid,
			"msgID": query.Message.MessageID}).Error("edit message failed")
	}
	return &tgbotapi.CallbackConfig{
		CallbackQueryID: query.ID,
		Text:            "成功离开队列",
		ShowAlert:       true,
	}, nil
}

func callbackQueryNextQueue(query *tgbotapi.CallbackQuery) (callbackConfig *tgbotapi.CallbackConfig, err error) {
	queueID := query.Data[6:]
	uid := query.From.ID
	ctx := context.Background()
	island, err := storage.GetAnimalCrossingIslandByUserID(ctx, uid)
	if err != nil {
		logrus.WithError(err).Error("query island failed")
		return &tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "failed",
			ShowAlert:       true,
		}, nil
	}

	if len(island.OnBoardQueueID) == 0 {
		_, err = tgbot.Send(tgbotapi.EditMessageTextConfig{
			BaseEdit: tgbotapi.BaseEdit{
				ChatID:    int64(uid),
				MessageID: query.Message.MessageID},
			Text: "队列已解散"})
		return &tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "解散成功",
			ShowAlert:       true,
		}, nil
	}
	if island.OnBoardQueueID != queueID {
		logrus.WithError(err).Error("not island owner")
		return &tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "你不能操作别人的队列",
			ShowAlert:       true,
		}, nil
	}
	queue, err := island.GetOnboardQueue(ctx)
	if err != nil {
		logrus.WithError(err).Error("query queue failed")
		return &tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "failed",
			ShowAlert:       true,
		}, nil
	}
	client, err := firestore.NewClient(ctx, _projectID)
	if err != nil {
		logrus.WithError(err).Error("create firestore client failed")
		return &tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "failed",
			ShowAlert:       true,
		}, nil
	}
	if chatID, err := queue.Next(ctx, client); err != nil {
		if err.Error() == "queue is empty" {
			return &tgbotapi.CallbackConfig{
				CallbackQueryID: query.ID,
				Text:            "并没有下一位在等候的访客……",
				ShowAlert:       true,
			}, nil
		}
		logrus.WithError(err).Error("append queue failed")
		return &tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "failed",
			ShowAlert:       true,
		}, nil
	} else {
		_, err = tgbot.Send(&tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID: chatID,
			},
			Text: "轮到你了！如果不能前往，请务必和岛主联系！",
		})
		if err != nil {
			logrus.WithError(err).Error("notify next failed")
			return &tgbotapi.CallbackConfig{
				CallbackQueryID: query.ID,
				Text:            "failed",
				ShowAlert:       true,
			}, nil
		}
	}

	return &tgbotapi.CallbackConfig{
		CallbackQueryID: query.ID,
		Text:            "成功通知下一位访客",
		ShowAlert:       true,
	}, nil
}

func callbackQueryJoinQueue(query *tgbotapi.CallbackQuery) (callbackConfig *tgbotapi.CallbackConfig, err error) {
	queueID := query.Data[6:]
	uid := query.From.ID
	ctx := context.Background()
	client, err := firestore.NewClient(ctx, _projectID)
	if err != nil {
		logrus.WithError(err).Error("create firestore client failed")
		return &tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "failed",
			ShowAlert:       true,
		}, nil
	}
	queue, err := storage.GetOnboardQueue(ctx, client, queueID)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return &tgbotapi.CallbackConfig{
				CallbackQueryID: query.ID,
				Text:            "队列已取消",
				ShowAlert:       true,
			}, nil
		}
		logrus.WithError(err).Error("query queue failed")
		return &tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "failed",
			ShowAlert:       true,
		}, nil
	}
	if err = queue.Append(ctx, client, int64(uid)); err != nil {
		logrus.WithError(err).Error("append queue failed")
		return &tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "failed",
			ShowAlert:       true,
		}, nil
	}
	var shareBtn = tgbotapi.NewInlineKeyboardButtonData("离开队列："+queue.Name, "/leave_"+queue.ID)
	var replyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(shareBtn))
	tgbot.Send(&tgbotapi.MessageConfig{
		BaseChat: tgbotapi.BaseChat{
			ChatID:      int64(uid),
			ReplyMarkup: replyMarkup,
		},
		Text: fmt.Sprintf("正在队列：%s 中排队", queue.Name),
	})
	return &tgbotapi.CallbackConfig{
		CallbackQueryID: query.ID,
		Text:            "加入成功",
		ShowAlert:       true,
	}, nil
}

func callbackQueryDismissQueue(query *tgbotapi.CallbackQuery) (callbackConfig *tgbotapi.CallbackConfig, err error) {
	queueID := query.Data[9:]
	uid := query.From.ID
	ctx := context.Background()
	island, err := storage.GetAnimalCrossingIslandByUserID(ctx, uid)
	if err != nil {
		logrus.WithError(err).Error("query island failed")
		return
	}
	if len(island.OnBoardQueueID) == 0 {
		_, err = tgbot.Send(tgbotapi.EditMessageTextConfig{
			BaseEdit: tgbotapi.BaseEdit{
				ChatID:    int64(uid),
				MessageID: query.Message.MessageID},
			Text: "队列已解散"})
		return &tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "解散成功",
			ShowAlert:       true,
		}, nil
	}
	if island.OnBoardQueueID != queueID {
		logrus.WithError(err).Error("not island owner")
		return &tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "你不能操作别人的队列",
			ShowAlert:       true,
		}, nil
	}
	queue, err := island.ClearOldOnboardQueue(ctx)
	if err != nil {
		logrus.WithError(err).Error("ClearOldOnboardQueue failed")
		return
	}
	for _, replyMsg := range notifyQueueDissmised(queue) {
		tgbot.Send(replyMsg)
	}
	_, err = tgbot.Send(tgbotapi.EditMessageTextConfig{
		BaseEdit: tgbotapi.BaseEdit{
			ChatID:    int64(uid),
			MessageID: query.Message.MessageID},
		Text: "队列已解散"})
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{"uid": uid,
			"msgID": query.Message.MessageID}).Error("edit message failed")
	}
	return &tgbotapi.CallbackConfig{
		CallbackQueryID: query.ID,
		Text:            "解散成功",
		ShowAlert:       true,
	}, nil
}
