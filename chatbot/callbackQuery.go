package chatbot

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/doylecnn/new-nsfc-bot/storage"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

//HandleCallbackQuery handle all CallbackQuery
func (c ChatBot) HandleCallbackQuery(query *tgbotapi.CallbackQuery) {
	var err error
	var result tgbotapi.CallbackConfig
	var processed = false
	if strings.HasPrefix(query.Data, "/back/") {
		processed = true
		result, err = callbackQueryBack(query)
	} else if query.Data == "/cancel" {
		processed = true
		result, err = callbackQueryCancel(query)
	} else if query.Data == "/queue" {
		processed = true
		result, err = callbackQueryStartQueue(query)
	} else if strings.HasPrefix(query.Data, "/updatepassword_") {
		processed = true
		result, err = callbackQueryUpdateQueuePassword(query)
	} else if strings.HasPrefix(query.Data, "/showqueuemember_") {
		processed = true
		result, err = callbackQueryShowQueueMembers(query)
	} else if strings.HasPrefix(query.Data, "/showqueueinfo_") {
		processed = true
		result, err = callbackQueryShowQueueInfo(query)
	} else if strings.HasPrefix(query.Data, "/join_") {
		processed = true
		result, err = callbackQueryJoinQueue(query)
	} else if strings.HasPrefix(query.Data, "/position_") {
		processed = true
		result, err = callbackQueryGetPositionInQueue(query)
	} else if strings.HasPrefix(query.Data, "/leave_") {
		processed = true
		result, err = callbackQueryLeaveQueue(query)
	} else if strings.HasPrefix(query.Data, "/next_") {
		processed = true
		result, err = callbackQueryNextQueue(query)
	} else if strings.HasPrefix(query.Data, "/coming_") {
		processed = true
		result, err = callbackQueryComing(query)
	} else if strings.HasPrefix(query.Data, "/done_") || strings.HasPrefix(query.Data, "/sorry_") {
		processed = true
		result, err = callbackQueryDoneOrSorry(query)
	} else if strings.HasPrefix(query.Data, "/dismiss_") {
		processed = true
		result, err = callbackQueryDismissQueue(query)
	} else if strings.HasPrefix(query.Data, "/manageFriendCodes") {
		processed = true
		result, err = callbackQueryManageFriendCodes(query)
	} else if strings.HasPrefix(query.Data, "/delFC_") {
		processed = true
		result, err = callbackQueryDeleteFriendCode(query)
	}
	if processed {
		if err != nil {
			if err.Error() != "no_alert" {
				logrus.Warn(err)
			}
		} else {
			c.TgBotClient.AnswerCallbackQuery(result)
		}
	}
}

func callbackQueryBack(query *tgbotapi.CallbackQuery) (callbackConfig tgbotapi.CallbackConfig, err error) {
	cmdargstr := query.Data[5:]
	_, err = tgbot.DeleteMessage(tgbotapi.NewDeleteMessage(int64(query.From.ID), query.Message.MessageID))
	if err != nil {
		logrus.WithError(err).Error("back failed")
		return tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "failed",
			ShowAlert:       false,
		}, nil
	}
	query.Data = cmdargstr
	if strings.HasPrefix(query.Data, "/manageFriendCodes") {
		callbackConfig, err = callbackQueryManageFriendCodes(query)
	} else {
		err = errors.New("no_alert")
	}
	return
}

func callbackQueryCancel(query *tgbotapi.CallbackQuery) (callbackConfig tgbotapi.CallbackConfig, err error) {
	_, err = tgbot.DeleteMessage(tgbotapi.NewDeleteMessage(int64(query.From.ID), query.Message.MessageID))
	if err != nil {
		logrus.WithError(err).Error("cancel failed")
		return tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "failed",
			ShowAlert:       false,
		}, nil
	}
	return tgbotapi.CallbackConfig{
		CallbackQueryID: query.ID,
		Text:            "已取消",
		ShowAlert:       false,
	}, nil
}

func callbackQueryStartQueue(query *tgbotapi.CallbackQuery) (callbackConfig tgbotapi.CallbackConfig, err error) {
	_, err = tgbot.Send(&tgbotapi.MessageConfig{
		BaseChat: tgbotapi.BaseChat{
			ChatID: int64(query.From.ID),
		},
		Text: "请使用指令 /queue [密码] 创建队列\n创建完成后请分享到其它聊天中邀请大家排队。"})
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{"uid": query.From.ID,
			"msgID": query.Message.MessageID}).Error("send message failed")
	}
	return tgbotapi.CallbackConfig{
		CallbackQueryID: query.ID,
		Text:            "请与 @NS_FC_bot 私聊",
		ShowAlert:       false,
	}, nil
}

