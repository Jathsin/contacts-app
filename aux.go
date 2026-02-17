package main

import (
	"context"
	"net/http"
	"strings"
	"time"

	et "braces.dev/errtrace"
	"github.com/a-h/templ"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func (app *app) get_contact_page(page int, username string) ([]Contact, error) {

	contacts, err := get_user_contacts_db(app.mongo_client, username)
	if err != nil {
		return nil, et.Wrap(err)
	}

	var contact_set []Contact
	if len(contacts) > 0 {
		p := page - 1
		limit := p*10 + 10
		for i := p * 10; i < limit && i < len(contacts); i++ {
			contact_set = append(contact_set, Contact{
				ID:    contacts[i].ID,
				First: contacts[i].First,
				Last:  contacts[i].Last,
				Email: contacts[i].Email,
				Phone: contacts[i].Phone,
			})
		}
	}

	return contact_set, nil
}

func validate_email(id int, email string, contacts []Contact) string {

	if email == "" {
		return "Email is empty"
	}
	for _, c := range contacts {
		if c.ID != id && c.Email == email {
			return "Email must be unique"
		}
	}
	return ""
}

// MIDDLEWARE ----------------------------------------------------------------------

func logging(f http.Handler) http.Handler {

	return (http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := uuid.New().String()
		log := log.With("request_id", id)

		log.Info("NewRequest",
			"method", r.Method, "url", r.URL.Path,
			"remoteAddress", r.RemoteAddr)

		ctx := context.WithValue(r.Context(), "log", log)
		r = r.WithContext(ctx)
		// Calls actual handler
		f.ServeHTTP(w, r)
	}))
}

// Auth
// It must be done this way to avoid collisions, it is an inherent Go practice
type ctx_key string

const user_key ctx_key = "username"

func redirect_register(w http.ResponseWriter, r *http.Request) {
	// If the request was made by htmx, force a full-page navigation instead of swapping into hx-target.
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/register")
		// 401 makes it clear the request is unauthorized; htmx will still follow HX-Redirect.
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// Non-htmx request: normal browser redirect.
	http.Redirect(w, r, "/register", http.StatusSeeOther)
}

func (app *app) auth(f http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if r.URL.Path == "/" || r.URL.Path == "/sign-in" || r.URL.Path == "/register" || strings.HasPrefix(r.URL.Path, "/static/") {
			f.ServeHTTP(w, r)
			return
		}

		// check session validity
		cookie, err := r.Cookie("session_token")
		if err == http.ErrNoCookie {
			err = delete_expired_sessions(app.mongo_client)
			if err != nil {
				http.Error(w, "Error deleting session", http.StatusInternalServerError)
				log.Error("auth: error in delete_expired_sessions", "error", err)
			}
			redirect_register(w, r)
			return
		} else if err != nil {
			log.Error("auth: error retrieving session cookie", "error", err)
			redirect_register(w, r)
			return
		}

		// does the session exist?
		session_token := cookie.Value
		current_session, err := find_session(app.mongo_client, session_token)
		if err != nil {
			http.Error(w, "Error finding session", http.StatusInternalServerError)
			log.Error("auth: error in find_session", "error", err)
			redirect_register(w, r)
			return
		}

		if current_session == nil {
			redirect_register(w, r)
			return
		}

		if time.Until(current_session.Expiry) < 30*time.Second {

			// extend session
			expires_at := time.Now().Add(120 * time.Second)

			err = update_session_expiry(app.mongo_client, session_token, expires_at)
			if err != nil {
				http.Error(w, "Error updating session expiry", http.StatusInternalServerError)
				log.Error("auth: error in update_session_expiry", "error", err)
				redirect_register(w, r)
				return
			}

			http.SetCookie(w, &http.Cookie{
				Name:    "session_token",
				Value:   session_token,
				Expires: expires_at,
			})

		} else if current_session.is_expired() {
			// even if the browser doesn´t, some user might send expired cookies
			redirect_register(w, r)
			return
		}

		// We pass the username to the handler through the context, so handlers can know which user is performing the request and act accordingly
		ctx := context.WithValue(r.Context(), user_key, current_session.Username)
		f.ServeHTTP(w, r.WithContext(ctx))
	})
}

func get_username(ctx context.Context) string {
	username, ok := ctx.Value(user_key).(string)
	if !ok {
		return ""
	}
	return username
}

// Others  ---------------------------------------------------------------------

// Convert results from Contact_db to  Contact
func get_user_contacts(mongo_client *mongo.Client, username string) ([]Contact, error) {

	user_contacts_db, err := get_user_contacts_db(mongo_client, username)
	if err != nil {
		return nil, et.Wrap(err)
	}

	var contacts []Contact
	for _, c := range user_contacts_db {
		contacts = append(contacts, Contact{
			ID:    c.ID,
			First: c.First,
			Last:  c.Last,
			Email: c.Email,
			Phone: c.Phone,
		})
	}
	return contacts, nil
}

func get_auth_or_profile(r *http.Request) templ.Component {
	username := get_username(r.Context())
	if username != "" {
		return profile_card(username)
	}
	return auth_dialog()
}
