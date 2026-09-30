package mister

import (
	"os"

	"github.com/synrais/SAMenu/pkg/config"
)

func GetActiveCoreName() (string, error) {
	data, err := os.ReadFile(config.CoreNameFile)
	if err != nil {
		return "", err
	}

	return string(data), nil
}
