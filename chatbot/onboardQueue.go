package chatbot

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"cloud.google.com/go/firestore"
	"github.com/doylecnn/new-nsfc-bot/storage"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

func cmdOpenIslandQueue(message *tgbotapi.Message) (replyMessage []*tgbotapi.MessageConfig, err error) {
	if !message.Chat.IsPrivate() {
		var btn = tgbotapi.NewInlineKeyboardButtonURL("开排队点此并输入/queue", "https://t.me/ns_fc_bot")
		var replyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(btn))
		return []*tgbotapi.MessageConfig{{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true,
					ReplyMarkup:         replyMarkup,
				},
				Text:      "请私聊bot 后使用 */queue* \\[密码\\] [同时在线人数（<\\=7）] 开始创建上岛队列。\n您必须已经使用 */addisand* 登记过岛屿。\n创建队列后，密码不会直接公开。\n创建队列是队列创建成功后，bot 会记录您的岛屿为开放状态。",
				ParseMode: "MarkdownV2",
			}},
			nil
	}
	ctx := context.Background()
	island, err := storage.GetAnimalCrossingIslandByUserID(ctx, message.From.ID)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, Error{InnerError: err,
				ReplyText: "没有找到您的岛屿信息狸，如未记录，请先使用/addisland 登记岛屿信息狸。",
			}
		}
		return nil, Error{InnerError: err,
			ReplyText: "查询记录时出错狸",
		}
	}
	if len(island.OnBoardQueueID) != 0 {
		queue, _ := island.GetOnboardQueue(ctx)
		if queue.Dismissed {
			_, err = island.ClearOldOnboardQueue(ctx)
		} else {
			return nil, Error{InnerError: err,
				ReplyText: "请先 /dismiss 解散您当前已发起的队列",
			}
		}
	}

	argstr := strings.TrimSpace(message.CommandArguments())
	if len(argstr) == 0 {
		return nil, Error{InnerError: err,
			ReplyText: "/queue 需要两个参数，第一个为开岛密码，第二个为最大同时在线人数。使用空格分割。",
		}
	}
	args := strings.Split(argstr, " ")
	if len(args) != 2 {
		return nil, Error{InnerError: err,
			ReplyText: "/queue 需要两个参数，第一个为开岛密码，第二个为最大同时在线人数。使用空格分割。",
		}
	}
	password := args[0]
	if len(password) != 5 {
		return nil, Error{InnerError: err,
			ReplyText: "动森岛屿密码必须是5位数字字母",
		}
	}
	maxGuestCount, err := strconv.Atoi(args[1])
	if err != nil || maxGuestCount > 7 || maxGuestCount < 1 {
		return nil, Error{InnerError: err,
			ReplyText: "最大同时在岛客人数取值范围：[1, 7]",
		}
	}

	queue, err := island.CreateOnboardQueue(ctx, password, maxGuestCount)
	if err != nil {
		logrus.WithError(err).Error("创建队列时出错")
		return []*tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				DisableNotification: false,
			},
			Text: fmt.Sprintf("创建队列 %s 时出错狸，已通知bot 管理员。", queue.Name)},
			{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              int64(botAdminID),
					DisableNotification: false,
				},
				Text: fmt.Sprintf("创建队列 %s 时出错，error：%v", queue.Name, err)},
		}, nil
	}
	var shareBtn = tgbotapi.NewInlineKeyboardButtonSwitch("分享队列："+island.Name, "/share_"+queue.ID)
	var dismissBtn = tgbotapi.NewInlineKeyboardButtonData("解散队列", "/dismiss_"+queue.ID)
	var nextBtn = tgbotapi.NewInlineKeyboardButtonData("有请下一位", "/next_"+queue.ID)
	var replyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(shareBtn, dismissBtn), tgbotapi.NewInlineKeyboardRow(nextBtn))
	var qid = strings.ReplaceAll(queue.ID, "-", "\\-")
	qid = strings.ReplaceAll(qid, "_", "\\_")
	qid = strings.ReplaceAll(qid, "*", "\\*")
	var replyText = fmt.Sprintf("队列已创建成功，队列ID：*%s*\n请使用分享按钮选择要分享排队的群/朋友\n/next 指令将向队列中下一顺位的朋友发送登岛密码\n/dismiss 立即解散队列", qid)

	return []*tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true,
				ReplyMarkup:         replyMarkup,
			},
			Text:      replyText,
			ParseMode: "MarkdownV2",
		}},
		nil
}

