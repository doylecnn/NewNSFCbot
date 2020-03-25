package chatbot

import (
	"errors"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	log "github.com/sirupsen/logrus"
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
	Do(message *tgbotapi.Message) (replyMessage *tgbotapi.MessageConfig, err error)
}

// Router is command router
type Router struct {
	commands map[string]func(message *tgbotapi.Message) (replyMessage *tgbotapi.MessageConfig, err error)
}

// NewRouter returns new Router
func NewRouter() Router {
	r := Router{}
	r.commands = make(map[string]func(message *tgbotapi.Message) (replyMessage *tgbotapi.MessageConfig, err error))
	return r
}

// HandleFunc regist HandleFunc
func (r Router) HandleFunc(cmd string, f func(message *tgbotapi.Message) (replyMessage *tgbotapi.MessageConfig, err error)) {
	if _, ok := r.commands[cmd]; ok {
		log.Fatalln(errors.New("already exists handle func"))
	}
	r.commands[cmd] = f
}

// Run the command
func (r Router) Run(message *tgbotapi.Message) (replyMessage *tgbotapi.MessageConfig, err *Error) {
	cmd := message.Command()
	if c, ok := r.commands[cmd]; ok {
		reply, e := c(message)
		if e != nil {
			if e, ok := e.(Error); ok {
				return nil, &Error{InnerError: fmt.Errorf("error occurred when running cmd: %s\n error is: %s", cmd, e.InnerError), ReplyText: e.ReplyText}
			}
			return nil, &Error{InnerError: fmt.Errorf("error occurred when running cmd: %s\n error is: %s", cmd, e)}
		}
		return reply, nil
	}
	return nil, &Error{InnerError: fmt.Errorf("no HandleFunc for %s", cmd)}
}
