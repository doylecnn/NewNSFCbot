package web

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

//Logout logout page
func (w Web) Logout(c *gin.Context) {
	w.delCookie(c, "auth_data_str")
	w.delCookie(c, "auth_data_hash")
	w.delCookie(c, "admin_authed")
	w.delCookie(c, "authed")
	c.Redirect(http.StatusTemporaryRedirect, "/")
}
