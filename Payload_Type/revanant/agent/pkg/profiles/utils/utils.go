package utils

import (
	"log"
)

var debug bool

func PrintDebug(msg string) {
	if debug {
		log.Printf("[DEBUG] %s\n", msg)
	}

}
