package dto

import (
	"encoding/base64"
	"errors"
	"os"
)

type Config struct {
	DSN         string `json:"DSN"`
	FirebaseKey string `json:"firebase_key"`
	RabbitMQURL string `json:"rabbitmq_url"`
}

func NewConfig() Config {
	return Config{
		DSN:         os.Getenv("DSN"),
		FirebaseKey: os.Getenv("FIREBASE_KEY"),
		RabbitMQURL: os.Getenv("RABBITMQ_URL"),
	}
}

func (c Config) DecodeFirebaseKey() ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(c.FirebaseKey)
	if err != nil {
		return []byte{}, errors.New("error decoding firebase key")
	}

	return key, nil
}
