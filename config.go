package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL string
	Port        int
}

func loadConfig() (Config, error) {
	godotenv.Load()
	dbURL := os.Getenv("DATABASE_URL")
	if strings.TrimSpace(dbURL) == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}

	port := os.Getenv("PORT")
	var portInt int
	if strings.TrimSpace(port) == "" {
		portInt = 8080
	} else {
		val, err := strconv.Atoi(port)
		if err != nil {
			return Config{}, fmt.Errorf("invalid PORT value %q: must be a valid integer between 0 and 65535", port)
		}
		if val < 1 || val > 65535 {
			return Config{}, fmt.Errorf("PORT value %q is out of range: must be between 1 and 65535", port)
		}

		portInt = val
	}

	config := Config{
		DatabaseURL: dbURL,
		Port:        portInt,
	}
	return config, nil
}
