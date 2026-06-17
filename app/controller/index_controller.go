package controller

import (
	"net/http"

	"cnb.cool/mliev/open/go-web/pkg/helper"
	httpInterfaces "cnb.cool/mliev/open/go-web/pkg/server/http_server/interfaces"
)

type IndexController struct {
	BaseResponse
}

func (receiver IndexController) GetIndex(c httpInterfaces.RouterContextInterface) {
	helper.GetRequestLogger(c).Info("visiting homepage")
	c.HTML(http.StatusOK, "index.html", map[string]any{
		"title": "Mulei Message Service",
	})
}
