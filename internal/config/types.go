package config

import (
	"crypto/rsa"
	"errors"
	"strings"
)

type (
	Config struct {
		Addr       string
		AppID      string
		Secret     []byte
		PrivateKey *rsa.PrivateKey
		Logging    LoggingConfig
	}

	LoggingConfig struct {
		File    string
		Verbose bool
	}

	EnvNotFoundError struct {
		V []string
	}
)

var ErrMissingPEMBlock = errors.New("no PEM block found")

const (
	defaultAddr = "0.0.0.0:8080"
)

func (e *EnvNotFoundError) Error() string {
	return "missing env vars: " + strings.Join(e.V, ", ")
}
