package account

import (
	"regexp"
	"strings"
	"unicode/utf8"

	apperrors "yunjian/internal/platform/errors"
)

var usernamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{2,31}$`)

func normalizeRegister(request *AccountRegisterRequest) error {
	if request == nil {
		return apperrors.BadRequest(400, "请求格式不正确")
	}
	request.Username = strings.ToLower(strings.TrimSpace(request.Username))
	request.DisplayName = strings.TrimSpace(request.DisplayName)
	if !usernamePattern.MatchString(request.Username) {
		return apperrors.BadRequest(400, "用户名需以字母开头，使用 3 到 32 位小写字母、数字或下划线")
	}
	if err := validatePassword(request.Password); err != nil {
		return err
	}
	if request.DisplayName == "" {
		request.DisplayName = request.Username
	}
	if utf8.RuneCountInString(request.DisplayName) > 64 || hasControl(request.DisplayName) {
		return apperrors.BadRequest(400, "显示名称不能超过 64 个字符")
	}
	return nil
}

func normalizeLogin(request *AccountLoginRequest) error {
	if request == nil {
		return apperrors.BadRequest(400, "请求格式不正确")
	}
	request.Username = strings.ToLower(strings.TrimSpace(request.Username))
	if request.Username == "" || request.Password == "" {
		return apperrors.BadRequest(400, "请输入用户名和密码")
	}
	return nil
}

func validatePassword(password string) error {
	if strings.ContainsRune(password, 0) || utf8.RuneCountInString(password) < 8 || len(password) > 72 {
		return apperrors.BadRequest(400, "密码至少 8 个字符，且不超过 72 字节")
	}
	return nil
}

func hasControl(value string) bool {
	for _, r := range value {
		if r < 32 || r == 127 {
			return true
		}
	}
	return false
}
