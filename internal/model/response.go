package model

type JSONResponse struct {
	Status string `json:"status"`
	Data   any    `json:"data,omitempty"`
}

func NewSuccessResponse(data any) JSONResponse {
	return JSONResponse{
		Status: "success",
		Data:   data,
	}
}

func NewErrorResponse(code string, message string, details any) APIError {
	return APIError{
		Status:  "error",
		Code:    code,
		Message: message,
		Details: details,
	}
}
