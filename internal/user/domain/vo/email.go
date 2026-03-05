package vo

import (
	"regexp"
	"simple-securities/internal/user/application/constant"
)

var emailRegex = regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}$`)

type Email struct {
	value string
}

func NewEmail(v string) (Email, error) {
	if !emailRegex.MatchString(v) {
		return Email{}, constant.ErrInvalidUserEmail
	}
	return Email{value: v}, nil
}

func (e Email) Value() string {
	return e.value
}
