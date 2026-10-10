package pkg

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Body struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Body{Code: 0, Message: "ok", Data: data})
}

func Fail(c *gin.Context, httpStatus, code int, msg string) {
	c.JSON(httpStatus, Body{Code: code, Message: msg})
}

func BadRequest(c *gin.Context, msg string) {
	Fail(c, http.StatusBadRequest, 40001, msg)
}

func Unauthorized(c *gin.Context, msg string) {
	Fail(c, http.StatusUnauthorized, 40101, msg)
}

func Forbidden(c *gin.Context, msg string) {
	Fail(c, http.StatusForbidden, 40301, msg)
}

func NotFound(c *gin.Context, msg string) {
	Fail(c, http.StatusNotFound, 40401, msg)
}

func Internal(c *gin.Context, msg string) {
	Fail(c, http.StatusInternalServerError, 50001, msg)
}

type PageResult struct {
	List     interface{} `json:"list"`
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
}

func PageParams(c *gin.Context) (page, pageSize int) {
	page = 1
	pageSize = 20
	if v := c.Query("page"); v != "" {
		if n, err := strconvAtoi(v); err == nil && n > 0 {
			page = n
		}
	}
	if v := c.Query("page_size"); v != "" {
		if n, err := strconvAtoi(v); err == nil && n > 0 {
			pageSize = n
		}
	}
	if pageSize > 50 {
		pageSize = 50
	}
	return
}

func strconvAtoi(s string) (int, error) {
	n := 0
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0, errInvalid
		}
		n = n*10 + int(ch-'0')
	}
	return n, nil
}

var errInvalid = errString("invalid")

type errString string

func (e errString) Error() string { return string(e) }