func callbackQueryUpdateQueuePassword(query *tgbotapi.CallbackQuery) (callbackConfig tgbotapi.CallbackConfig, err error) {
	queueID := query.Data[16:]
	uid := query.From.ID
	ctx := context.Background()
	island, err := storage.GetAnimalCrossingIslandByUserID(ctx, uid)
	if err != nil {
		logrus.WithError(err).Error("query island failed")
		return tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "failed",
			ShowAlert:       false,
		}, nil
	}

	if island.OnBoardQueueID != queueID {
		logrus.WithError(err).Error("not island owner")
		return tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "你不能操作别人的队列",
			ShowAlert:       false,
		}, nil
	}
	queue, err := island.GetOnboardQueue(ctx)
	if err != nil {
		logrus.WithError(err).Error("query queue failed")
		return tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "failed",
			ShowAlert:       false,
		}, nil
	}
	if queue != nil && !queue.Dismissed {
		_, err = tgbot.Send(&tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:      int64(query.From.ID),
				ReplyMarkup: tgbotapi.ForceReply{ForceReply: true, Selective: true},
			},
			Text: "请输入新的密码",
		})
		if err != nil {
			return tgbotapi.CallbackConfig{
				CallbackQueryID: query.ID,
				Text:            "查找队列出错：" + err.Error(),
				ShowAlert:       false,
			}, nil
		}
		err = errors.New("no_alert")
		return
	}
	return tgbotapi.CallbackConfig{
		CallbackQueryID: query.ID,
		Text:            "failed",
		ShowAlert:       false,
	}, nil
}

func callbackQueryShowQueueMembers(query *tgbotapi.CallbackQuery) (callbackConfig tgbotapi.CallbackConfig, err error) {
	queueID := query.Data[17:]
	ctx := context.Background()
	client, err := firestore.NewClient(ctx, _projectID)
	if err != nil {
		logrus.WithError(err).Error("create new firestore client failed")
		return tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "failed",
			ShowAlert:       false,
		}, nil
	}
	queue, err := storage.GetOnboardQueue(ctx, client, queueID)
	if err != nil {
		logrus.WithError(err).Error("query queue failed")
		return tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "failed",
			ShowAlert:       false,
		}, nil
	}
	if queue == nil || queue.Dismissed {
		logrus.WithError(err).Error("query queue failed")
		return tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "此队列已取消",
			ShowAlert:       false,
		}, nil
	}
	if queue.Len() == 0 {
		return tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "目前队列为空",
			ShowAlert:       false,
		}, nil
	}
	var queueInfo []string
	for _, p := range queue.Queue {
		var name string
		if len(p.Name) > 0 {
			name = "@" + p.Name
		} else {
			name = fmt.Sprintf("tg://user?id=%d", p.UID)
		}
		queueInfo = append(queueInfo, name)
	}
	if len(queueInfo) == 0 {
		return tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "目前队列为空",
			ShowAlert:       false,
		}, nil
	}
	replyText := strings.Join(queueInfo, "\n")
	if len(replyText) == 0 {
		return tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "目前队列为空",
			ShowAlert:       false,
		}, nil
	}
	_, err = tgbot.Send(&tgbotapi.MessageConfig{
		BaseChat: tgbotapi.BaseChat{
			ChatID: int64(query.From.ID),
		},
		Text: replyText,
	})
	if err != nil {
		return tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "列出队列出错：" + err.Error(),
			ShowAlert:       false,
		}, nil
	}

	return tgbotapi.CallbackConfig{
		CallbackQueryID: query.ID,
		Text:            "已列出队列",
		ShowAlert:       false,
	}, nil
}

