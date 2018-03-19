package repository

import (
	"log"
	"os"
)

func pathExists(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	} else if err != nil {
		log.Panic(err)
	}
	return true
}
