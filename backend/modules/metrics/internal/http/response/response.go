package response

type SuccessResponse[T any] struct {
	Success bool `json:"success"`
	Data    T    `json:"data"`
	Meta    Meta `json:"meta,omitempty"`
}

type ErrorResponse struct {
	Success bool      `json:"success"`
	Error   ErrorBody `json:"error"`
	Meta    Meta      `json:"meta,omitempty"`
}

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Meta struct {
	RequestID string `json:"requestId,omitempty"`
}
