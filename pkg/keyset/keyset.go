// Package keyset menyediakan cursor opaque untuk keyset (seek) pagination
// berbasis (created_at, id). Keyset jauh lebih murah dari OFFSET besar karena
// langsung memakai indeks komposit (created_at DESC, id DESC).
package keyset

import (
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"
)

// ErrInvalidCursor dikembalikan bila cursor tidak dapat diurai.
var ErrInvalidCursor = errors.New("cursor tidak valid")

// Encode membuat cursor opaque dari (createdAt, id).
func Encode(createdAt time.Time, id int) string {
	raw := strconv.FormatInt(createdAt.UTC().UnixNano(), 10) + "|" + strconv.Itoa(id)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// Decode mengurai cursor hasil Encode.
func Decode(cursor string) (time.Time, int, error) {
	c := strings.TrimSpace(cursor)
	if c == "" {
		return time.Time{}, 0, ErrInvalidCursor
	}
	b, err := base64.RawURLEncoding.DecodeString(c)
	if err != nil {
		return time.Time{}, 0, ErrInvalidCursor
	}
	parts := strings.SplitN(string(b), "|", 2)
	if len(parts) != 2 {
		return time.Time{}, 0, ErrInvalidCursor
	}
	ns, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return time.Time{}, 0, ErrInvalidCursor
	}
	id, err := strconv.Atoi(parts[1])
	if err != nil {
		return time.Time{}, 0, ErrInvalidCursor
	}
	return time.Unix(0, ns).UTC(), id, nil
}
