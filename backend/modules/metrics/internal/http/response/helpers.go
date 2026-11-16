package response

import (
	"github.com/gin-gonic/gin"
)

func OK[T any](c *gin.Context, status int, data T) {
	c.JSON(status, SuccessResponse[T]{
		Success: true,
		Data:    data,
		Meta:    metaFromContext(c),
	})
}

func Fail(
	c *gin.Context,
	status int,
	code string,
	message string,
) {
	c.JSON(status, ErrorResponse{
		Success: false,
		Error: ErrorBody{
			Code:    code,
			Message: message,
		},
		Meta: metaFromContext(c),
	})
}

func metaFromContext(c *gin.Context) Meta {
	if c == nil || c.Request == nil {
		return Meta{}
	}
	return Meta{
		RequestID: c.GetString("request_id"),
	}
}
