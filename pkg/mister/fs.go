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

func ActiveGameEnabled() bool {
	_, err := os.Stat(config.ActiveGameFile)
	return err == nil
}

func SetActiveGame(path string) error {
	file, err := os.Create(config.ActiveGameFile)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(path)
	if err != nil {
		return err
	}

	return nil
}
