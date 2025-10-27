package app

import (
	"log"
	"slices"
)

func validateEvent(e string, validEvents ...string) bool {
	if slices.Contains(validEvents, e) {
		return true
	}
	log.Printf("Unknown event found %s\n.", e)
	return false
}
