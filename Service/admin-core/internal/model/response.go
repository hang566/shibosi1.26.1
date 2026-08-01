package model

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Response 统一API响应格式
type Response struct {
	Code      int         `json:"code"`
	Msg       string      `json:"msg"`
	Data      interface{} `json:"data"`
	Timestamp int64       `json:"timestamp"`
}

// PageResponse 分页响应
type PageResponse struct {
	List     interface{} `json:"list"`
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
}

// 业务状态码
const (
	CodeSuccess      = 0
	CodeBadRequest   = 400
	CodeUnauthorized = 401
	CodeForbidden    = 403
	CodeNotFound     = 404
	CodeConflict     = 409
	CodeInternal     = 500
	CodeServiceUnavail = 503
)

// Success 成功响应
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:      CodeSuccess,
		Msg:       "success",
		Data:      data,
		Timestamp: time.Now().UnixMilli(),
	})
}

// SuccessPage 分页成功响应
func SuccessPage(c *gin.Context, list interface{}, total int64, page, pageSize int) {
	c.JSON(http.StatusOK, Response{
		Code: CodeSuccess,
		Msg:  "success",
		Data: PageResponse{
			List:     list,
			Total:    total,
			Page:     page,
			PageSize: pageSize,
		},
		Timestamp: time.Now().UnixMilli(),
	})
}

// Fail 业务失败响应
func Fail(c *gin.Context, code int, msg string) {
	c.JSON(http.StatusOK, Response{
		Code:      code,
		Msg:       msg,
		Data:      nil,
		Timestamp: time.Now().UnixMilli(),
	})
}

// Error 系统错误响应
func Error(c *gin.Context, httpStatus int, msg string) {
	c.JSON(httpStatus, Response{
		Code:      httpStatus,
		Msg:       msg,
		Data:      nil,
		Timestamp: time.Now().UnixMilli(),
	})
}

// AbortError 中断并返回错误
func AbortError(c *gin.Context, httpStatus int, msg string) {
	c.AbortWithStatusJSON(httpStatus, Response{
		Code:      httpStatus,
		Msg:       msg,
		Data:      nil,
		Timestamp: time.Now().UnixMilli(),
	})
}