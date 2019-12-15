package logger

import (
	"log"
)

// Log writes in log if debug flag is set
func Log(v ...interface{}) {
	var debug bool // TODO: figure out
	if debug {
		log.Println(v...)
	}
}
