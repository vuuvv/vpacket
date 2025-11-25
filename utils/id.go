package utils

import "github.com/google/uuid"

func GenId() string {
	return uuid.New().String()
}