func callbackQueryLeaveQueue(query *tgbotapi.CallbackQuery) (callbackConfig tgbotapi.CallbackConfig, err error) {
	queueID := query.Data[7:]
	uid := query.From.ID
	username := query.From.UserName
	if len(username) == 0 {
		username = query.From.FirstName
	}
	ctx := context.Background()
	client, err := firestore.NewClient(ctx, _projectID)
	if err != nil {
		logrus.WithError(err).Error("create firestore client failed")
		return tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "failed",
			ShowAlert:       false,
		}, nil
	}
	queue, err := storage.GetOnboardQueue(ctx, client, queueID)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return tgbotapi.CallbackConfig{
				CallbackQueryID: query.ID,
				Text:            "success",
				ShowAlert:       false,
			}, nil
		}
		logrus.WithError(err).Error("query queue failed")
		return tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "failed",
			ShowAlert:       false,
		}, nil
	}
	if err = queue.Remove(ctx, client, int64(uid)); err != nil {
		logrus.WithError(err).Error("remove queue failed")
		return tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "failed",
			ShowAlert:       false,
		}, nil
	}
	_, err = tgbot.Send(tgbotapi.EditMessageTextConfig{
		BaseEdit: tgbotapi.BaseEdit{
			ChatID:    int64(uid),
			MessageID: query.Message.MessageID},
		Text: fmt.Sprintf("您已离开前往 %s 的队列", queue.Name)})
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{"uid": uid,
			"msgID": query.Message.MessageID}).Error("edit message failed")
	}
	sentMsg, err := tgbot.Send(tgbotapi.MessageConfig{
		BaseChat: tgbotapi.BaseChat{
			ChatID: queue.OwnerID},
		Text: fmt.Sprintf("%s 已离您的队列", username)})
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{"uid": uid,
			"msgID": query.Message.MessageID}).Error("send leave message failed")
	} else {
		go func() {
			time.Sleep(30 * time.Second)
			_, err = tgbot.DeleteMessage(tgbotapi.NewDeleteMessage(sentMsg.Chat.ID, sentMsg.MessageID))
			if err != nil {
				logrus.WithError(err).WithFields(logrus.Fields{"uid": uid,
					"msgID": query.Message.MessageID}).Error("delete leave message failed")
			}
		}()
	}
	return tgbotapi.CallbackConfig{
		CallbackQueryID: query.ID,
		Text:            "成功离开队列",
		ShowAlert:       false,
	}, nil
}

func callbackQueryNextQueue(query *tgbotapi.CallbackQuery) (callbackConfig tgbotapi.CallbackConfig, err error) {
	queueID := query.Data[6:]
	uid := query.From.ID
	ctx := context.Background()
	island, err := storage.GetAnimalCrossingIslandByUserID(ctx, uid)
	if err != nil {
		logrus.WithError(err).Error("query island failed")
		return tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "failed",
			ShowAlert:       false,
		}, nil
	}

	if len(island.OnBoardQueueID) == 0 {
		_, err = tgbot.Send(tgbotapi.EditMessageTextConfig{
			BaseEdit: tgbotapi.BaseEdit{
				ChatID:    int64(uid),
				MessageID: query.Message.MessageID},
			Text: "队列已解散"})
		return tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "解散成功",
			ShowAlert:       false,
		}, nil
	}
	if island.OnBoardQueueID != queueID {
		logrus.WithError(err).Error("not island owner")
		return tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "你不能操作别人的队列",
			ShowAlert:       false,
		}, nil
	}
	queue, err := island.GetOnboardQueue(ctx)
	if err != nil {
		logrus.WithError(err).Error("query queue failed")
		return tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "failed",
			ShowAlert:       false,
		}, nil
	}
	client, err := firestore.NewClient(ctx, _projectID)
	if err != nil {
		logrus.WithError(err).Error("create firestore client failed")
		return tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "failed",
			ShowAlert:       false,
		}, nil
	}
	chatID, err := queue.Next(ctx, client)
	if err != nil {
		if err.Error() == "queue is empty" {
			return tgbotapi.CallbackConfig{
				CallbackQueryID: query.ID,
				Text:            "并没有下一位在等候的访客……",
				ShowAlert:       false,
			}, nil
		}
		logrus.WithError(err).Error("append queue failed")
		return tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "failed",
			ShowAlert:       false,
		}, nil
	}

	var comingBtn = tgbotapi.NewInlineKeyboardButtonData("我来啦！"+queue.Name, "/coming_"+queue.ID)
	var sorryBtn = tgbotapi.NewInlineKeyboardButtonData("抱歉来不了", "/sorry_"+queue.ID)
	var doneBtn = tgbotapi.NewInlineKeyboardButtonData("我好了！", "/done_"+queue.ID)
	var replyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(comingBtn),
		tgbotapi.NewInlineKeyboardRow(doneBtn),
		tgbotapi.NewInlineKeyboardRow(sorryBtn))
	_, err = tgbot.Send(&tgbotapi.MessageConfig{
		BaseChat: tgbotapi.BaseChat{
			ChatID:      chatID,
			ReplyMarkup: replyMarkup,
		},
		Text:      fmt.Sprintf("轮到你了！\n密码：*%s*\n%s\n如果不能前往，请务必和岛主联系！", queue.Password, markdownSafe(queue.IslandInfo)),
		ParseMode: "MarkdownV2",
	})
	if err != nil {
		logrus.WithError(err).Error("notify next failed")
		return tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "failed",
			ShowAlert:       false,
		}, nil
	}

	return tgbotapi.CallbackConfig{
		CallbackQueryID: query.ID,
		Text:            "成功通知下一位访客",
		ShowAlert:       false,
	}, nil
}

