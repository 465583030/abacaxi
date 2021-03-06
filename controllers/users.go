package controllers

import (
	"fmt"
	"net/http"
	"time"

	"gopkg.in/mgo.v2/bson"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/microcosm-cc/bluemonday"
	"github.com/nicomo/abacaxi/logger"
	"github.com/nicomo/abacaxi/models"
	"github.com/nicomo/abacaxi/session"
	"github.com/nicomo/abacaxi/views"
)

const (
	// session name to store the number of login attemps
	sessLoginAttempt = "log_attempts"
)

// logAttempt increments a counter of logging attempts in session
// used to prevent bruce force login attempts
func logAttempt(sess *sessions.Session) {
	// log the login attempt
	if sess.Values[sessLoginAttempt] == nil {
		sess.Values[sessLoginAttempt] = 1
	} else {
		sess.Values[sessLoginAttempt] = sess.Values[sessLoginAttempt].(int) + 1
	}
}

// UsersHandler displays the list of existing users
func UsersHandler(w http.ResponseWriter, r *http.Request) {
	// our messages (errors, confirmation, etc) to the user & the template will be store in this map
	d := make(map[string]interface{})

	// Get session
	sess := session.Instance(r)
	if sess.Values["id"] != nil {
		d["IsLoggedIn"] = true
	}

	// get flash messages if any
	if flashes := sess.Flashes(); len(flashes) > 0 {
		d["Flashes"] = flashes
	}
	sess.Save(r, w)

	result, err := models.GetUsers()
	if err != nil {
		logger.Error.Println(err)
	}

	d["users"] = result

	TSListing, _ := models.GetTargetServicesListing()
	d["TSListing"] = TSListing
	d["TSCount"] = len(TSListing)

	views.RenderTmpl(w, "users", d)
}

// UserLoginGetHandler to display user login form
func UserLoginGetHandler(w http.ResponseWriter, r *http.Request) {

	// UI data
	d := make(map[string]interface{})

	// Get session & flash messages if any
	sess := session.Instance(r)
	if flashes := sess.Flashes(); len(flashes) > 0 {
		d["Flashes"] = flashes
	}
	sess.Save(r, w)

	views.RenderTmpl(w, "userlogin", d)
}

// UserLoginPostHandler to parse user login form
func UserLoginPostHandler(w http.ResponseWriter, r *http.Request) {
	// Get session
	sess := session.Instance(r)

	// Prevent brute force login attempt: invalidate request
	if sess.Values[sessLoginAttempt] != nil && sess.Values[sessLoginAttempt].(int) >= 10 {
		sess.AddFlash("Too many login attempts. Account suspended")
		sess.Save(r, w)
		UserLoginGetHandler(w, r)
		return
	}

	// new strict sanitizing policy for the login form
	policy := bluemonday.StrictPolicy()
	// get form values
	username := policy.Sanitize(r.FormValue("username"))
	pw := policy.Sanitize(r.FormValue("password"))

	if username == "" || pw == "" {
		sess.AddFlash("Login attempt missing required field")
		sess.Save(r, w)
		UserLoginGetHandler(w, r)
		return
	}

	// Get user in DB
	user, err := models.UserByUsername(username)
	if err != nil {
		sess.AddFlash("User not found")
		sess.Save(r, w)
		logAttempt(sess)
		UserLoginGetHandler(w, r)
		return
	}

	if session.MatchString(user.Password, pw) {
		// login is successful

		// update date last seen
		user.DateLastSeen = time.Now()
		err := models.UserUpdateDateLastSeen(user)
		if err != nil {
			logger.Error.Println(err)
		}

		// reset session login attempts counter
		sess.Values[sessLoginAttempt] = nil

		// fill session values, save & redirect to home
		sess.Values["id"] = user.ID.Hex()
		sess.Values["username"] = user.Username
		sess.Save(r, w)
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	logAttempt(sess)
	sess.AddFlash("wrong username or password")
	sess.Save(r, w)
	UserLoginGetHandler(w, r)
	return
}

// UserLogoutHandler logs user out
func UserLogoutHandler(w http.ResponseWriter, r *http.Request) {
	// Get session
	sess := session.Instance(r)

	// If user is authenticated we empty the session
	if sess.Values["id"] != nil {
		session.Empty(sess)
		sess.Save(r, w)
	}

	// now logged out, redirect to home
	http.Redirect(w, r, "/", http.StatusFound)
}

// UserNewGetHandler displays the form to create a new user
func UserNewGetHandler(w http.ResponseWriter, r *http.Request) {
	// our messages (errors, confirmation, etc) to the user & the template will be store in this map
	d := make(map[string]interface{})

	// Get session
	sess := session.Instance(r)
	if sess.Values["id"] != nil {
		d["IsLoggedIn"] = true
	}

	TSListing, _ := models.GetTargetServicesListing()
	d["TSListing"] = TSListing
	d["TSCount"] = len(TSListing)

	views.RenderTmpl(w, "usernew", d)
}

// UserNewPostHandler creates a new user
func UserNewPostHandler(w http.ResponseWriter, r *http.Request) {
	// our messages (errors, confirmation, etc) to the user & the template will be store in this map
	d := make(map[string]interface{})

	// new strict sanitizing policy for the user create form
	policy := bluemonday.StrictPolicy()
	// get form values
	username := policy.Sanitize(r.FormValue("username"))
	pw := policy.Sanitize(r.FormValue("password"))

	err := models.UserCreate(username, pw)
	if err != nil {
		logger.Error.Println(err)
		d["userCreateErr"] = err
		logger.Error.Println(err)
		views.RenderTmpl(w, "usernew", d)
		return
	}

	http.Redirect(w, r, "/users", http.StatusFound)
}

// UserDeleteHandler deletes a user
func UserDeleteHandler(w http.ResponseWriter, r *http.Request) {

	// get the user ID to delete from the url
	vars := mux.Vars(r)
	userID := vars["userID"]

	// get the user concerned
	targetUser, err := models.UserByID(userID)
	if err != nil {
		logger.Error.Println(err)
		// redirect to users list
		http.Redirect(w, r, "/users", http.StatusSeeOther)
		return
	}

	// Get session & the logged in user ID
	sess := session.Instance(r)
	currentID := sess.Values["id"]

	fmt.Printf("type of currentID: %T\n", currentID)

	// is the user trying to delete herself?
	if targetUser.ID == bson.ObjectIdHex(currentID.(string)) {
		sess.AddFlash("This app doesn't allow a user to delete herself...")
		sess.Save(r, w)
		// redirect to users list
		http.Redirect(w, r, "/users", http.StatusSeeOther)
		return
	}

	errUserDelete := models.UserDelete(userID)
	if errUserDelete != nil {
		logger.Error.Println(errUserDelete)
		// redirect to users list
		http.Redirect(w, r, "/users", http.StatusSeeOther)
		return
	}

	// redirect to users list
	http.Redirect(w, r, "/users", http.StatusSeeOther)
}
