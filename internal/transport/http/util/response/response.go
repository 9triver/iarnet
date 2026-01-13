package response

import (
	"encoding/json"
	"net/http"
)

// BaseResponse 统一的API响应结构
type BaseResponse struct {
	Code       int    `json:"code"`            // 业务状态码
	Message    string `json:"message"`         // 响应消息
	Data       any    `json:"data,omitempty"`  // 响应数据
	Error      string `json:"error,omitempty"` // 错误信息
	HTTPStatus int    `json:"-"`               // HTTP 状态码（可选，默认与 Code 相同）
}

// Success 创建成功响应
func Success(data any) *BaseResponse {
	return &BaseResponse{
		Code:    http.StatusOK,
		Message: "success",
		Data:    data,
	}
}

// SuccessWithMessage 创建带自定义消息的成功响应
func SuccessWithMessage(message string, data any) *BaseResponse {
	return &BaseResponse{
		Code:    http.StatusOK,
		Message: message,
		Data:    data,
	}
}

// Created 创建资源成功响应
func Created(data any) *BaseResponse {
	return &BaseResponse{
		Code:    http.StatusCreated,
		Message: "created",
		Data:    data,
	}
}

// Accepted 请求已接受响应
func Accepted(message string) *BaseResponse {
	return &BaseResponse{
		Code:    http.StatusAccepted,
		Message: message,
	}
}

// BadRequest 创建错误请求响应
func BadRequest(error string) *BaseResponse {
	return &BaseResponse{
		Code:    http.StatusBadRequest,
		Message: "bad request",
		Error:   error,
	}
}

// Unauthorized 创建未授权响应
func Unauthorized(error string) *BaseResponse {
	return &BaseResponse{
		Code:    http.StatusUnauthorized,
		Message: "unauthorized",
		Error:   error,
	}
}

// Forbidden 创建禁止访问响应
func Forbidden(error string) *BaseResponse {
	return &BaseResponse{
		Code:    http.StatusForbidden,
		Message: "forbidden",
		Error:   error,
	}
}

// InternalError 创建内部服务器错误响应
func InternalError(error string) *BaseResponse {
	return &BaseResponse{
		Code:    http.StatusInternalServerError,
		Message: "internal server error",
		Error:   error,
	}
}

// ServiceUnavailable 创建服务不可用响应
func ServiceUnavailable(error string) *BaseResponse {
	return &BaseResponse{
		Code:    http.StatusServiceUnavailable,
		Message: "service unavailable",
		Error:   error,
	}
}

// NotFound 创建资源未找到响应
func NotFound(error string) *BaseResponse {
	return &BaseResponse{
		Code:    http.StatusNotFound,
		Message: "not found",
		Error:   error,
	}
}

// WriteJSON 将响应写入HTTP响应
func (r *BaseResponse) WriteJSON(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json")
	status := r.HTTPStatus
	if status == 0 {
		status = r.Code
	}
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(r)
}

// WriteJSONWithStatus 写入JSON响应并设置状态码
func WriteJSON(w http.ResponseWriter, statusCode int, data any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	return json.NewEncoder(w).Encode(data)
}

// WriteSuccess 写入成功响应
func WriteSuccess(w http.ResponseWriter, data any) error {
	return Success(data).WriteJSON(w)
}

// WriteError 写入错误响应
func WriteError(w http.ResponseWriter, statusCode int, message string, err error) error {
	errorMsg := ""
	if err != nil {
		errorMsg = err.Error()
	}

	response := &BaseResponse{
		Code:    statusCode,
		Message: message,
		Error:   errorMsg,
	}

	return response.WriteJSON(w)
}

// BusinessError 创建业务错误响应（HTTP 状态依然为 200）
func BusinessError(code int, message string, data any) *BaseResponse {
	return &BaseResponse{
		Code:       code,
		Message:    message,
		Data:       data,
		HTTPStatus: http.StatusOK,
	}
}
