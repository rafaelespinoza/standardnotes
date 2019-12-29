package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/rafaelespinoza/standardfile/interactors"
	"github.com/rafaelespinoza/standardfile/interactors/itemsync"
	"github.com/rafaelespinoza/standardfile/interactors/token"
	"github.com/rafaelespinoza/standardfile/logger"
	"github.com/rafaelespinoza/standardfile/models"
)

func makeError(err error, code int) map[string]interface{} {
	var serr = err
	if code >= 500 {
		// log the real error, obfuscate the error for end user.
		log.Println(err)
		serr = fmt.Errorf(http.StatusText(code))
	}

	return map[string]interface{}{
		"message": serr.Error(),
		"code":    code,
	}
}

func mustShowError(w http.ResponseWriter, err error, code int) {
	logger.LogIfDebug(err)
	body, merr := json.Marshal(makeError(err, code))
	if merr != nil {
		panic(merr)
	}
	w.WriteHeader(code)
	fmt.Fprintf(w, `{"error": %s}`, string(body))
}

func writeJSONResponse(w http.ResponseWriter, status int, data interface{}) error {
	body, err := json.Marshal(data)
	if err != nil {
		return err
	}

	w.WriteHeader(status)
	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
	return nil
}

func readJSONRequest(r *http.Request, dst interface{}) error {
	// TODO: use buffering
	return json.NewDecoder(r.Body).Decode(dst)
}

func authenticateUser(r *http.Request) (*models.User, error) {
	return token.AuthenticateUser(r.Header.Get("Authorization"))
}

// authHandlers groups http handlers for "/auth/" routes.
var authHandlers = struct {
	ChangePassword http.HandlerFunc
	UpdateUser     http.HandlerFunc
	RegisterUser   http.HandlerFunc
	LoginUser      http.HandlerFunc
	GetParams      http.HandlerFunc
}{
	ChangePassword: ChangePassword,
	UpdateUser:     UpdateUser,
	RegisterUser:   RegisterUser,
	LoginUser:      LoginUser,
	GetParams:      GetParams,
}

// ChangePassword is the change password handler.
// POST /auth/change_pw
func ChangePassword(w http.ResponseWriter, r *http.Request) {
	user, err := authenticateUser(r)
	if err != nil {
		mustShowError(w, err, http.StatusUnauthorized)
		return
	}
	var password models.NewPassword
	if err := readJSONRequest(r, &password); err != nil {
		mustShowError(w, err, http.StatusUnprocessableEntity)
		return
	}

	token, err := interactors.ChangeUserPassword(user, password)
	switch err {
	case nil:
		break
	case
		interactors.ErrNoPasswordProvidedDuringChange,
		interactors.ErrMissingNewAuthParams,
		interactors.ErrPasswordIncorrect:
		mustShowError(w, err, http.StatusUnauthorized)
		return
	default:
		mustShowError(w, err, http.StatusInternalServerError)
		return
	}

	writeJSONResponse(
		w,
		http.StatusAccepted,
		map[string]interface{}{"token": token, "user": user.MakeSaferCopy()},
	)
}

// UpdateUser updates user info.
// POST /auth/update
func UpdateUser(w http.ResponseWriter, r *http.Request) {
	user, err := authenticateUser(r)
	if err != nil {
		mustShowError(w, err, http.StatusUnauthorized)
		return
	}
	p := models.User{}
	if err := readJSONRequest(r, &p); err != nil {
		mustShowError(w, err, http.StatusUnprocessableEntity)
		return
	}
	logger.LogIfDebug("Request: ", p)

	if err := user.Update(p); err != nil {
		mustShowError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSONResponse(w, http.StatusAccepted, nil)
}

// RegisterUser is the registration handler.
// POST /auth/register
func RegisterUser(w http.ResponseWriter, r *http.Request) {
	var user = models.NewUser()
	if err := readJSONRequest(r, &user); err != nil {
		mustShowError(w, err, http.StatusUnprocessableEntity)
		return
	}
	logger.LogIfDebug("Request:", user)
	token, err := interactors.RegisterUser(user)
	if err != nil {
		mustShowError(w, err, http.StatusUnprocessableEntity)
		return
	}
	writeJSONResponse(
		w,
		http.StatusCreated,
		map[string]interface{}{"token": token, "user": user.MakeSaferCopy()},
	)
}

// LoginUser handles sign in.
// POST /auth/sign_in
func LoginUser(w http.ResponseWriter, r *http.Request) {
	var user = models.NewUser()
	if err := readJSONRequest(r, &user); err != nil {
		mustShowError(w, err, http.StatusUnprocessableEntity)
		return
	}
	logger.LogIfDebug("Request:", user)
	token, err := interactors.LoginUser(*user, user.Email, user.Password)
	if err != nil {
		mustShowError(w, err, http.StatusUnauthorized)
		return
	}
	writeJSONResponse(
		w,
		http.StatusAccepted,
		map[string]interface{}{"token": token, "user": user.MakeSaferCopy()},
	)
}

// GetParams is the get auth parameters handler.
// GET /auth/params
func GetParams(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	logger.LogIfDebug("Request:", string(email))
	var params models.PwGenParams
	var err error
	if params, err = interactors.MakeAuthParams(email); err == interactors.ErrInvalidEmail {
		mustShowError(w, err, http.StatusUnauthorized)
		return
	} else if err != nil {
		mustShowError(w, err, http.StatusInternalServerError)
		return
	}
	if v := params.Version; v == "" {
		mustShowError(w, fmt.Errorf("Invalid email or password"), http.StatusNotFound)
		return
	}
	content, _ := json.MarshalIndent(params, "", "  ")
	logger.LogIfDebug("Response:", string(content))
	writeJSONResponse(w, http.StatusOK, params)
}

// itemsHandlers groups http handlers for "/items/" routes.
var itemsHandlers = struct {
	SyncItems   http.HandlerFunc
	BackupItems http.HandlerFunc
}{
	SyncItems:   SyncItems,
	BackupItems: BackupItems,
}

// SyncItems is the items sync handler.
// POST /items/sync
func SyncItems(w http.ResponseWriter, r *http.Request) {
	user, err := authenticateUser(r)
	if err != nil {
		mustShowError(w, err, http.StatusUnauthorized)
		return
	}
	var request itemsync.Request
	if err := readJSONRequest(r, &request); err != nil {
		mustShowError(w, err, http.StatusUnprocessableEntity)
		return
	}
	logger.LogIfDebug("Request:", request)
	response, err := itemsync.SyncUserItems(*user, request)
	if err != nil {
		mustShowError(w, err, http.StatusInternalServerError)
		return
	}
	content, _ := json.MarshalIndent(response, "", "  ")
	logger.LogIfDebug("Response:", string(content))
	writeJSONResponse(w, http.StatusAccepted, response)
}

// BackupItems export items.
// POST /items/backup
func BackupItems(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		mustShowError(w, err, http.StatusInternalServerError)
		return
	}
	fmt.Printf("%+v\n", r.Form)
}
