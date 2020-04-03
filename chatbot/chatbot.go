package chatbot

import (
	"bufio"
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/doylecnn/new-nsfc-bot/storage"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/sirupsen/logrus"
)

var (
	tgbot      *tgbotapi.BotAPI
	botAdminID int
	_projectID string
)

// ChatBot is chat bot
type ChatBot struct {
	TgBotClient *tgbotapi.BotAPI
	Route       Router
	ProjectID   string
}

// NewChatBot return new chat bot
func NewChatBot(token, appID, projectID, port string, adminID int) ChatBot {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		logrus.Fatalln(err)
	}
	//bot.Debug = true
	tgbot = bot

	var botHelpText = `/addfc 添加你的fc，可批量添加：/addfc id1:fc1;id2:fc2……
/myfc 显示自己的所有fc
/sfc 搜索你回复或at 的人的fc
/fc 与sfc 相同
/fclist 列出本群所有人的fc 列表
/whois name 查找NSAccount/Island是 name 的用户
/addisland 添加你的动森岛：/addisland 岛名 N/S 岛主 其它信息
/sac 搜索你回复或at 的人的AnimalCrossing 信息
/myisland 显示自己的岛信息
/open_island 开放自己的岛 相同指令 /open_airport
/close_island 关闭自己的岛 相同指令 /close_airport
/dtcj 更新大头菜价格, 不带参数时，和 /gj 相同
/gj 大头菜最新价格，只显示同群中价格从高到低前10
/ghs 搞化石交换用的登记spreadsheet链接
/islands 提供网页展示本bot 记录的所有动森岛屿信息
/login 登录到本bot 的web 界面，更方便查看信息
/help 查看本帮助信息`
	var botCommands []BotCommand
	scanner := bufio.NewScanner(strings.NewReader(botHelpText))
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		sep := strings.SplitN(scanner.Text(), " ", 2)
		botCommands = append(botCommands,
			BotCommand{Command: strings.TrimSpace(sep[0]),
				Description: strings.TrimSpace(sep[1]),
			})
	}

	router := NewRouter()
	router.HandleFunc("help", func(message *tgbotapi.Message) (replyMessage []*tgbotapi.MessageConfig, err error) {
		return []*tgbotapi.MessageConfig{&tgbotapi.MessageConfig{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true},
				Text: botHelpText}},
			nil
	})

	if _, err = setMyCommands(botCommands); err != nil {
		logrus.Warn(err)
	}
	router.HandleFunc("addfc", cmdAddFC)
	router.HandleFunc("myfc", cmdMyFC)
	router.HandleFunc("sfc", cmdSearchFC)
	router.HandleFunc("fc", cmdSearchFC)
	router.HandleFunc("fclist", cmdListFriendCodes)

	// Animal Crossing: New Horizons
	router.HandleFunc("islands", cmdListIslands)
	router.HandleFunc("addisland", cmdAddMyIsland)
	router.HandleFunc("myisland", cmdMyIsland)
	router.HandleFunc("open_airport", cmdOpenIsland)
	router.HandleFunc("open_island", cmdOpenIsland)
	router.HandleFunc("close_airport", cmdCloseIsland)
	router.HandleFunc("close_island", cmdCloseIsland)
	router.HandleFunc("dtcj", cmdDTCPriceUpdate)
	router.HandleFunc("gj", cmdDTCMaxPriceInGroup)
	router.HandleFunc("sac", cmdSearchAnimalCrossingInfo)
	router.HandleFunc("ghs", cmdHuaShiJiaoHuanBiaoGe)

	router.HandleFunc("whois", cmdWhois)

	// web login
	router.HandleFunc("login", cmdWebLogin)

	// admin
	router.HandleFunc("importDATA", cmdImportData)
	router.HandleFunc("updateGroupInfo", cmdUpdateGroupInfo)
	router.HandleFunc("fclistall", cmdListAllFriendCodes)

	logrus.WithFields(logrus.Fields{"bot username": bot.Self.UserName,
		"bot id": bot.Self.ID}).Infof("Authorized on account %s, ID: %d", bot.Self.UserName, bot.Self.ID)

	botAdminID = adminID
	_projectID = projectID

	c := ChatBot{bot, router, projectID}

	info, err := bot.GetWebhookInfo()
	if err != nil {
		logrus.Fatal(err)
	}
	if info.LastErrorDate != 0 {
		logrus.WithField("last error message", info.LastErrorMessage).Info("Telegram callback failed")
	}
	if !info.IsSet() {
		var webhookConfig WebhookConfig
		var wc = tgbotapi.NewWebhook(fmt.Sprintf("https://%s.appspot.com/%s", appID, token))
		webhookConfig = WebhookConfig{WebhookConfig: wc}
		webhookConfig.MaxConnections = 10
		webhookConfig.AllowedUpdates = []string{"message", "inline_query"}
		_, err = c.SetWebhook(webhookConfig)
		if err != nil {
			logrus.Fatal(err)
		}
	}

	return c
}

// MessageHandler process message
func (c ChatBot) MessageHandler(updates chan tgbotapi.Update) {
	for i := 0; i < 2; i++ {
		go c.messageHandlerWorker(updates)
	}
}