func callbackQueryShowQueueInfo(query *tgbotapi.CallbackQuery) (callbackConfig tgbotapi.CallbackConfig, err error) {
	queueID := query.Data[15:]
	uid := int64(query.From.ID)
	username := query.From.UserName
	if len(username) == 0 {
		username = query.From.FirstName
	}
	ctx := context.Background()
	client, err := firestore.NewClient(ctx, _projectID)
	if err != nil {
		logrus.WithError(err).Error("create firestore client failed")
		return tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "failed",
			ShowAlert:       false,
		}, nil
	}
	queue, err := storage.GetOnboardQueue(ctx, client, queueID)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return tgbotapi.CallbackConfig{
				CallbackQueryID: query.ID,
				Text:            "队列已取消",
				ShowAlert:       false,
			}, nil
		}
		logrus.WithError(err).Error("query queue failed")
		return tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "failed",
			ShowAlert:       false,
		}, nil
	}
	t := queue.Len()
	l, err := queue.GetPosition(uid)
	if err != nil {
		return tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "您不在此队列中",
			ShowAlert:       false,
		}, nil
	}
	var leaveBtn = tgbotapi.NewInlineKeyboardButtonData("离开队列："+queue.Name, "/leave_"+queue.ID)
	var myPositionBtn = tgbotapi.NewInlineKeyboardButtonData("我的位置？", "/position_"+queue.ID)
	var listBtn = tgbotapi.NewInlineKeyboardButtonData("队列成员", "/showqueuemember_"+queue.ID)
	var replyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(myPositionBtn, listBtn, leaveBtn))
	tgbot.Send(&tgbotapi.MessageConfig{
		BaseChat: tgbotapi.BaseChat{
			ChatID:      uid,
			ReplyMarkup: replyMarkup,
		},
		Text: fmt.Sprintf("正在队列：%s 中排队，当前位置：%d/%d", queue.Name, l+1, t),
	})
	return tgbotapi.CallbackConfig{
		CallbackQueryID: query.ID,
		Text:            "success",
		ShowAlert:       false,
	}, nil
}

func callbackQueryJoinQueue(query *tgbotapi.CallbackQuery) (callbackConfig tgbotapi.CallbackConfig, err error) {
	queueID := query.Data[6:]
	uid := int64(query.From.ID)
	username := query.From.UserName
	if len(username) == 0 {
		username = query.From.FirstName
	}
	ctx := context.Background()
	client, err := firestore.NewClient(ctx, _projectID)
	if err != nil {
		logrus.WithError(err).Error("create firestore client failed")
		return tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "failed",
			ShowAlert:       false,
		}, nil
	}
	queue, err := storage.GetOnboardQueue(ctx, client, queueID)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return tgbotapi.CallbackConfig{
				CallbackQueryID: query.ID,
				Text:            "队列已取消",
				ShowAlert:       false,
			}, nil
		}
		logrus.WithError(err).Error("query queue failed")
		return tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "failed",
			ShowAlert:       false,
		}, nil
	}
	if queue.OwnerID == uid {
		return tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "自己不用排自己的队伍狸……",
			ShowAlert:       false,
		}, nil
	}
	if err = queue.Append(ctx, client, uid, username); err != nil {
		if err.Error() == "already in this queue" {
			return tgbotapi.CallbackConfig{
				CallbackQueryID: query.ID,
				Text:            "您已经加入了这个队列",
				ShowAlert:       false,
			}, nil
		}
		logrus.WithError(err).Error("append queue failed")
		return tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "failed",
			ShowAlert:       false,
		}, nil
	}
	t := queue.Len()
	l, err := queue.GetPosition(uid)
	if err != nil {
		t++
	}
	var leaveBtn = tgbotapi.NewInlineKeyboardButtonData("离开队列："+queue.Name, "/leave_"+queue.ID)
	var myPositionBtn = tgbotapi.NewInlineKeyboardButtonData("我的位置？", "/position_"+queue.ID)
	var listBtn = tgbotapi.NewInlineKeyboardButtonData("队列成员", "/showqueuemember_"+queue.ID)
	var replyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(myPositionBtn, listBtn, leaveBtn))
	tgbot.Send(&tgbotapi.MessageConfig{
		BaseChat: tgbotapi.BaseChat{
			ChatID:      uid,
			ReplyMarkup: replyMarkup,
		},
		Text: fmt.Sprintf("正在队列：%s 中排队，当前位置：%d/%d", queue.Name, l+1, t),
	})
	sentMsg, err := tgbot.Send(&tgbotapi.MessageConfig{
		BaseChat: tgbotapi.BaseChat{
			ChatID: queue.OwnerID,
		},
		Text: fmt.Sprintf("@%s 加入了队列", username),
	})
	if err != nil {
		logrus.WithError(err).Error("send msg failed")
	} else {
		go func() {
			time.Sleep(55 * time.Second)
			tgbot.DeleteMessage(tgbotapi.NewDeleteMessage(sentMsg.Chat.ID, sentMsg.MessageID))
		}()
	}
	return tgbotapi.CallbackConfig{
		CallbackQueryID: query.ID,
		Text:            "success",
		ShowAlert:       false,
	}, nil
}

