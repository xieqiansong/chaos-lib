package resp

const (
	CodeOK           = 0
	CodeInvalidParam = 40001
	CodeUnauthorized = 40100
	CodeForbidden    = 40300
	CodeNotFound     = 40400
	CodeConflict     = 40900
	CodeSystemError  = 50000
)

func statusToCode(status int) int {
	switch status {
	case 400, 422:
		return CodeInvalidParam
	case 401:
		return CodeUnauthorized
	case 403:
		return CodeForbidden
	case 404:
		return CodeNotFound
	case 409:
		return CodeConflict
	case 500, 502, 503, 504:
		return CodeSystemError
	default:
		if status >= 500 {
			return CodeSystemError
		}
		return CodeInvalidParam
	}
}
