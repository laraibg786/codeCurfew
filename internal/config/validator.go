package config

import (
	"net"
	"os"
)

func validateAddr(addr string) (err error) {
	_, _, err = net.SplitHostPort(addr)
	return
}

func validateLogFileWriteable(path string) (err error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	return f.Close()
}