func callbackQueryDismissQueue(query *tgbotapi.CallbackQuery) (callbackConfig tgbotapi.CallbackConfig, err error) {
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
		return tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "解散成功",
			ShowAlert:       false,
		}, nil
	}
	if island.OnBoardQueueID != queueID {
		logrus.WithError(err).Error("not island owner")
		return tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "你不能操作别人的队列",
			ShowAlert:       false,
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
	return tgbotapi.CallbackConfig{
		CallbackQueryID: query.ID,
		Text:            "解散成功",
		ShowAlert:       false,
	}, nil
}

func callbackQueryGetPositionInQueue(query *tgbotapi.CallbackQuery) (callbackConfig tgbotapi.CallbackConfig, err error) {
	queueID := query.Data[10:]
	uid := int64(query.From.ID)
	ctx := context.Background()
	client, err := firestore.NewClient(ctx, _projectID)
	if err != nil {
		logrus.WithError(err).Error("create firestore client failed")
		return tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "failed",
			ShowAlert:       false,
		}, nil
	}
	queue, err := storage.GetOnboardQueue(ctx, client, queueID)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return tgbotapi.CallbackConfig{
				CallbackQueryID: query.ID,
				Text:            "队列已取消",
				ShowAlert:       false,
			}, nil
		}
		logrus.WithError(err).Error("query queue failed")
		return tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "failed",
			ShowAlert:       false,
		}, nil
	}
	t := queue.Len()
	l, err := queue.GetPosition(uid)
	if err != nil {
		return tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "您不在此队列中",
			ShowAlert:       false,
		}, nil
	}
	var leaveBtn = tgbotapi.NewInlineKeyboardButtonData("离开队列："+queue.Name, "/leave_"+queue.ID)
	var myPositionBtn = tgbotapi.NewInlineKeyboardButtonData("我的位置？", "/position_"+queue.ID)
	var listBtn = tgbotapi.NewInlineKeyboardButtonData("队列成员", "/showqueuemember_"+queue.ID)
	var replyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(myPositionBtn, listBtn, leaveBtn))
	_, err = tgbot.Send(&tgbotapi.EditMessageTextConfig{
		BaseEdit: tgbotapi.BaseEdit{
			ChatID:      uid,
			MessageID:   query.Message.MessageID,
			ReplyMarkup: &replyMarkup,
		},
		Text: fmt.Sprintf("正在队列：%s 中排队，当前位置：%d/%d", queue.Name, l+1, t),
	})
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{"uid": uid,
			"msgID": query.Message.MessageID}).Error("edit message failed")
		return
	}
	err = errors.New("no_alert")
	return
}

