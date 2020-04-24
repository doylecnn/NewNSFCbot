package chatbot

import (
	"context"
	"errors"
	"fmt"
	"sort"

	fuzzy "github.com/doylecnn/go-fuzzywuzzy"
	"github.com/doylecnn/new-nsfc-bot/storage"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	// ErrCommandNotImplemented command not  implemented
	ErrCommandNotImplemented = errors.New("NotImplemented")
)

// Error is error
type Error struct {
	InnerError error
	ReplyText  string
}

func (e Error) Error() string {
	return e.InnerError.Error()
}

// Command interface
type Command interface {
	Do(message *tgbotapi.Message) (replyMessage []*tgbotapi.MessageConfig, err error)
}

// Router is command router
type Router struct {
	commands       map[string]func(message *tgbotapi.Message) (replyMessage []*tgbotapi.MessageConfig, err error)
	commandSuggest func(message *tgbotapi.Message) (replyMessage []*tgbotapi.MessageConfig, err error)
}

// NewRouter returns new Router
func NewRouter() Router {
	r := Router{}
	r.commands = make(map[string]func(message *tgbotapi.Message) (replyMessage []*tgbotapi.MessageConfig, err error))
	r.commandSuggest = cmdSuggest
	return r
}

// HandleFunc regist HandleFunc
func (r Router) HandleFunc(cmd string, f func(message *tgbotapi.Message) (replyMessage []*tgbotapi.MessageConfig, err error)) {
	if _, ok := r.commands[cmd]; ok {
		_logger.Fatalln(errors.New("already exists handle func"))
	}
	r.commands[cmd] = f
}

// Run the command
func (r Router) Run(message *tgbotapi.Message) (replyMessage []*tgbotapi.MessageConfig, err *Error) {
	if !message.IsCommand() {
		return
	}
	ctx := context.Background()
	var groupID int64 = 0
	if !message.Chat.IsPrivate() {
		groupID = message.Chat.ID
	}
	if groupID != 0 {
		if lerr := storage.AddGroupIDToUserGroupIDs(ctx, message.From.ID, groupID); lerr != nil {
			_logger.WithError(err).Error("add groupid to user's groupids failed")
		}
		g, err := storage.GetGroup(ctx, groupID)
		if err != nil && status.Code(err) != codes.NotFound {
			_logger.Errorf("GetGroupError:%v", err)
		} else if err != nil && status.Code(err) == codes.NotFound {
			g = storage.Group{ID: message.Chat.ID, Type: message.Chat.Type, Title: message.Chat.Title}
			g.Set(ctx)
		} else {
			if g.Title != message.Chat.Title || g.Type != message.Chat.Type {
				g.Type = message.Chat.Type
				g.Title = message.Chat.Title
				g.Update(ctx)
			}
		}
	}
	_logger.WithFields(logrus.Fields{
		"command":          message.Command(),
		"args":             message.CommandArguments(),
		"receive datetime": message.Time().Format("2016-01-02 15:04:05 -0700"),
		"UID":              message.From.ID,
		"ChatID":           message.Chat.ID,
		"FromUser":         message.From.UserName,
	}).Info("receive command")
	cmd := message.Command()
	if c, ok := r.commands[cmd]; ok {
		replies, e := c(message)
		if e != nil {
			if e, ok := e.(Error); ok {
				return nil, &Error{InnerError: fmt.Errorf("error occurred when running cmd: %s\n error is: %s", cmd, e.InnerError), ReplyText: e.ReplyText}
			}
			return nil, &Error{InnerError: fmt.Errorf("error occurred when running cmd: %s\n error is: %s", cmd, e)}
		}
		return replies, nil
	} else if r.commandSuggest != nil {
		replies, e := r.commandSuggest(message)
		if e != nil {
			if e, ok := e.(Error); ok {
				return nil, &Error{InnerError: fmt.Errorf("error occurred when running cmd: /suggest\n error is: %s", e.InnerError), ReplyText: e.ReplyText}
			}
			return nil, &Error{InnerError: fmt.Errorf("error occurred when running cmd: /suggest\n error is: %s", e)}
		}
		return replies, nil
	}
	return nil, &Error{InnerError: fmt.Errorf("no HandleFunc for command /%s", cmd)}
}

func cmdSuggest(message *tgbotapi.Message) (replyMessage []*tgbotapi.MessageConfig, err error) {
	cmd := message.Command()
	cmds, err := getMyCommands()
	if err != nil {
		_logger.WithError(err).Warn("get my commands failed.")
	}
	var fuzzyScores []int
	var scoreCmdIdx map[int]int = make(map[int]int)
	for i, c := range cmds {
		score := fuzzy.Ratio(cmd, c.Command)
		fuzzyScores = append(fuzzyScores, score)
		scoreCmdIdx[score] = i
	}
	sort.Slice(fuzzyScores, func(i, j int) bool {
		return fuzzyScores[i] > fuzzyScores[j]
	})
	mostSuggestCommand := cmds[scoreCmdIdx[fuzzyScores[0]]]

	return []*tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: fmt.Sprintf("没有找到你输入的指令狸。猜测你想执行的是：\n/%s %s",
				mostSuggestCommand.Command,
				mostSuggestCommand.Description)}},
		nil
}
