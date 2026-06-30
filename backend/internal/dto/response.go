// Package dto 定义 HTTP 传输层的请求与响应对象（Data Transfer Object）。
//
// 它与领域层（service）解耦：service 只关心业务输入/输出，dto 负责承载
// gin 的参数绑定规则、字段校验、Swagger 文档注解以及统一响应信封。handler
// 负责在 dto 与 service 类型之间做显式转换。
package dto

// Response 是所有 HTTP 接口的统一返回信封。
type Response struct {
	Code    int    `json:"code"`              // 业务状态码：0 表示成功，非 0 表示失败
	Message string `json:"message"`           // 提示信息：成功为 "success"，失败为错误描述
	Data    any    `json:"data,omitempty"`    // 业务数据载荷：失败时为空
}

// 业务状态码常量。HTTP 状态码由 handler 单独决定，这里只表达业务语义。
const (
	CodeOK         = 0    // 成功
	CodeValidation = 4000 // 参数校验失败
	CodeNotFound   = 4040 // 资源不存在
	CodeConflict   = 4090 // 状态冲突（如榜单未上线）
	CodeInternal   = 5000 // 服务内部错误
)

// Success 构造一个成功响应。
func Success(data any) Response {
	return Response{Code: CodeOK, Message: "success", Data: data}
}

// Fail 构造一个失败响应。
func Fail(code int, message string) Response {
	return Response{Code: code, Message: message}
}