func callbackQueryComing(query *tgbotapi.CallbackQuery) (callbackConfig tgbotapi.CallbackConfig, err error) {
	var name = query.From.UserName
	if len(name) == 0 {
		name = query.From.FirstName
	}

	action := "coming"
	queueID := query.Data[8:]
	replyText := fmt.Sprintf("@%s 兴奋地表示正在路上", name)

	uid := query.From.ID
	ctx := context.Background()
	client, err := firestore.NewClient(ctx, _projectID)
	if err != nil {
		logrus.WithError(err).Error("create firestore client failed")
		return tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "failed",
			ShowAlert:       false,
		}, nil
	}
	queue, err := storage.GetOnboardQueue(ctx, client, queueID)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return tgbotapi.CallbackConfig{
				CallbackQueryID: query.ID,
				Text:            "队列已取消",
				ShowAlert:       false,
			}, nil
		}
		logrus.WithError(err).Error("query queue failed")
		return tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "failed",
			ShowAlert:       false,
		}, nil
	}
	var sorryBtn = tgbotapi.NewInlineKeyboardButtonData("抱歉来不了", "/sorry_"+queue.ID)
	var doneBtn = tgbotapi.NewInlineKeyboardButtonData("我好了！", "/done_"+queue.ID)
	var replyMarkup1 = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(doneBtn),
		tgbotapi.NewInlineKeyboardRow(sorryBtn))
	_, err = tgbot.Send(&tgbotapi.EditMessageTextConfig{
		BaseEdit: tgbotapi.BaseEdit{
			ChatID:      query.Message.Chat.ID,
			MessageID:   query.Message.MessageID,
			ReplyMarkup: &replyMarkup1,
		},
		Text:      fmt.Sprintf("轮到你了！\n密码：*%s*\n如果不能前往，请务必和岛主联系！\n如果好了也请通知一下岛主。", queue.Password),
		ParseMode: "MarkdownV2",
	})
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{"uid": uid,
			"msgID":  query.Message.MessageID,
			"action": action}).Error("message send failed")
		err = nil
	}

	replyText += fmt.Sprintf("\n队列剩余：%d", queue.Len())

	var shareBtn = tgbotapi.NewInlineKeyboardButtonSwitch("分享队列："+queue.Name, "/share_"+queue.ID)
	var dismissBtn = tgbotapi.NewInlineKeyboardButtonData("解散队列", "/dismiss_"+queue.ID)
	var listBtn = tgbotapi.NewInlineKeyboardButtonData("查看队列", "/showqueuemember_"+queue.ID)
	var updatePasswordBtn = tgbotapi.NewInlineKeyboardButtonData("修改密码", "/updatepassword_"+queue.ID)
	var nextBtn = tgbotapi.NewInlineKeyboardButtonData("有请下一位", "/next_"+queue.ID)
	var replyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(shareBtn, dismissBtn),
		tgbotapi.NewInlineKeyboardRow(listBtn, updatePasswordBtn),
		tgbotapi.NewInlineKeyboardRow(nextBtn))

	_, err = tgbot.Send(&tgbotapi.MessageConfig{
		BaseChat: tgbotapi.BaseChat{
			ChatID:      queue.OwnerID,
			ReplyMarkup: replyMarkup,
		},
		Text: replyText,
	})
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{"uid": uid,
			"msgID":  query.Message.MessageID,
			"action": action}).Error("message send failed")
		return
	}
	err = errors.New("no_alert")
	return
}

func callbackQueryDoneOrSorry(query *tgbotapi.CallbackQuery) (callbackConfig tgbotapi.CallbackConfig, err error) {
	var name = query.From.UserName
	if len(name) == 0 {
		name = query.From.FirstName
	}

	var replyText string
	var queueID string
	var action string
	if strings.HasPrefix(query.Data, "/done_") {
		action = "done"
		queueID = query.Data[6:]
		replyText = fmt.Sprintf("@%s 满足地表示已经好了", name)
	} else if strings.HasPrefix(query.Data, "/sorry_") {
		action = "sorry"
		queueID = query.Data[7:]
		replyText = fmt.Sprintf("@%s 遗憾地表示自己现在有事无法前来", name)
	}

	uid := query.From.ID
	ctx := context.Background()
	client, err := firestore.NewClient(ctx, _projectID)
	if err != nil {
		logrus.WithError(err).Error("create firestore client failed")
		return tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "failed",
			ShowAlert:       false,
		}, nil
	}
	queue, err := storage.GetOnboardQueue(ctx, client, queueID)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return tgbotapi.CallbackConfig{
				CallbackQueryID: query.ID,
				Text:            "队列已取消",
				ShowAlert:       false,
			}, nil
		}
		logrus.WithError(err).Error("query queue failed")
		return tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "failed",
			ShowAlert:       false,
		}, nil
	}

	_, err = tgbot.Send(&tgbotapi.EditMessageTextConfig{
		BaseEdit: tgbotapi.BaseEdit{
			ChatID:    query.Message.Chat.ID,
			MessageID: query.Message.MessageID,
		},
		Text: "感谢本次使用",
	})
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{"uid": uid,
			"msgID":  query.Message.MessageID,
			"action": action}).Error("message send failed")
		err = nil
	}

	replyText += fmt.Sprintf("\n队列剩余：%d", queue.Len())

	var shareBtn = tgbotapi.NewInlineKeyboardButtonSwitch("分享队列："+queue.Name, "/share_"+queue.ID)
	var dismissBtn = tgbotapi.NewInlineKeyboardButtonData("解散队列", "/dismiss_"+queue.ID)
	var listBtn = tgbotapi.NewInlineKeyboardButtonData("查看队列", "/showqueuemember_"+queue.ID)
	var updatePasswordBtn = tgbotapi.NewInlineKeyboardButtonData("修改密码", "/updatepassword_"+queue.ID)
	var nextBtn = tgbotapi.NewInlineKeyboardButtonData("有请下一位", "/next_"+queue.ID)
	var replyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(shareBtn, dismissBtn),
		tgbotapi.NewInlineKeyboardRow(listBtn, updatePasswordBtn),
		tgbotapi.NewInlineKeyboardRow(nextBtn))

	_, err = tgbot.Send(&tgbotapi.MessageConfig{
		BaseChat: tgbotapi.BaseChat{
			ChatID:      queue.OwnerID,
			ReplyMarkup: replyMarkup,
		},
		Text: replyText,
	})
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{"uid": uid,
			"msgID":  query.Message.MessageID,
			"action": action}).Error("message send failed")
		return
	}
	err = errors.New("no_alert")
	return
}

