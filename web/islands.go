package web

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/doylecnn/new-nsfc-bot/storage"
	"github.com/doylecnn/new-nsfc-bot/web/middleware"

	"github.com/gin-gonic/gin"
)

//Islands public islands page
func (w Web) Islands(c *gin.Context) {
	if v, exists := c.Get("authed"); exists {
		if authed, ok := v.(bool); ok && authed {
			ctx := context.Background()
			authData, _ := c.Cookie("auth_data_str")
			userID, err := middleware.GetAuthDataInfo(authData, "id")
			if err != nil {
				_logger.WithError(err).Warning("get auth data info")
				c.AbortWithError(http.StatusInternalServerError, err)
				return
			}
			uid, err := strconv.ParseInt(userID, 10, 64)
			if err != nil {
				_logger.WithError(err).Warning("parse int")
				c.AbortWithError(http.StatusInternalServerError, err)
				return
			}
			u, err := storage.GetUser(ctx, int(uid), 0)
			if err != nil {
				_logger.WithError(err).Warning("get user")
				c.AbortWithError(http.StatusInternalServerError, err)
				return
			}
			var users map[int]*storage.User = make(map[int]*storage.User)
			for _, gid := range u.GroupIDs {
				us, err := storage.GetGroupUsers(ctx, gid)
				if err != nil {
					_logger.WithError(err).Warning("get group users")
					c.AbortWithError(http.StatusInternalServerError, err)
					return
				} else if len(us) == 0 {
					_logger.WithError(err).Warning("no users in group")
					c.AbortWithError(http.StatusInternalServerError, errors.New("not found user by groupid"))
					return
				}
				for _, u = range us {
					if _, ok := users[u.ID]; !ok {
						users[u.ID] = u
					}
				}
			}

			var haveIslandUsers []storage.User
			var priceOutDate []bool
			for _, user := range users {
				island, _, err := user.GetAnimalCrossingIsland(ctx)
				if err != nil || island == nil {
					continue
				}
				if !strings.HasSuffix(island.Name, "岛") {
					island.Name += "岛"
				}
				priceOutDate = append(priceOutDate, time.Since(island.LastPrice.Date) > 12*time.Hour)
				if island.AirportIsOpen {
					locOpenTime := island.OpenTime.In(island.Timezone.Location())
					locNow := time.Now().In(island.Timezone.Location())
					if locNow.Hour() >= 5 && (locOpenTime.Hour() >= 0 && locOpenTime.Hour() < 5 ||
						locNow.Day()-locOpenTime.Day() >= 1) {
						island.Close(ctx)
					}
				}
				user.Island = island
				haveIslandUsers = append(haveIslandUsers, *user)
			}
			c.HTML(200, "islands.html", gin.H{
				"uid":          userID,
				"users":        haveIslandUsers,
				"priceOutDate": priceOutDate,
			})

			return
		}
	}
	c.Redirect(http.StatusTemporaryRedirect, "/login")
}
