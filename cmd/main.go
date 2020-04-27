package main

import (
	"errors"
	"io/ioutil"
	"os"
	"strconv"

	"github.com/doylecnn/new-nsfc-bot/chatbot"
	"github.com/doylecnn/new-nsfc-bot/stackdriverhook"
	"github.com/doylecnn/new-nsfc-bot/storage"
	"github.com/doylecnn/new-nsfc-bot/web"
	"github.com/sirupsen/logrus"
)

type env struct {
	Port       string
	BotToken   string
	BotAdminID int
	AppID      string
	Domain     string
	projectID  string
}

func main() {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	env := readEnv()
	storage.ProjectID = env.projectID
	if h, err := stackdriverhook.NewStackDriverHook(env.projectID, "nsfcbot"); err != nil {
		logger.WithError(err).Error("new hook failed")
	} else {
		defer h.Close()
		logger.Hooks.Add(h)
		logger.Out = ioutil.Discard
	}
	logger.SetLevel(logrus.DebugLevel)
	storage.Logger = logger
	bot := chatbot.NewChatBot(env.BotToken, env.Domain, env.AppID, env.projectID, env.Port, env.BotAdminID, logger)
	web, updates := web.NewWeb(env.BotToken, env.Domain, env.AppID, env.projectID, env.Port, env.BotAdminID, bot, logger)

	go bot.MessageHandler(updates)
	web.Run()
}

func readEnv() env {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		logrus.WithField("port", port).Info("set default port")
	}

	token := os.Getenv("BOT_TOKEN")

	botAdmin := os.Getenv("BOT_ADMIN")
	if botAdmin == "" {
		err := errors.New("not set env BOT_ADMIN")
		logrus.Fatal(err)
	}
	botAdminID, err := strconv.Atoi(botAdmin)
	if err != nil {
		logrus.Fatal(err)
	}

	appID := os.Getenv("GAE_APPLICATION")
	if appID == "" {
		logrus.Fatal("no env var: GAE_APPLICATION")
	}
	appID = appID[2:]
	logrus.Infof("appID:%s", appID)

	projectID := os.Getenv("PROJECT_ID")
	if projectID == "" {
		logrus.Fatal("no env var: PROJECT_ID")
	}

	domain := os.Getenv("DOMAIN")
	if domain == "" {
		logrus.Fatal("no env var: DOMAIN")
	}

	return env{port, token, botAdminID, appID, domain, projectID}
}