func cmdDismissIslandQueue(message *tgbotapi.Message) (replyMessage []*tgbotapi.MessageConfig, err error) {
	if !message.Chat.IsPrivate() {
		var btn = tgbotapi.NewInlineKeyboardButtonURL("请私聊我", "https://t.me/ns_fc_bot")
		var replyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(btn))
		return []*tgbotapi.MessageConfig{{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true,
					ReplyMarkup:         replyMarkup,
				},
				Text:      "请私聊bot 后使用 */dismiss* 解散队列",
				ParseMode: "MarkdownV2",
			}},
			nil
	}

	ctx := context.Background()
	island, err := storage.GetAnimalCrossingIslandByUserID(ctx, message.From.ID)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, Error{InnerError: err,
				ReplyText: "没有找到您的岛屿信息狸，如未记录，请先使用/addisland 登记岛屿信息狸。",
			}
		}
		return nil, Error{InnerError: err,
			ReplyText: "查询岛屿记录时出错狸",
		}
	}
	if len(island.OnBoardQueueID) == 0 {
		return nil, Error{InnerError: err,
			ReplyText: "当前没有创建登岛队列狸。如有需要请使用 /queue [密码] [最大同时登岛客人数] 创建队列",
		}
	}
	queue, err := island.ClearOldOnboardQueue(ctx)
	if err != nil {
		return []*tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				DisableNotification: false,
			},
			Text: fmt.Sprintf("清理队列 %s 时出错，已通知bot 管理员。error：%v", queue.Name, err)},
			{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              int64(botAdminID),
					DisableNotification: false,
				},
				Text: fmt.Sprintf("清理队列 %s 时出错，error：%v", queue.Name, err)},
		}, nil
	}
	replyMessage = append(replyMessage, notifyQueueDissmised(queue)...)
	replyMessage = append(replyMessage, &tgbotapi.MessageConfig{
		BaseChat: tgbotapi.BaseChat{
			ChatID:              message.Chat.ID,
			DisableNotification: false,
		},
		Text:      fmt.Sprintf("排队前来*%s*的航程已被取消，正在通知旅客狸", queue.Name),
		ParseMode: "MarkdownV2",
	})
	return
}

func notifyQueueDissmised(queue *storage.OnboardQueue) (replyMessage []*tgbotapi.MessageConfig) {
	for _, cid := range queue.Queue {
		replyMessage = append(replyMessage, &tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              cid,
				DisableNotification: false,
			},
			Text: fmt.Sprintf("排队前往*%s*的旅客请注意，本次航程已被取消狸", queue.Name),
		})
	}
	return
}

