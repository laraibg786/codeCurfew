package config

import (
	"crypto/rsa"
	"fmt"
	"strings"
)

type (
	Config struct {
		Addr    string
		AppID   string
		Secret  []byte
		KeyPEM  *rsa.PrivateKey
		Logging LoggingConfig
	}

	LoggingConfig struct {
		File    string
		Verbose bool
	}

	EnvNotFoundError struct {
		V []string
	}
)

var ErrMissingPEMBlock = fmt.Errorf("no PEM block found")

func (e *EnvNotFoundError) Error() string {
	return "missing env vars: " + strings.Join(e.V, ", ")
}
