package controller

import (
	"net/http"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/dto"
	"github.com/gin-gonic/gin"
)

type BaseResponse struct {
}

// Success 成功响应
func (receiver BaseResponse) Success(c *gin.Context, data any) {
	c.JSON(http.StatusOK, dto.Response{
		Code:    constants.CodeSuccess,
		Message: constants.GetErrorMessage(constants.CodeSuccess),
		Data:    data,
	})
}

// SuccessWithMessage 带自定义消息的成功响应
func (receiver BaseResponse) SuccessWithMessage(c *gin.Context, message string, data any) {
	c.JSON(http.StatusOK, dto.Response{
		Code:    constants.CodeSuccess,
		Message: message,
		Data:    data,
	})
}

// Error 错误响应
func (receiver BaseResponse) Error(c *gin.Context, code int, message string) {
	httpStatus := receiver.getHTTPStatus(code)
	if message == "" {
		message = constants.GetErrorMessage(code)
	}

	c.JSON(httpStatus, dto.Response{
		Code:    code,
		Message: message,
	})
}

// ErrorWithData 带数据的错误响应
func (receiver BaseResponse) ErrorWithData(c *gin.Context, code int, message string, data any) {
	httpStatus := receiver.getHTTPStatus(code)
	if message == "" {
		message = constants.GetErrorMessage(code)
	}

	c.JSON(httpStatus, dto.Response{
		Code:    code,
		Message: message,
		Data:    data,
	})
}

// Helper functions for convenience

// SuccessWithData sends success response with data
func SuccessWithData(c *gin.Context, data any) {
	BaseResponse{}.Success(c, data)
}

// FailWithMessage sends error response with message
func FailWithMessage(c *gin.Context, message string) {
	BaseResponse{}.Error(c, constants.CodeBadRequest, message)
}

// FailWithCode sends error response with error code
func FailWithCode(c *gin.Context, code int) {
	BaseResponse{}.Error(c, code, "")
}

// SuccessResponse 成功响应（便捷函数）
func SuccessResponse(c *gin.Context, data any) {
	BaseResponse{}.Success(c, data)
}

// ErrorResponse 错误响应（便捷函数）
func ErrorResponse(c *gin.Context, code int, message string) {
	BaseResponse{}.Error(c, code, message)
}

// getHTTPStatus 根据业务错误码获取HTTP状态码
func (receiver BaseResponse) getHTTPStatus(code int) int {
	// 如果是标准HTTP状态码，直接返回
	if code >= 400 && code < 600 {
		return code
	}
	// 其他情况返回200
	return http.StatusOK
}