func cmdJoinIslandQueue(message *tgbotapi.Message) (replyMessage []*tgbotapi.MessageConfig, err error) {
	if !message.Chat.IsPrivate() {
		var btn = tgbotapi.NewInlineKeyboardButtonURL("请私聊我", "https://t.me/ns_fc_bot")
		var replyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(btn))
		return []*tgbotapi.MessageConfig{{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true,
					ReplyMarkup:         replyMarkup,
				},
				Text:      "请私聊bot 后使用 */join [队列ID]* 来加入队列",
				ParseMode: "MarkdownV2",
			}},
			nil
	}
	argstr := strings.TrimSpace(message.CommandArguments())
	if len(argstr) == 0 {
		return nil, Error{InnerError: nil, ReplyText: "请使用 /join [队列ID] 来加入队列"}
	}

	ctx := context.Background()
	client, err := firestore.NewClient(ctx, _projectID)
	if err != nil {
		return []*tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				DisableNotification: false,
			},
			Text: "加入队列时出错，已通知bot 管理员。"},
			{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              int64(botAdminID),
					DisableNotification: false,
				},
				Text: fmt.Sprintf("创建 firestore.Client 时出错，error：%v", err)},
		}, nil
	}
	queue, err := storage.GetOnboardQueue(ctx, client, argstr)
	if err != nil {
		return []*tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				DisableNotification: false,
			},
			Text: "加入队列时出错，已通知bot 管理员。"},
			{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              int64(botAdminID),
					DisableNotification: false,
				},
				Text: fmt.Sprintf("查找队列ID %s 时出错，error：%v", argstr, err)},
		}, nil
	}
	err = queue.Append(ctx, client, message.Chat.ID)
	if err != nil {
		if err.Error() == "queue has been dismissed" {
			return []*tgbotapi.MessageConfig{{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					DisableNotification: false,
				},
				Text: fmt.Sprintf("前往 %s 的航程已经取消，无法加入。", queue.Name)},
			}, nil
		}
		if err.Error() == "already in this queue" {
			return []*tgbotapi.MessageConfig{{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					DisableNotification: false,
				},
				Text: fmt.Sprintf("您已经在准备前往 %s 的航程中排队了，无法重新加入。", queue.Name)},
			}, nil
		}
		return []*tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				DisableNotification: false,
			},
			Text: fmt.Sprintf("加入队列 %s 时出错，已通知bot 管理员。", queue.Name)},
			{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              int64(botAdminID),
					DisableNotification: false,
				},
				Text: fmt.Sprintf("加入队列ID %s ：%s 时出错，error：%v", queue.ID, queue.Name, err)},
		}, nil
	}
	return []*tgbotapi.MessageConfig{{
		BaseChat: tgbotapi.BaseChat{
			ChatID:              message.Chat.ID,
			DisableNotification: false,
		},
		Text: fmt.Sprintf("成功加入队列ID %s\n请耐心您的等待前往 %s 的航程起飞", queue.ID, queue.Name)}}, nil
}

func cmdLeaveIslandQueue(message *tgbotapi.Message) (replyMessage []*tgbotapi.MessageConfig, err error) {
	if !message.Chat.IsPrivate() {
		var btn = tgbotapi.NewInlineKeyboardButtonURL("请私聊我", "https://t.me/ns_fc_bot")
		var replyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(btn))
		return []*tgbotapi.MessageConfig{{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true,
					ReplyMarkup:         replyMarkup,
				},
				Text:      "请私聊bot 后使用 */leave [队列ID]* 来加立即队列",
				ParseMode: "MarkdownV2",
			}},
			nil
	}
	argstr := strings.TrimSpace(message.CommandArguments())
	if len(argstr) == 0 {
		return nil, Error{InnerError: nil, ReplyText: "请使用 /leave [队列ID] 来离开队列"}
	}

	ctx := context.Background()
	client, err := firestore.NewClient(ctx, _projectID)
	if err != nil {
		return []*tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				DisableNotification: false,
			},
			Text: "离开队列时出错，已通知bot 管理员。"},
			{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              int64(botAdminID),
					DisableNotification: false,
				},
				Text: fmt.Sprintf("创建 firestore.Client 时出错，error：%v", err)},
		}, nil
	}
	queue, err := storage.GetOnboardQueue(ctx, client, argstr)
	if err != nil {
		return []*tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				DisableNotification: false,
			},
			Text: "离开队列时出错，已通知bot 管理员。"},
			{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              int64(botAdminID),
					DisableNotification: false,
				},
				Text: fmt.Sprintf("查找队列ID %s 时出错，error：%v", argstr, err)},
		}, nil
	}
	err = queue.Remove(ctx, client, message.Chat.ID)
	if err != nil {
		if err.Error() == "not join in this queue" || err.Error() == "queue has been dismissed" {
			err = nil
		} else {
			return []*tgbotapi.MessageConfig{{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					DisableNotification: false,
				},
				Text: fmt.Sprintf("离开队列 %s 时出错，已通知bot 管理员。", queue.Name)},
				{
					BaseChat: tgbotapi.BaseChat{
						ChatID:              int64(botAdminID),
						DisableNotification: false,
					},
					Text: fmt.Sprintf("离开队列ID %s ：%s 时出错，error：%v", queue.ID, queue.Name, err)},
			}, nil
		}
	}
	return []*tgbotapi.MessageConfig{{
		BaseChat: tgbotapi.BaseChat{
			ChatID:              message.Chat.ID,
			DisableNotification: false,
		},
		Text: fmt.Sprintf("成功离开队列ID %s\n您已取消前往 %s 的航程", queue.ID, queue.Name)}}, nil
}

