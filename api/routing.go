package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/rafaelespinoza/standardfile/config"
	"github.com/rafaelespinoza/standardfile/encryption"
	"github.com/rafaelespinoza/standardfile/interactors"
	"github.com/rafaelespinoza/standardfile/logger"
	"github.com/rafaelespinoza/standardfile/models"
)

type sfError struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

func showError(w http.ResponseWriter, err error, code int) {
	log.Println(err)
	body, perr := json.Marshal(
		sfError{
			err.Error(),
			code,
		},
	)
	if perr != nil {
		panic(perr)
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
	w.Write(body)
	return nil
}

func readJSONRequest(r *http.Request, dst interface{}) error {
	// TODO: use buffering
	return json.NewDecoder(r.Body).Decode(dst)
}

func authenticateUser(r *http.Request) (*models.User, error) {
	var user = models.NewUser()

	authHeaderParts := strings.Split(r.Header.Get("Authorization"), " ")
	if len(authHeaderParts) != 2 || strings.ToLower(authHeaderParts[0]) != "bearer" {
		return user, fmt.Errorf("Missing authorization header")
	}

	token, err := jwt.ParseWithClaims(authHeaderParts[1], &models.UserClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return encryption.SigningKey, nil
	})

	if err != nil {
		return user, err
	}

	if claims, ok := token.Claims.(*models.UserClaims); ok && token.Valid {
		logger.Log("Token is valid, claims: ", claims)

		if ok := user.LoadByUUID(claims.UUID); !ok {
			return user, fmt.Errorf("Unknown user")
		}

		if user.Validate(claims.PwHash) {
			return user, nil
		}
	}

	return user, fmt.Errorf("Invalid token")
}

// Dashboard is the root handler.
// GET /
func Dashboard(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Dashboard. Server version: " + config.Metadata.Version))
}

// ChangePassword is the change password handler.
// POST /auth/change_pw
func ChangePassword(w http.ResponseWriter, r *http.Request) {
	// TODO: move to interactor. This function should only do http.
	user, err := authenticateUser(r)
	if err != nil {
		showError(w, err, http.StatusUnauthorized)
		return
	}
	password := models.NewPassword{}
	if err := readJSONRequest(r, &password); err != nil {
		showError(w, err, http.StatusUnprocessableEntity)
		return
	}

	if len(password.CurrentPassword) == 0 {
		showError(w, fmt.Errorf("your current password is required to change your password. Please update your application if you do not see this option"), http.StatusUnauthorized)
		return
	}

	if _, err := user.Login(password.Email, password.CurrentPassword); err != nil {
		showError(w, fmt.Errorf("the current password you entered is incorrect. Please try again"), http.StatusUnauthorized)
		return
	}

	if err := user.UpdatePassword(password); err != nil {
		showError(w, err, http.StatusInternalServerError)
		return
	}
	// c.Code(http.StatusNoContent).Body("") //in spec, but SN requires token in return
	token, err := user.Login(user.Email, user.Password)
	if err != nil {
		showError(w, err, http.StatusUnauthorized)
		return
	}
	writeJSONResponse(
		w,
		http.StatusAccepted,
		map[string]interface{}{"token": token, "user": user.ToJSON()},
	)
}

// UpdateUser updates user info.
// POST /auth/update
func UpdateUser(w http.ResponseWriter, r *http.Request) {
	user, err := authenticateUser(r)
	if err != nil {
		showError(w, err, http.StatusUnauthorized)
		return
	}
	p := models.Params{}
	if err := readJSONRequest(r, &p); err != nil {
		showError(w, err, http.StatusUnprocessableEntity)
		return
	}
	logger.Log("Request:", p)

	if err := user.UpdateParams(p); err != nil {
		showError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSONResponse(w, http.StatusAccepted, nil)
}

// Registration is the registration handler.
// POST /auth/register
func Registration(w http.ResponseWriter, r *http.Request) {
	var user = models.NewUser()
	if err := readJSONRequest(r, &user); err != nil {
		showError(w, err, http.StatusUnprocessableEntity)
		return
	}
	logger.Log("Request:", user)
	token, err := user.Register()
	if err != nil {
		showError(w, err, http.StatusUnprocessableEntity)
		return
	}
	writeJSONResponse(
		w,
		http.StatusCreated,
		map[string]interface{}{"token": token, "user": user.ToJSON()},
	)
}

// Login handles sign in.
// POST /auth/sign_in
func Login(w http.ResponseWriter, r *http.Request) {
	var user = models.NewUser()
	if err := readJSONRequest(r, &user); err != nil {
		showError(w, err, http.StatusUnprocessableEntity)
		return
	}
	logger.Log("Request:", user)
	token, err := user.Login(user.Email, user.Password)
	if err != nil {
		showError(w, err, http.StatusUnauthorized)
		return
	}
	writeJSONResponse(
		w,
		http.StatusAccepted,
		map[string]interface{}{"token": token, "user": user.ToJSON()},
	)
}

// GetParams is the get auth parameters handler.
// GET /auth/params
func GetParams(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	logger.Log("Request:", string(email))
	var params models.Params
	var err error
	if params, err = interactors.MakeAuthParams(email); err == interactors.ErrInvalidEmail {
		showError(w, err, http.StatusUnauthorized)
		return
	} else if err != nil {
		showError(w, err, http.StatusInternalServerError)
		return
	}
	if v := params.Version; v == "" {
		showError(w, fmt.Errorf("Invalid email or password"), http.StatusNotFound)
		return
	}
	content, _ := json.MarshalIndent(params, "", "  ")
	logger.Log("Response:", string(content))
	writeJSONResponse(w, http.StatusOK, params)
}

// SyncItems is the items sync handler.
// POST /items/sync
func SyncItems(w http.ResponseWriter, r *http.Request) {
	user, err := authenticateUser(r)
	if err != nil {
		showError(w, err, http.StatusUnauthorized)
		return
	}
	var request models.SyncRequest
	if err := readJSONRequest(r, &request); err != nil {
		showError(w, err, http.StatusUnprocessableEntity)
		return
	}
	logger.Log("Request:", request)
	response, err := interactors.SyncUserItems(*user, request)
	if err != nil {
		showError(w, err, http.StatusInternalServerError)
		return
	}
	content, _ := json.MarshalIndent(response, "", "  ")
	logger.Log("Response:", string(content))
	writeJSONResponse(w, http.StatusAccepted, response)
}

// BackupItems export items.
// POST /items/backup
func BackupItems(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		showError(w, err, http.StatusInternalServerError)
		return
	}
	fmt.Printf("%+v\n", r.Form)
}
