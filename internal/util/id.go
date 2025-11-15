package util

import (
	"github.com/lithammer/shortuuid/v4"
)

func GenID() string {
	return shortuuid.New()
}

func GenIDWith(prefix string) string {
	return prefix + shortuuid.New()
}
