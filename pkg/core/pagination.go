package core

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	// DefaultPageSize is used when the caller omits a page size.
	DefaultPageSize = 25
	// MaxPageSize is the upper bound on a single page.
	MaxPageSize = 200
)

// ParsePageToken decodes an opaque page token into a zero-based offset.
// An empty token means the first page.
func ParsePageToken(token string) (int, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return 0, nil
	}
	offset, err := strconv.Atoi(token)
	if err != nil || offset < 0 {
		return 0, fmt.Errorf("%w: invalid page token", ErrInvalidInput)
	}
	return offset, nil
}

// FormatPageToken encodes a zero-based offset as an opaque page token.
func FormatPageToken(offset int) string {
	return strconv.Itoa(offset)
}

// NextPageToken returns the token for the page following the current one, or ""
// when there are no more rows.
func NextPageToken(offset, returned int, total int32) string {
	if returned == 0 {
		return ""
	}
	next := offset + returned
	if int32(next) >= total {
		return ""
	}
	return FormatPageToken(next)
}

// NormalizePageSize clamps a requested page size to [1, MaxPageSize], defaulting to DefaultPageSize.
func NormalizePageSize(size int) (int, error) {
	if size == 0 {
		return DefaultPageSize, nil
	}
	if size < 0 || size > MaxPageSize {
		return 0, fmt.Errorf("%w: page size must be between 1 and %d", ErrInvalidInput, MaxPageSize)
	}
	return size, nil
}