func cmdNextIslandQueue(message *tgbotapi.Message) (replyMessage []*tgbotapi.MessageConfig, err error) {
	if !message.Chat.IsPrivate() {
		var btn = tgbotapi.NewInlineKeyboardButtonURL("请私聊我", "https://t.me/ns_fc_bot")
		var replyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(btn))
		return []*tgbotapi.MessageConfig{{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true,
					ReplyMarkup:         replyMarkup,
				},
				Text:      "请私聊bot 后使用 */next* 通知下一位访客",
				ParseMode: "MarkdownV2",
			}},
			nil
	}

	ctx := context.Background()
	island, err := storage.GetAnimalCrossingIslandByUserID(ctx, message.From.ID)
	if err != nil {
		return []*tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				DisableNotification: false,
			},
			Text: "通知访客时出错，已通知bot 管理员。"},
			{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              int64(botAdminID),
					DisableNotification: false,
				},
				Text: fmt.Sprintf("查找岛主%d岛屿时出错，error：%v", message.From.ID, err)},
		}, nil
	}
	queue, err := island.GetOnboardQueue(ctx)
	if err != nil {
		return []*tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				DisableNotification: false,
			},
			Text: "通知访客时出错，已通知bot 管理员。"},
			{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              int64(botAdminID),
					DisableNotification: false,
				},
				Text: fmt.Sprintf("查找岛主 %d 队列 %s 时出错，error：%v", message.From.ID, island.OnBoardQueueID, err)},
		}, nil
	}

	client, err := firestore.NewClient(ctx, _projectID)
	if err != nil {
		return []*tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				DisableNotification: false,
			},
			Text: "通知访客时出错，已通知bot 管理员。"},
			{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              int64(botAdminID),
					DisableNotification: false,
				},
				Text: fmt.Sprintf("创建 firestore.Client 时出错，error：%v", err)},
		}, nil
	}
	chatID, err := queue.Next(ctx, client)
	if err != nil {
		if err.Error() == "queue is empty" {
			return []*tgbotapi.MessageConfig{{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					DisableNotification: false,
				},
				Text: "访客队列已清空，当前没有等待中的访客了。请使用 /dismiss 关闭队列。"}}, nil
		}
		return []*tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				DisableNotification: false,
			},
			Text: "通知访客时出错，已通知bot 管理员。"},
			{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              int64(botAdminID),
					DisableNotification: false,
				},
				Text: fmt.Sprintf("通知岛主 %d 队列 %s 访客 %d 时出错，error：%v", message.From.ID, island.OnBoardQueueID, chatID, err)},
		}, nil
	}
	return []*tgbotapi.MessageConfig{{
		BaseChat: tgbotapi.BaseChat{
			ChatID:              message.Chat.ID,
			DisableNotification: false,
		},
		Text: "已通知下一位访客"},
		{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              chatID,
				DisableNotification: false,
			},
			Text: fmt.Sprintf("您前往 %s 的航程即将触发，密码为：%s。祝你平安归来。\n如因有事不能前往，请务必通知岛主。", queue.Name, queue.Password)}}, nil
}
