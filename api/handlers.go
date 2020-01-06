package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/rafaelespinoza/standardnotes/errs"
	"github.com/rafaelespinoza/standardnotes/interactors/itemsync"
	userInteractors "github.com/rafaelespinoza/standardnotes/interactors/user"
	"github.com/rafaelespinoza/standardnotes/logger"
	"github.com/rafaelespinoza/standardnotes/models"
)

func sanitizeAuthError(e error) bool {
	return errs.ValidationError(e) || errs.NotFoundError(e)
}

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
	logger.LogIfDebug(fmt.Sprintf("%d %#v", code, err))
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
	return userInteractors.AuthenticateUser(r.Header.Get("Authorization"))
}

// authHandlers groups http handlers for "/auth/" routes.
var authHandlers = struct {
	changePassword http.HandlerFunc
	updateUser     http.HandlerFunc
	registerUser   http.HandlerFunc
	loginUser      http.HandlerFunc
	getParams      http.HandlerFunc
}{
	changePassword: changePassword,
	updateUser:     updateUser,
	registerUser:   registerUser,
	loginUser:      loginUser,
	getParams:      getParams,
}

// changePassword is the change password handler.
// POST /auth/change_pw
func changePassword(w http.ResponseWriter, r *http.Request) {
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
	token, err := userInteractors.ChangeUserPassword(user, password)
	if errs.ValidationError(err) {
		mustShowError(w, err, http.StatusUnauthorized)
		return
	} else if err != nil {
		mustShowError(w, err, http.StatusInternalServerError)
		return
	}

	writeJSONResponse(
		w,
		http.StatusAccepted,
		map[string]interface{}{"token": token, "user": user.MakeSaferCopy()},
	)
}

// updateUser updates user info.
// POST /auth/update
func updateUser(w http.ResponseWriter, r *http.Request) {
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

// registerUser is the registration handler.
// POST /auth/register
func registerUser(w http.ResponseWriter, r *http.Request) {
	var params userInteractors.RegisterUserParams

	if err := readJSONRequest(r, &params); err != nil {
		mustShowError(w, err, http.StatusUnprocessableEntity)
		return
	}
	logger.LogIfDebug("Request:", params)
	user, token, err := userInteractors.RegisterUser(params)
	if err != nil {
		mustShowError(w, err, http.StatusUnprocessableEntity)
		return
	}
	writeJSONResponse(
		w,
		http.StatusOK,
		map[string]interface{}{"token": token, "user": user.MakeSaferCopy()},
	)
}

// loginUser handles sign in.
// POST /auth/sign_in
func loginUser(w http.ResponseWriter, r *http.Request) {
	var params struct {
		API      string `json:"api"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := readJSONRequest(r, &params); err != nil {
		mustShowError(w, err, http.StatusUnprocessableEntity)
		return
	}
	logger.LogIfDebug("Request:", params)
	user, token, err := userInteractors.LoginUser(
		params.Email,
		&models.PwHash{Value: params.Password},
	)
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

// getParams is the get auth parameters handler.
// GET /auth/params
func getParams(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	logger.LogIfDebug("Request:", string(email))
	var params models.PwGenParams
	var err error
	if params, err = userInteractors.MakeAuthParams(email); sanitizeAuthError(err) {
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
	syncItems   http.HandlerFunc
	backupItems http.HandlerFunc
}{
	syncItems:   syncItems,
	backupItems: backupItems,
}

// syncItems is the items sync handler.
// POST /items/sync
func syncItems(w http.ResponseWriter, r *http.Request) {
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

// backupItems export items.
// POST /items/backup
func backupItems(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		mustShowError(w, err, http.StatusInternalServerError)
		return
	}
	fmt.Printf("%+v\n", r.Form)
}