func (c ChatBot) messageHandlerWorker(updates chan tgbotapi.Update) {
	for update := range updates {
		inlineQuery := update.InlineQuery
		message := update.Message
		if inlineQuery != nil && inlineQuery.Query == "myfc" {
			if result, err := inlineQueryMyFC(inlineQuery); err != nil {
				logrus.Warn(err)
			} else {
				c.TgBotClient.AnswerInlineQuery(*result)
			}
		} else if message != nil {
			if (message.Chat.IsGroup() || message.Chat.IsSuperGroup() || message.Chat.IsPrivate()) && message.IsCommand() {
				messageSendTime := time.Unix(int64(message.Date), 0)
				if time.Since(messageSendTime).Seconds() > 30 {
					logrus.Infof("old message dropped: %s", message.Text)
					continue
				}
				replyMessages, err := c.Route.Run(message)
				if err != nil {
					logrus.Warnf("%s", err.InnerError)
					if len(err.ReplyText) > 0 {
						replyMessage := tgbotapi.MessageConfig{
							BaseChat: tgbotapi.BaseChat{
								ChatID:           message.Chat.ID,
								ReplyToMessageID: message.MessageID},
							Text: err.ReplyText}
						c.TgBotClient.Send(replyMessage)
						continue
					}
				}
				for _, replyMessage := range replyMessages {
					var text = replyMessage.Text
					l := len(text)
					if replyMessage != nil && l > 0 {
						if l > 4096 {
							offset := 0
							for l > 4096 {
								remain := l - offset
								if remain > 4096 {
									remain = 4096
								}
								fm := tgbotapi.MessageConfig{
									BaseChat: tgbotapi.BaseChat{
										ChatID:           message.Chat.ID,
										ReplyToMessageID: message.MessageID},
									Text: replyMessage.Text[offset : offset+remain]}
								c.TgBotClient.Send(fm)
							}
						} else {
							c.TgBotClient.Send(*replyMessage)
						}
					}
				}
			} else if message != nil && message.LeftChatMember != nil {
				if message.Chat.IsPrivate() {
					continue
				}
				logrus.WithFields(logrus.Fields{
					"uid":   message.LeftChatMember.ID,
					"name":  message.LeftChatMember.FirstName + " " + message.LeftChatMember.LastName + "(" + message.LeftChatMember.UserName + ")",
					"gid":   message.Chat.ID,
					"group": message.Chat.Title,
				}).Info("user left group")
				if message.Chat.IsPrivate() {
					continue
				}
				var groupID int64 = message.Chat.ID
				ctx := context.Background()
				if err := storage.RemoveGroupIDFromUserGroupIDs(ctx, message.From.ID, groupID); err != nil {
					logrus.WithError(err).Error("remove groupid from user's groupids failed")
				}
			} else if message != nil && message.NewChatMembers != nil && len(*message.NewChatMembers) > 0 {
				if message.Chat.IsPrivate() {
					continue
				}
				ctx := context.Background()
				g := storage.Group{ID: message.Chat.ID, Type: message.Chat.Type, Title: message.Chat.Title}
				og, err := storage.GetGroup(ctx, g.ID)
				if err != nil {
					if strings.HasPrefix(err.Error(), "Not found group:") {
						g.Create(ctx)
					} else {
						logrus.WithError(err).Error("get group failed")
					}
				} else {
					if og.Title != g.Title || og.Type != g.Type {
						g.Update(ctx)
					}
				}
				for _, u := range *message.NewChatMembers {
					logrus.WithFields(logrus.Fields{
						"uid":   u.ID,
						"name":  u.FirstName + " " + u.LastName + "(" + u.UserName + ")",
						"gid":   message.Chat.ID,
						"group": message.Chat.Title,
					}).Info("user joined group")
					u, err := storage.GetUser(ctx, u.ID, g.ID)
					if err != nil {
						logrus.WithError(err).Error("get user failed")
					} else {
						if len(u.GroupIDs) > 0 {
							u.GroupIDs = append(u.GroupIDs, g.ID)
						} else {
							u.GroupIDs = []int64{g.ID}
						}
						if err = storage.AddGroupIDToUserGroupIDs(ctx, u.ID, g.ID); err != nil {
							logrus.WithError(err).Error("add groupid to user's groupids faild")
						}
					}
				}
			}
		}
	}
}

// WebhookConfig contains information about a SetWebhook request.
type WebhookConfig struct {
	tgbotapi.WebhookConfig
	AllowedUpdates []string
}

// SetWebhook sets a webhook.
//
// If this is set, GetUpdates will not get any data!
//
// If you do not have a legitimate TLS certificate, you need to include
// your self signed certificate with the config.
func (c ChatBot) SetWebhook(config WebhookConfig) (tgbotapi.APIResponse, error) {

	if config.Certificate == nil {
		v := url.Values{}
		v.Add("url", config.URL.String())
		if config.MaxConnections != 0 {
			v.Add("max_connections", strconv.Itoa(config.MaxConnections))
		}
		if len(config.AllowedUpdates) != 0 {
			v["allowed_updates"] = config.AllowedUpdates
		}

		return c.TgBotClient.MakeRequest("setWebhook", v)
	}

	params := make(map[string]string)
	params["url"] = config.URL.String()
	if config.MaxConnections != 0 {
		params["max_connections"] = strconv.Itoa(config.MaxConnections)
	}

	resp, err := c.TgBotClient.UploadFile("setWebhook", params, "certificate", config.Certificate)
	if err != nil {
		return tgbotapi.APIResponse{}, err
	}

	return resp, nil
}

// Stop the bot
func (c ChatBot) Stop() {
	c.TgBotClient.RemoveWebhook()
}
