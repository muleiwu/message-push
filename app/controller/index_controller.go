package controller

import (
	"net/http"

	"cnb.cool/mliev/examples/go-web/internal/interfaces"
	"github.com/gin-gonic/gin"
)

type IndexController struct {
	BaseResponse
}

func (receiver IndexController) GetIndex(c *gin.Context, helper interfaces.HelperInterface) {
	helper.GetLogger().Info("hello world")
	c.String(http.StatusOK, "你好, 世界")
}
