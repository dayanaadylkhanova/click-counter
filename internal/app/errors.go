package app

import "errors"

var (
	ErrAppStartup           = errors.New("app startup error")
	ErrAppShutdownNormal    = errors.New("app shutdown normal")
	ErrAppShutdownWithError = errors.New("app shutdown with error")
)
