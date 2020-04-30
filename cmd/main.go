package main

import (
	"errors"
	"os"
	"strconv"
	"time"

	"math/rand"

	"github.com/doylecnn/new-nsfc-bot/chatbot"
	"github.com/doylecnn/new-nsfc-bot/storage"
	"github.com/doylecnn/new-nsfc-bot/web"
	"github.com/rs/zerolog/log"
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
	env := readEnv()
	rand.Seed(time.Now().Unix())

	storage.InitLogger(env.projectID)

	bot := chatbot.NewChatBot(env.BotToken, env.Domain, env.AppID, env.projectID, env.Port, env.BotAdminID)
	defer bot.Close()
	web, updates := web.NewWeb(env.BotToken, env.Domain, env.AppID, env.projectID, env.Port, env.BotAdminID, bot)
	defer web.Close()

	go bot.MessageHandler(updates)
	web.Run()
}

func readEnv() env {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Info().Str("port", port).Msg("set default port")
	}

	token := os.Getenv("BOT_TOKEN")

	botAdmin := os.Getenv("BOT_ADMIN")
	if botAdmin == "" {
		err := errors.New("not set env BOT_ADMIN")
		log.Logger.Fatal().Err(err).Send()
	}
	botAdminID, err := strconv.Atoi(botAdmin)
	if err != nil {
		log.Logger.Fatal().Err(err).Send()
	}

	appID := os.Getenv("GAE_APPLICATION")
	if appID == "" {
		log.Logger.Fatal().Msg("no env var: GAE_APPLICATION")
	}
	appID = appID[2:]
	log.Logger.Info().Str("appID", appID).Send()

	projectID := os.Getenv("PROJECT_ID")
	if projectID == "" {
		log.Logger.Fatal().Msg("no env var: PROJECT_ID")
	}

	domain := os.Getenv("DOMAIN")
	if domain == "" {
		log.Logger.Fatal().Msg("no env var: DOMAIN")
	}

	return env{port, token, botAdminID, appID, domain, projectID}
}
