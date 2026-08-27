package middleware

import (
	"bytes"
	"io"
	"net/http"

	httpInterfaces "cnb.cool/mliev/open/go-web/pkg/server/http_server/interfaces"
	"cnb.cool/mliev/push/message-push/app/controller"
	appHelper "cnb.cool/mliev/push/message-push/app/helper"
)

// EmailAttachmentRequestBodyLimitMiddleware 在 JSON 与 HMAC 解析前限制请求体大小。
func EmailAttachmentRequestBodyLimitMiddleware() httpInterfaces.HandlerFunc {
	return func(c httpInterfaces.RouterContextInterface) {
		request := c.Request()
		if request.Body == nil {
			c.Next()
			return
		}

		maxBytes := appHelper.GetEmailAttachmentLimits().MaxJSONBodyBytes()
		if request.ContentLength > maxBytes {
			controller.ErrorResponse(c, http.StatusRequestEntityTooLarge, "request body exceeds email attachment limit")
			c.Abort()
			return
		}

		body, err := io.ReadAll(io.LimitReader(request.Body, maxBytes+1))
		if err != nil {
			controller.ErrorResponse(c, http.StatusBadRequest, "failed to read request body")
			c.Abort()
			return
		}
		if int64(len(body)) > maxBytes {
			controller.ErrorResponse(c, http.StatusRequestEntityTooLarge, "request body exceeds email attachment limit")
			c.Abort()
			return
		}
		request.Body = io.NopCloser(bytes.NewReader(body))
		c.Next()
	}
}
