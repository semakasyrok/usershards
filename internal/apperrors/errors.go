package apperrors

import "errors"

var (
	ErrCompensationCompleted = errors.New("compensation is completed")
	ErrUserIsBlocked         = errors.New("user is blocked")
)
