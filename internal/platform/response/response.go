// Package response 定义统一结果，并提供 Gin 直接输出函数。
package response

import "github.com/gin-gonic/gin"

// Response 是单条业务结果。Code=200 表示成功。
type Response[T any] struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data T      `json:"data,omitempty"`
}

// PageResponse 是分页结果，成功时不额外包裹 code/msg。
type PageResponse[T any] struct {
	Data     []T   `json:"data"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
}

func SuccessResponse[T any](c *gin.Context, data Response[T]) {
	c.JSON(200, data)
}

func SuccessPage[T any](c *gin.Context, data PageResponse[T]) {
	c.JSON(200, data)
}

func OK[T any](data T) Response[T] {
	return Response[T]{Code: 200, Msg: "success", Data: data}
}
