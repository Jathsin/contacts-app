package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// TODO: use MongoDB
var users = map[string]string{
	"user1": "password1",
	"user2": "password2",
}

var sessions = map[string]session{}

type session struct {
	username string
	expiry   time.Time
}

type credentials struct {
	Password string `json:"password"`
	Username string `json:"username"`
}

func (s session) is_expired() bool {
	return s.expiry.Before(time.Now())
}

func Get_mux() (*http.ServeMux, error) {

	mux := http.NewServeMux()

	mux.HandleFunc("POST /sign-in", sign_in_handler)
	mux.HandleFunc("GET /welcome", welcome_handler)
	mux.HandleFunc("POST /refresh", refresh_handler)
	mux.HandleFunc("POST /logout", logout_handler)

	return mux, nil
}

func sign_in_handler(w http.ResponseWriter, r *http.Request) {

	var cred credentials

	err := json.NewDecoder(r.Body).Decode(&cred)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	expected_password, ok := users[cred.Username]
	if !ok || expected_password != cred.Password {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// create session
	session_token := uuid.NewString()
	expires_at := time.Now().Add(120 * time.Second)
	sessions[session_token] = session{
		username: cred.Username,
		expiry:   expires_at,
	}

	// tell browser
	http.SetCookie(w, &http.Cookie{
		Name:    "session_token",
		Value:   session_token,
		Expires: expires_at,
	})
}

func welcome_handler(w http.ResponseWriter, r *http.Request) {

	// check session validity
	cookie, err := r.Cookie("session_token")
	if err != nil {
		if err == http.ErrNoCookie {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// does the session exist?
	session_token := cookie.Value
	current_session, exists := sessions[session_token]
	if !exists {
		http.Error(w, "session does not exist", http.StatusUnauthorized)
		return
	}
	if current_session.is_expired() {
		delete(sessions, session_token)
		http.Error(w, "session is expired", http.StatusUnauthorized)
		return
	}

	w.Write([]byte(fmt.Sprintf("Welcome %s!", current_session.username)))
}

// Prevents user from login in everytime a session expires
func refresh_handler(w http.ResponseWriter, r *http.Request) {

	// check session validity
	cookie, err := r.Cookie("session_token")
	if err != nil {
		if err == http.ErrNoCookie {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// does the session exist?
	session_token := cookie.Value
	current_session, exists := sessions[session_token]
	if !exists {
		http.Error(w, "session does not exist", http.StatusUnauthorized)
		return
	}
	if current_session.is_expired() {
		delete(sessions, session_token)
		http.Error(w, "session is expired", http.StatusUnauthorized)
		return
	}
	// End of boilerplate code for validating session cookie

	new_session_token := uuid.NewString()
	expires_at := time.Now().Add(120 * time.Second)

	delete(sessions, session_token)

	sessions[new_session_token] = session{
		username: current_session.username,
		expiry:   expires_at,
	}

	http.SetCookie(w, &http.Cookie{
		Name:    "session_token",
		Value:   new_session_token,
		Expires: expires_at,
	})
}

func logout_handler(w http.ResponseWriter, r *http.Request) {
	// check session validity
	cookie, err := r.Cookie("session_token")
	if err != nil {
		if err == http.ErrNoCookie {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	delete(sessions, cookie.Value)
	http.SetCookie(w, &http.Cookie{
		Name:    "session_token",
		Value:   "",
		Expires: time.Now(),
	})
}
