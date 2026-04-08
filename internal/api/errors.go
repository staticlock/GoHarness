package api

import "errors"

var (
	// ErrAuthentication is returned when upstream rejects credentials.
	ErrAuthentication = errors.New("authentication failure")
	// ErrRateLimit is returned when upstream rate-limits requests.
	ErrRateLimit = errors.New("rate limit failure")
	// ErrRequest is returned for generic transport/request failures.
	ErrRequest = errors.New("request failure")
)
