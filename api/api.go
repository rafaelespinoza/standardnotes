package api

import (
	"log"
	"os"

	"github.com/rafaelespinoza/standardfile/config"
	"github.com/rafaelespinoza/standardfile/db"
)

// Serve is the main workhorse of this package. It maps request routes to
// handlers and listens on the configured socket.
func Serve(cfg config.Config) (err error) {
	if _Server != nil {
		log.Println("server already running")
		return
	}

	db.Init(cfg.DB)
	log.Printf("started StandardFile Server\n\tconfig:\n\t%+v\n", cfg)

	_Server, err := newServer(cfg)
	if err != nil {
		log.Println(err)
		return
	}

	defer func(cfg config.Config) {
		if len(cfg.Socket) != 0 {
			os.Remove(cfg.Socket)
		}
	}(cfg)

	go _Server.listen()
	<-_Server.work

	log.Println("server stopped")
	os.Exit(0)
	return
}

// Shutdown closes the server's internal channel.
func Shutdown() {
	if _Server != nil {
		_Server.shutdown()
	}
}