func callbackQueryManageFriendCodes(query *tgbotapi.CallbackQuery) (callbackConfig tgbotapi.CallbackConfig, err error) {
	if query.Data == "/manageFriendCodes" {
		uid := query.From.ID
		ctx := context.Background()
		var u *storage.User
		u, err = storage.GetUser(ctx, uid, 0)
		if err != nil && status.Code(err) != codes.NotFound {
			tgbot.Send(&tgbotapi.MessageConfig{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              int64(uid),
					DisableNotification: true},
				Text: "查询FriendCode 记录时出错狸！"})
			err = errors.New("no_alert")
			return
		}
		if err != nil && status.Code(err) == codes.NotFound {
			logrus.Debug("没有找到用户记录")
			tgbot.Send(&tgbotapi.MessageConfig{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              int64(uid),
					DisableNotification: true},
				Text: "没有找到您的记录，请先使用 addfc 命令添加记录"})
			err = errors.New("no_alert")
			return
		}
		var rows [][]tgbotapi.InlineKeyboardButton
		for i, account := range u.NSAccounts {
			var manageFCBtn = tgbotapi.NewInlineKeyboardButtonData(account.String(), fmt.Sprintf("/manageFriendCodes_%d_%d", uid, i))
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(manageFCBtn))
		}
		var cancelBtn = tgbotapi.NewInlineKeyboardButtonData("取消", "/cancel")
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(cancelBtn))
		var replyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
		tgbot.Send(&tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:      int64(uid),
				ReplyMarkup: replyMarkup,
			},
			Text: "请点击要管理的 Friend Code\n/addfc [id]:[FC] 添加新的 Friend Code"})
	} else if strings.HasPrefix(query.Data, "/manageFriendCodes_") {
		args := strings.Split(query.Data[19:], "_")
		if len(args) != 2 {
			logrus.WithError(err).Error("manage fc wrong parameters")
			return tgbotapi.CallbackConfig{
				CallbackQueryID: query.ID,
				Text:            "wrong parameters",
				ShowAlert:       false,
			}, nil
		}
		var uid int
		uid, err = strconv.Atoi(args[0])
		if err != nil {
			logrus.WithError(err).Error("wrond uid")
			return tgbotapi.CallbackConfig{
				CallbackQueryID: query.ID,
				Text:            "wrong uid",
				ShowAlert:       false,
			}, nil
		}
		if uid != query.From.ID {
			logrus.WithError(err).Error("wrond owner")
			return tgbotapi.CallbackConfig{
				CallbackQueryID: query.ID,
				Text:            "not your friend code",
				ShowAlert:       false,
			}, nil
		}
		var idx int
		idx, err = strconv.Atoi(args[1])
		if err != nil {
			logrus.WithError(err).Error("wrond idx")
			return tgbotapi.CallbackConfig{
				CallbackQueryID: query.ID,
				Text:            "wrong index",
				ShowAlert:       false,
			}, nil
		}
		ctx := context.Background()
		var u *storage.User
		u, err = storage.GetUser(ctx, uid, 0)
		if err != nil && status.Code(err) != codes.NotFound {
			logrus.WithError(err).Error("query users Freind Code")
			return
		}
		if err != nil && status.Code(err) == codes.NotFound {
			logrus.WithError(err).Error("Freind Code not found")
			return
		}
		var delFCBtn = tgbotapi.NewInlineKeyboardButtonData("删 FriendCode", fmt.Sprintf("/delFC_%d_%d", uid, idx))
		var backBtn = tgbotapi.NewInlineKeyboardButtonData("返回", fmt.Sprintf("/back/manageFriendCodes"))
		var cancelBtn = tgbotapi.NewInlineKeyboardButtonData("取消", "/cancel")
		var replyMarkup = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(delFCBtn, backBtn, cancelBtn),
		)
		tgbot.Send(&tgbotapi.EditMessageTextConfig{
			BaseEdit: tgbotapi.BaseEdit{
				ChatID:      int64(uid),
				MessageID:   query.Message.MessageID,
				ReplyMarkup: &replyMarkup,
			},
			Text: fmt.Sprintf("要修改的FriendCode：%s", u.NSAccounts[idx].String())})
	}
	err = errors.New("no_alert")
	return
}

