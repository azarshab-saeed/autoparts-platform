package pagination

import (
	"encoding/base64"
	"errors"
	"strconv"
)

const DefaultLimit = 30
const MaxLimit = 100

func Limit(raw string) (int, error) {
	if raw == "" {
		return DefaultLimit, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > MaxLimit {
		return 0, errors.New("limit must be between 1 and 100")
	}
	return n, nil
}

func EncodeOffset(offset int) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(offset)))
}

func DecodeOffset(cursor string) (int, error) {
	if cursor == "" {
		return 0, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, errors.New("invalid cursor")
	}
	n, err := strconv.Atoi(string(b))
	if err != nil || n < 0 {
		return 0, errors.New("invalid cursor")
	}
	return n, nil
}
