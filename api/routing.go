package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/go-playground/pure"
	"github.com/rafaelespinoza/standardfile/config"
	"github.com/rafaelespinoza/standardfile/logger"
	"github.com/rafaelespinoza/standardfile/models"
)

type data map[string]interface{}

type sfError struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

func showError(w http.ResponseWriter, err error, code int) {
	log.Println(err)
	pure.JSON(w, code, data{"error": sfError{err.Error(), code}})
}

func authenticateUser(r *http.Request) (models.User, error) {
	var user = models.NewUser()

	authHeaderParts := strings.Split(r.Header.Get("Authorization"), " ")
	if len(authHeaderParts) != 2 || strings.ToLower(authHeaderParts[0]) != "bearer" {
		return user, fmt.Errorf("Missing authorization header")
	}

	token, err := jwt.ParseWithClaims(authHeaderParts[1], &models.UserClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return models.SigningKey, nil
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

// Dashboard - is the root handler
func Dashboard(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Dashboard. Server version: " + config.Metadata.Version))
}

// ChangePassword - is the change password handler
func ChangePassword(w http.ResponseWriter, r *http.Request) {
	user, err := authenticateUser(r)
	if err != nil {
		showError(w, err, http.StatusUnauthorized)
		return
	}
	np := models.NewPassword{}
	if err := pure.Decode(r, true, 104857600, &np); err != nil {
		showError(w, err, http.StatusUnprocessableEntity)
		return
	}

	if len(np.CurrentPassword) == 0 {
		showError(w, fmt.Errorf("your current password is required to change your password. Please update your application if you do not see this option"), http.StatusUnauthorized)
		return
	}

	if _, err := user.Login(np.Email, np.CurrentPassword); err != nil {
		showError(w, fmt.Errorf("the current password you entered is incorrect. Please try again"), http.StatusUnauthorized)
		return
	}

	if err := user.UpdatePassword(np); err != nil {
		showError(w, err, http.StatusInternalServerError)
		return
	}
	// c.Code(http.StatusNoContent).Body("") //in spec, but SN requires token in return
	token, err := user.Login(user.Email, user.Password)
	if err != nil {
		showError(w, err, http.StatusUnauthorized)
		return
	}
	pure.JSON(w, http.StatusAccepted, data{"token": token, "user": user.ToJSON()})
}

//UpdateUser - updates user params
func UpdateUser(w http.ResponseWriter, r *http.Request) {
	user, err := authenticateUser(r)
	if err != nil {
		showError(w, err, http.StatusUnauthorized)
		return
	}
	p := models.Params{}
	if err := pure.Decode(r, true, 104857600, &p); err != nil {
		showError(w, err, http.StatusUnprocessableEntity)
		return
	}
	logger.Log("Request:", p)

	if err := user.UpdateParams(p); err != nil {
		showError(w, err, http.StatusInternalServerError)
		return
	}
	pure.JSON(w, http.StatusAccepted, data{})
}

//Registration - is the registration handler
func Registration(w http.ResponseWriter, r *http.Request) {
	var user = models.NewUser()
	if err := pure.Decode(r, true, 104857600, &user); err != nil {
		showError(w, err, http.StatusUnprocessableEntity)
		return
	}
	logger.Log("Request:", user)
	token, err := user.Register()
	if err != nil {
		showError(w, err, http.StatusUnprocessableEntity)
		return
	}
	pure.JSON(w, http.StatusCreated, data{"token": token, "user": user.ToJSON()})
}

//Login - is the login handler
func Login(w http.ResponseWriter, r *http.Request) {
	var user = models.NewUser()
	if err := pure.Decode(r, true, 104857600, &user); err != nil {
		showError(w, err, http.StatusUnprocessableEntity)
		return
	}
	logger.Log("Request:", user)
	token, err := user.Login(user.Email, user.Password)
	if err != nil {
		showError(w, err, http.StatusUnauthorized)
		return
	}
	pure.JSON(w, http.StatusAccepted, data{"token": token, "user": user.ToJSON()})
}

//GetParams - is the get auth parameters handler
func GetParams(w http.ResponseWriter, r *http.Request) {
	user := models.NewUser()
	email := r.FormValue("email")
	logger.Log("Request:", string(email))
	if email == "" {
		showError(w, fmt.Errorf("Empty email"), http.StatusUnauthorized)
		return
	}
	params := user.GetParams(email)
	if _, ok := params["version"]; !ok {
		showError(w, fmt.Errorf("Invalid email or password"), http.StatusNotFound)
		return
	}
	content, _ := json.MarshalIndent(params, "", "  ")
	logger.Log("Response:", string(content))
	pure.JSON(w, http.StatusOK, params)
}

// SyncItems is the items sync handler.
func SyncItems(w http.ResponseWriter, r *http.Request) {
	user, err := authenticateUser(r)
	if err != nil {
		showError(w, err, http.StatusUnauthorized)
		return
	}
	var request models.SyncRequest
	if err := pure.Decode(r, true, 104857600, &request); err != nil {
		showError(w, err, http.StatusUnprocessableEntity)
		return
	}
	logger.Log("Request:", request)
	response, err := user.SyncItems(request)
	if err != nil {
		showError(w, err, http.StatusInternalServerError)
		return
	}
	content, _ := json.MarshalIndent(response, "", "  ")
	logger.Log("Response:", string(content))
	pure.JSON(w, http.StatusAccepted, response)
}

// BackupItems export items.
func BackupItems(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		showError(w, err, http.StatusInternalServerError)
		return
	}
	fmt.Printf("%+v\n", r.Form)
}
