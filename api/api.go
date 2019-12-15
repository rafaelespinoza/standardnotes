package api

import (
	"net/http"
	"os"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/rafaelespinoza/standardfile/config"
)

const (
	_CertFile string = "path/to/certFile"
	_KeyFile  string = "path/to/keyFile"
)

func RunServer(conf config.Config) {
	server := newServer(conf)

	if os.Getenv("ENV") != "production" {
		server.ListenAndServe()
		return
	}

	server.ListenAndServeTLS(_CertFile, _KeyFile)
}

var itemsHandlers struct {
	Sync   http.Handler
	Backup http.Handler
	List   http.Handler
	Show   http.Handler
	Create http.Handler
	Update http.Handler
	Delete http.Handler
}

var authHandlers struct {
	Params         http.Handler
	UpdateUser     http.Handler
	Login          http.Handler
	ChangePassword http.Handler
	RegisterUser   http.Handler
}

// newServer sets up an http.Server with request handlers.
func newServer(conf config.Config) *http.Server {
	router := mux.NewRouter()
	// Main
	router.HandleFunc("/", Dashboard).Methods(http.MethodGet)

	/* Items */
	router.HandleFunc("/items/sync", itemsHandlers.Sync.ServeHTTP).Methods(http.MethodPost)
	router.HandleFunc("/items/backup", itemsHandlers.Backup.ServeHTTP).Methods(http.MethodPost)
	router.HandleFunc("/items", itemsHandlers.List.ServeHTTP).Methods(http.MethodGet)
	router.HandleFunc("/items/{id}", itemsHandlers.Show.ServeHTTP).Methods(http.MethodGet)
	router.HandleFunc("/items", itemsHandlers.Create.ServeHTTP).Methods(http.MethodPost)
	router.HandleFunc("/items/{id}", itemsHandlers.Update.ServeHTTP).Methods(http.MethodPatch)
	router.HandleFunc("/items/{id}", itemsHandlers.Delete.ServeHTTP).Methods(http.MethodDelete)

	// Auth
	router.HandleFunc("/auth/params", authHandlers.Params.ServeHTTP).Methods(http.MethodGet)
	router.HandleFunc("/auth/update", authHandlers.UpdateUser.ServeHTTP).Methods(http.MethodPost)
	router.HandleFunc("/auth/change_pw", authHandlers.ChangePassword.ServeHTTP).Methods(http.MethodPost)
	router.HandleFunc("/auth/sign_in", authHandlers.Login.ServeHTTP).Methods(http.MethodPost)
	router.HandleFunc("/auth/sign_in.json", authHandlers.Login.ServeHTTP).Methods(http.MethodPost)
	if !conf.NoReg {
		router.HandleFunc("/auth", authHandlers.RegisterUser.ServeHTTP).Methods(http.MethodPost)
	}

	return &http.Server{
		Addr:    conf.Host + ":" + strconv.Itoa(conf.Port),
		Handler: router,
	}
}
