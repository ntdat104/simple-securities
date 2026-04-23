package constant

import (
	stderrors "errors"
	"simple-securities/pkg/errors"
)

var (
	ErrUserNotFound        = errors.New(errors.ErrorTypeNotFound, "user not found")
	ErrEmptyUserName       = errors.New(errors.ErrorTypeValidation, "user full name cannot be empty")
	ErrInvalidUserID       = errors.New(errors.ErrorTypeValidation, "invalid user id")
	ErrInvalidUserEmail    = errors.New(errors.ErrorTypeValidation, "invalid user email")
	ErrUserPasswordMissing = errors.New(errors.ErrorTypeValidation, "user password missing")
	ErrUserEmailTaken      = errors.New(errors.ErrorTypeConflict, "user email already taken")
	ErrUserInvalidUpdate   = errors.New(errors.ErrorTypeValidation, "invalid user update data")
	ErrUserModified        = errors.New(errors.ErrorTypeConflict, "user modified by another process")

	ErrInsufficientCredits   = errors.New(errors.ErrorTypeBusiness, "insufficient credits")
	ErrWalletNotFound        = errors.New(errors.ErrorTypeNotFound, "credit wallet not found")
	ErrReservationNotFound   = errors.New(errors.ErrorTypeNotFound, "reservation not found")
	ErrReservationExpired    = errors.New(errors.ErrorTypeBusiness, "reservation has expired")
	ErrReservationNotPending = errors.New(errors.ErrorTypeBusiness, "reservation is not in PENDING status")
	ErrDuplicateIdempotency  = errors.New(errors.ErrorTypeBusiness, "duplicate request: idempotency key already used")
	ErrInvalidAmount         = errors.New(errors.ErrorTypeBusiness, "amount must be greater than zero")
	ErrExpireAtRequired      = errors.New(errors.ErrorTypeBusiness, "expire_at is required for this wallet type")
)

func NewUserNotFoundWithID(id uint64) *errors.AppError {
	return errors.Newf(errors.ErrorTypeNotFound, "user with ID %d not found", id)
}

func NewUserNotFoundWithEmail(email string) *errors.AppError {
	return errors.Newf(errors.ErrorTypeNotFound, "user with email %s not found", email)
}

func NewUserEmailTakenError(email string) *errors.AppError {
	return errors.Newf(errors.ErrorTypeConflict, "user with email '%s' already exists", email)
}

func IsUserNotFoundError(err error) bool {
	return stderrors.Is(err, ErrUserNotFound) || errors.IsNotFoundError(err)
}

func IsUserValidationError(err error) bool {
	return stderrors.Is(err, ErrEmptyUserName) ||
		stderrors.Is(err, ErrInvalidUserID) ||
		stderrors.Is(err, ErrInvalidUserEmail) ||
		errors.IsValidationError(err)
}

func IsUserEmailTakenError(err error) bool {
	return stderrors.Is(err, ErrUserEmailTaken)
}

func IsUserModifiedError(err error) bool {
	return stderrors.Is(err, ErrUserModified)
}
