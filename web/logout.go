package web

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

//Logout logout page
func Logout(c *gin.Context) {
	delCookie(c, "auth_data_str")
	delCookie(c, "auth_data_hash")
	c.Redirect(http.StatusTemporaryRedirect, "/")
}
