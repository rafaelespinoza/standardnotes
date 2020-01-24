package logger

import (
	"fmt"
	"log"
	"time"

	"github.com/rafaelespinoza/standardnotes/internal/config"
)

func init() {
	log.SetFlags(0)
	log.SetOutput(new(writer))
}

// TimeFormat is the layout for all logging timestamps in this service.
const TimeFormat = "20060102150405.000"

type writer struct{}

func (w writer) Write(bytes []byte) (int, error) {
	return fmt.Print(time.Now().UTC().Format(TimeFormat) + " " + string(bytes))
}

// LogIfDebug writes to the log only if debug mode is true.
func LogIfDebug(v ...interface{}) {
	if config.Conf.Debug {
		log.Println(v...)
	}
}
