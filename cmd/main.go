package main

import (
	"errors"
	"os"
	"strconv"

	"github.com/doylecnn/new-nsfc-bot/chatbot"
	"github.com/doylecnn/new-nsfc-bot/storage"
	"github.com/doylecnn/new-nsfc-bot/web"
	"github.com/sirupsen/logrus"
)

type env struct {
	Port       string
	BotToken   string
	BotAdminID int
	AppID      string
	projectID  string
}

func main() {
	logrus.SetLevel(logrus.DebugLevel)
	env := readEnv()
	storage.ProjectID = env.projectID
	bot := chatbot.NewChatBot(env.BotToken, env.AppID, env.projectID, env.Port, env.BotAdminID)
	web, updates := web.NewWeb(env.BotToken, env.AppID, env.projectID, env.Port, env.BotAdminID, bot)

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

	return env{port, token, botAdminID, appID, projectID}
}