func callbackQueryDeleteFriendCode(query *tgbotapi.CallbackQuery) (callbackConfig tgbotapi.CallbackConfig, err error) {
	// /delFC_%uid_%idx
	args := strings.Split(query.Data[7:], "_")
	if len(args) != 2 {
		logrus.WithError(err).Error("manage fc wrong parameters")
		return tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "wrong parameters",
			ShowAlert:       false,
		}, nil
	}
	var uid int
	uid, err = strconv.Atoi(args[0])
	if err != nil {
		logrus.WithError(err).Error("wrond uid")
		return tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "wrong uid",
			ShowAlert:       false,
		}, nil
	}
	if uid != query.From.ID {
		logrus.WithError(err).Error("wrond owner")
		return tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "not your friend code",
			ShowAlert:       false,
		}, nil
	}
	var idx int
	idx, err = strconv.Atoi(args[1])
	if err != nil {
		logrus.WithError(err).Error("wrond idx")
		return tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "wrong index",
			ShowAlert:       false,
		}, nil
	}
	ctx := context.Background()
	var u *storage.User
	u, err = storage.GetUser(ctx, uid, 0)
	if err != nil && status.Code(err) != codes.NotFound {
		logrus.WithError(err).Error("query users Freind Code")
		return
	}
	if err != nil && status.Code(err) == codes.NotFound {
		logrus.WithError(err).Error("Freind Code not found")
		return
	}
	u, err = storage.GetUser(ctx, uid, 0)
	if err != nil && status.Code(err) != codes.NotFound {
		tgbot.Send(&tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              int64(uid),
				DisableNotification: true},
			Text: "查询FriendCode 记录时出错狸！"})
		err = errors.New("no_alert")
		return
	}
	if err != nil && status.Code(err) == codes.NotFound {
		logrus.Debug("没有找到用户记录")
		tgbot.Send(&tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              int64(uid),
				DisableNotification: true},
			Text: "没有找到您的记录，请先使用 addfc 命令添加记录"})
		err = errors.New("no_alert")
		return
	}
	if len(u.NSAccounts) == 0 {
		logrus.Debug("没有找到用户记录")
		tgbot.Send(&tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              int64(uid),
				DisableNotification: true},
			Text: "没有找到您的记录，请先使用 addfc 命令添加记录"})
		err = errors.New("no_alert")
		return
	}
	err = u.DeleteNSAccountByIndex(ctx, idx)
	if err != nil {
		logrus.WithError(err).Error("DeleteNSAccountByIndex idx")
		return tgbotapi.CallbackConfig{
			CallbackQueryID: query.ID,
			Text:            "failed",
			ShowAlert:       false,
		}, nil
	}
	var rows [][]tgbotapi.InlineKeyboardButton
	for i, account := range u.NSAccounts {
		var manageFCBtn = tgbotapi.NewInlineKeyboardButtonData(account.String(), fmt.Sprintf("/manageFriendCodes_%d_%d", uid, i))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(manageFCBtn))
	}
	var cancelBtn = tgbotapi.NewInlineKeyboardButtonData("取消", "/cancel")
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(cancelBtn))
	var replyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	tgbot.Send(&tgbotapi.EditMessageTextConfig{
		BaseEdit: tgbotapi.BaseEdit{
			ChatID:      int64(uid),
			MessageID:   query.Message.MessageID,
			ReplyMarkup: &replyMarkup,
		},
		Text: "请点击要管理的 Friend Code\n/addfc [id]:[FC] 添加新的 Friend Code"})
	err = errors.New("no_alert")
	return
}
