package main

import (
	"encoding/json"
	"hypermedia/archiver"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	et "braces.dev/errtrace"
	"github.com/a-h/templ"
	"github.com/google/uuid"
)

var log *slog.Logger

var myArchiver archiver.Archiver

var SUCCESS = "Contact added successfully"
var DELETE = "Contact deleted successfully"

// TODO: use MongoDB
var users = map[string]string{
	"user1": "password1",
	"user2": "password2",
}

func templ_error(r *http.Request, err error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: checkpr wrap
		log.Error("error", "error", err)
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusInternalServerError)
	})
}

func init_app() (app, error) {
	client, err := get_mongo_client()
	if err != nil {
		return app{}, et.Wrap(err)
	}

	return app{mongo_client: client}, nil
}

func main() {

	// Init logger
	log = slog.New(slog.NewJSONHandler(os.Stdout, nil))

	app, err := init_app()
	if err != nil {
		log.Error("Error initializing app", "error", err)
		return
	}

	// Set default archvier status for user
	myArchiver = *archiver.Get()

	mux := http.NewServeMux()

	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// hypermedia api
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/register", http.StatusFound)
	})

	mux.HandleFunc("GET /contacts", app.contact_query_handler)

	mux.HandleFunc("GET /contacts/{id}", app.contact_id_handler)

	mux.HandleFunc("GET /contacts/new", app.get_add_contact_handler)

	mux.HandleFunc("POST /contacts/new", app.post_add_contact_handler)

	mux.HandleFunc("GET /contacts/{id}/edit", app.get_edit_contact_handler)

	mux.HandleFunc("POST /contacts/{id}/edit", app.post_edit_contact_handler)

	mux.HandleFunc("DELETE /contacts/{id}", app.delete_contact_handler)

	mux.HandleFunc("DELETE /contacts", app.delete_multiple_contacts_handler)

	mux.HandleFunc("GET /contacts/count", app.count_contacts_handler)

	mux.HandleFunc("GET /contacts/{id}/email", app.validate_email_handler)

	mux.HandleFunc("POST /contacts/archive", app.post_archive_handler)

	mux.HandleFunc("GET /contacts/archive", app.get_archive_handler)

	mux.HandleFunc("DELETE /contacts/archive", app.delete_archive_handler)

	mux.HandleFunc("GET /contacts/archive/file", app.archive_file_handler)

	// Auth api

	mux.HandleFunc("POST /sign-in", app.sign_in_handler)

	mux.HandleFunc("POST /logout", app.logout_handler)

	mux.HandleFunc("GET /register", app.get_register_handler)

	mux.HandleFunc("POST /register", app.post_register_handler)

	// json api
	mux.HandleFunc("GET /api/v1/contacts", app.get_contacts_handler)

	mux.HandleFunc("POST /api/v1/contacts", app.post_contacts_handler)

	mux.HandleFunc("GET /api/v1/contacts/{id}", app.get_contact_handler)

	mux.HandleFunc("PUT /api/v1/contacts/{id}", app.put_contact_handler)

	mux.HandleFunc("DELETE /api/v1/contacts/{id}", app.delete_contact_handler_json)

	// Start server
	server := http.Server{
		Addr:         ":8080",
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 90 * time.Second,
		Handler:      auth(logging(mux)),
	}
	err = server.ListenAndServe()
	if err != nil {
		log.Error("Error in server.ListenAndServe", "error", err)
		return
	}
}

//------------------------------------------------------------------------------
// Hypermedia Api
//------------------------------------------------------------------------------

// /contacts?q={id}
func (app *app) contact_query_handler(w http.ResponseWriter, r *http.Request) {

	q := r.URL.Query().Get("q")

	if q == "" {

		// Show first 10 contacts
		page_string := r.URL.Query().Get("page")
		page, _ := strconv.Atoi(page_string)

		if page <= 0 {
			page = 1
		}

		contact_list, err := app.get_contact_page(page, get_username(r.Context()))
		if err != nil {
			http.Error(w, "Error getting contacts", http.StatusInternalServerError)
			log.Error("contact_query_handler: error in get_contact_list", "error", err)
			return
		}

		if r.Header.Get("HX-Request") == "true" {
			templ.Handler(index(contact_list, "", page, myArchiver, ""), templ.WithErrorHandler(templ_error)).ServeHTTP(w, r)
		} else {
			templ.Handler(layout(con_boton_tema(index(contact_list, "", page, myArchiver, ""), get_auth_or_profile(r))), templ.WithErrorHandler(templ_error)).ServeHTTP(w, r)
		}
		return
	}

	// Search for specific contact
	contact_results, err := find_contacts(app.mongo_client, get_username(r.Context()), q)
	if err != nil {
		http.Error(w, "Error, contact not found", http.StatusBadRequest)
		log.Error("contact_query_handler: error in find_contact", "error", err)
		return
	}

	// Show contact information
	templ.Handler(index(contact_results, q, 0, myArchiver, ""), templ.WithErrorHandler(templ_error)).ServeHTTP(w, r)
}

// /contacts/{id}
func (app *app) contact_id_handler(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "text/html")

	id_string := r.PathValue("id")

	if id_string == "" {

		// Show first 10 contacts
		page_string := r.URL.Query().Get("page")
		page, _ := strconv.Atoi(page_string)
		if page <= 0 {
			page = 1
		}

		// Show contact information depending on trigger
		// var err error
		contact_list, err := app.get_contact_page(page, get_username(r.Context()))
		if err != nil {
			http.Error(w, "Error getting contacts", http.StatusInternalServerError)
			log.Error("contact_id_handler: error in get_contact_list", "error", err)
			return
		}

		if r.Header.Get("HX-Trigger") == "search" {
			templ.Handler(rows(contact_list), templ.WithErrorHandler(templ_error)).ServeHTTP(w, r)
		} else {
			templ.Handler(index(contact_list, "", page, myArchiver, ""), templ.WithErrorHandler(templ_error)).ServeHTTP(w, r)
		}

		return
	}

	// Parse id
	id_int, err := strconv.Atoi(id_string)
	if err != nil {
		http.Error(w, "Error, id must be an integer", http.StatusBadRequest)
		log.Error("contact_id_handler: error in strconv.Atoi(id)", "error", err)
		return
	}

	// Search for specific contact
	c, err := find_contact_id(app.mongo_client, get_username(r.Context()), id_int)
	if err != nil {
		http.Error(w, "Error, contact not found", http.StatusBadRequest)
		log.Error("contact_id_handler: error in find_contact", "error", err)
		return
	}

	// Show contact information
	if r.Header.Get("HX-Request") == "true" {
		templ.Handler(show(*c), templ.WithErrorHandler(templ_error)).ServeHTTP(w, r)
	} else {
		templ.Handler(layout(con_boton_tema(show(*c), get_auth_or_profile(r))), templ.WithErrorHandler(templ_error)).ServeHTTP(w, r)
	}

	// s := show(*c)
	// err = s.Render(context.Background(), w)
	// // err = app.Templates.Render(w, "show", c)
	// if err != nil {
	// 	http.Error(w, "Error showing contact", http.StatusBadRequest)
	// 	log.Error("contact_id_handler: error in app.Templates.Render()", "error", err)
	// 	return
	// }
}

// GET /contacts/new
func (app *app) get_add_contact_handler(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "text/html")

	var c = Contact{}
	if r.Header.Get("HX-Request") == "true" {
		templ.Handler(new_contact(c), templ.WithErrorHandler(templ_error)).ServeHTTP(w, r)

	} else {
		templ.Handler(layout(new_contact(c)), templ.WithErrorHandler(templ_error)).ServeHTTP(w, r)
	}
	// err := app.Templates.Render(w, "new", c)
	// if err != nil {
	// 	http.Error(w, "Error, could not render page", http.StatusInternalServerError)
	// 	log.Error("add_contact_get_handler: error in app.Templates.Render()", "error", err)
	// 	return
	// }

}

// POST /contacts/new
func (app *app) post_add_contact_handler(w http.ResponseWriter, r *http.Request) {

	// Compute new id
	contacts, err := get_user_contacts(app.mongo_client, get_username(r.Context()))
	if err != nil {
		http.Error(w, "Error getting contacts", http.StatusInternalServerError)
		log.Error("post_add_contact_handler: error in get_contacts_from_user", "error", err)
		return
	}
	max_id := 0
	if contacts != nil {
		max_id = contacts[len(contacts)-1].ID
	}

	// Get form values
	c := Contact{
		ID:     max_id + 1,
		First:  r.FormValue("first_name"),
		Last:   r.FormValue("last_name"),
		Email:  r.FormValue("email"),
		Phone:  r.FormValue("phone"),
		Errors: make(map[string]string),
	}

	email_error := validate_email(-1, c.Email, contacts)
	if email_error != "" {
		// We must check this in order to keep the map length to zero when
		// no errors are found
		c.Errors["email"] = email_error
	}
	if c.First == "" {
		c.Errors["first"] = "First name is required"
	}
	if c.Last == "" {
		c.Errors["last"] = "Last name is required"
	}
	if c.Phone == "" {
		c.Errors["phone"] = "Phone is required"
	}

	w.Header().Set("Content-Type", "text/html")
	if len(c.Errors) == 0 {
		// Insert contact in DB
		err := insert_contact(app.mongo_client, c, get_username(r.Context()))
		if err != nil {
			http.Error(w, "Error inserting contact in DB", http.StatusInternalServerError)
			log.Error("post_add_contact_handler: error in insert_contact()", "error", err)
			return
		}

		log.Info("Contact added successfully")

		contact_list, err := app.get_contact_page(1, get_username(r.Context()))
		if err != nil {
			http.Error(w, "Error getting contacts", http.StatusInternalServerError)
			log.Error("post_add_contact_handler: error in get_contact_list", "error", err)
			return
		}

		// Inform user
		templ.Handler(index(contact_list, "", 1, myArchiver, SUCCESS), templ.WithErrorHandler(templ_error)).ServeHTTP(w, r)

		return
	}

	// We cannot add contact
	templ.Handler(new_contact(c), templ.WithErrorHandler(templ_error)).ServeHTTP(w, r)

}

// GET /contacts/{id}/edit
func (app *app) get_edit_contact_handler(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "text/html")

	// Parse id
	id_string := r.PathValue("id")
	id_int, err := strconv.Atoi(id_string)
	if err != nil {
		http.Error(w, "Error, id must be an integer", http.StatusBadRequest)
		log.Error("edit_contact_post_handler: error in strconv.Atoi(id)", "error", err)
		return
	}
	// Search for contact to edit
	c, err := find_contact_id(app.mongo_client, get_username(r.Context()), id_int)
	if err != nil {
		http.Error(w, "Error, contact not found", http.StatusBadRequest)
		log.Error("edit_contact_get_handler: error in find_contact", "error", err)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		templ.Handler(edit_contact(*c), templ.WithErrorHandler(templ_error)).ServeHTTP(w, r)
	} else {
		templ.Handler(layout(con_boton_tema(edit_contact(*c), get_auth_or_profile(r))), templ.WithErrorHandler(templ_error)).ServeHTTP(w, r)
	}
}

// POST /contacts/{id}/edit
func (app *app) post_edit_contact_handler(w http.ResponseWriter, r *http.Request) {

	id_string := r.PathValue("id")
	// Parse id
	id_int, err := strconv.Atoi(id_string)
	if err != nil {
		http.Error(w, "Error, id must be an integer", http.StatusBadRequest)
		log.Error("edit_contact_post_handler: error in strconv.Atoi(id)", "error", err)
		return
	}

	// Get form values
	c := Contact{
		ID:     id_int,
		First:  r.FormValue("first_name"),
		Last:   r.FormValue("last_name"),
		Email:  r.FormValue("email"),
		Phone:  r.FormValue("phone"),
		Errors: make(map[string]string),
	}

	contacts, err := get_user_contacts(app.mongo_client, get_username(r.Context()))
	if err != nil {
		http.Error(w, "Error getting contacts", http.StatusInternalServerError)
		log.Error("edit_contact_post_handler: error in get_contacts_from_user_contacts_db", "error", err)
		return
	}

	email_error := validate_email(id_int, c.Email, contacts)
	if email_error != "" {
		// We must check this in order to keep the map length to zero when
		// no errors are found
		c.Errors["email"] = email_error
	}
	if c.First == "" {
		c.Errors["first"] = "First name is required"
	}
	if c.Last == "" {
		c.Errors["last"] = "Last name is required"
	}
	if c.Phone == "" {
		c.Errors["phone"] = "Phone is required"
	}

	if len(c.Errors) == 0 {
		// Update contact in DB
		err := update_contact(app.mongo_client, c, get_username(r.Context()))
		if err != nil {
			http.Error(w, "Error updating contact in DB", http.StatusInternalServerError)
			log.Error("edit_contact_post_handler: error in update_contact()", "error", err)
			return
		}

		log.Info("Contact edited successfully")

		templ.Handler(show(c), templ.WithErrorHandler(templ_error)).ServeHTTP(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	templ.Handler(edit_contact(c), templ.WithErrorHandler(templ_error)).ServeHTTP(w, r)
}

// DELETE /contacts/{id}/edit
func (app *app) delete_contact_handler(w http.ResponseWriter, r *http.Request) {

	id_string := r.PathValue("id")
	// Parse id
	id_int, err := strconv.Atoi(id_string)
	if err != nil {
		http.Error(w, "Error, id must be an integer", http.StatusBadRequest)
		log.Error("delete_contact_handler: error in strconv.Atoi(id)", "error", err)
		return
	}

	contact, err := find_contact_id(app.mongo_client, get_username(r.Context()), id_int)
	if err != nil {
		http.Error(w, "Error finding contact", http.StatusInternalServerError)
		log.Error("delete_contact_handler: error in find_contact_id", "error", err)
		return
	}
	if contact != nil {

		// Delete contact
		err = delete_contact(app.mongo_client, get_username(r.Context()), id_int)
		if err != nil {
			http.Error(w, "Error deleting contact", http.StatusInternalServerError)
			log.Error("delete_contact_handler: error in delete_contact", "error", err)
			return
		}

		log.Info("Contact deleted successfully")

		// Tell htmx clients that contacts changed so widgets like the count can refresh.
		w.Header().Set("HX-Trigger", "contacts-changed")

		if r.Header.Get("HX-Trigger") == "delete-btn" {

			contacts_list, err := app.get_contact_page(1, get_username(r.Context()))
			if err != nil {
				http.Error(w, "Error getting contacts", http.StatusInternalServerError)
				log.Error("delete_contact_handler: error in get_contact_list", "error", err)
				return
			}
			templ.Handler(index(contacts_list, "", 1, myArchiver, DELETE), templ.WithErrorHandler(templ_error)).ServeHTTP(w, r)
			http.Redirect(w, r, "/contacts", http.StatusSeeOther)
		}
		// We do not want to render anything
		return
	}

	// We cannot delete contact, which should always be possible form this endpoint
	http.Error(w, "Error deleting contact", http.StatusInternalServerError)
	log.Error("delete_contact_handler: contact not found")
}

// /contacts/count
func (app *app) count_contacts_handler(w http.ResponseWriter, r *http.Request) {

	time.Sleep(1 * time.Second)

	contacts, err := get_user_contacts_db(app.mongo_client, get_username(r.Context()))
	if err != nil {
		http.Error(w, "Error getting contacts", http.StatusInternalServerError)
		log.Error("count_contacts_handler: error in get_contacts_from_user", "error", err)
		return
	}
	count := len(contacts)

	_, err = w.Write([]byte(strconv.Itoa(count) + " total Contacts"))
	if err != nil {
		http.Error(w, "Error, could not write response", http.StatusInternalServerError)
		log.Error("count_contacts_handler: error in  w.Write()", "error", err)
		return
	}
}

// DELETE /contacts
func (app *app) delete_multiple_contacts_handler(w http.ResponseWriter, r *http.Request) {

	// Parse ids
	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Error processing submitted data", http.StatusBadRequest)
		log.Error("delete_multiple_contacts_handler: error in r.ParseForm()", "error", err)
		return
	}
	ids := r.Form["selected_contact_ids"]

	var ids_int []int

	// Parse array
	for i, id_string := range ids {
		id_int, err := strconv.Atoi(id_string)
		if err != nil {
			http.Error(w, "Error, id nº "+strconv.Itoa(i)+", id must be an integer", http.StatusBadRequest)
			log.Error("delete_multiple_contacts_handler: error in strconv.Atoi(id)", "error", err)
			return
		}
		ids_int = append(ids_int, id_int)
	}

	// Delete selected contacts
	for _, id_int := range ids_int {
		err = delete_contact(app.mongo_client, get_username(r.Context()), id_int)
		if err != nil {
			http.Error(w, "Error deleting contact with id "+strconv.Itoa(id_int), http.StatusInternalServerError)
			log.Error("delete_multiple_contacts_handler: error in delete_contact", "error", err, "id", id_int)
			return
		}
	}

	w.Header().Set("Content-Type", "text/html")

	contact_list, err := app.get_contact_page(1, get_username(r.Context()))
	if err != nil {
		http.Error(w, "Error getting contacts", http.StatusInternalServerError)
		log.Error("delete_multiple_contacts_handler: error in get_contact_list", "error", err)
		return
	}

	templ.Handler(index(contact_list, "", 1, myArchiver, ""), templ.WithErrorHandler(templ_error)).ServeHTTP(w, r)
}

// /contacts/{id}/{email}
func (app *app) validate_email_handler(w http.ResponseWriter, r *http.Request) {

	id_string := r.PathValue("id")
	// Parse id
	id_int, err := strconv.Atoi(id_string)
	if err != nil {
		http.Error(w, "Error, id must be an integer", http.StatusBadRequest)
		log.Error("validate_email_handler: error in strconv.Atoi()", "error", err)
		return
	}

	c, err := find_contact_id(app.mongo_client, get_username(r.Context()), id_int)
	if err != nil {
		http.Error(w, "Error, contact not found", http.StatusBadRequest)
		log.Error("validate_email_handler: error in find_contact", "error", err)
		return
	}

	// Check email is unique
	contacts, err := get_user_contacts(app.mongo_client, get_username(r.Context()))
	if err != nil {
		http.Error(w, "Error getting contacts", http.StatusInternalServerError)
		log.Error("validate_email_handler: error in get_contacts_from_user", "error", err)
		return
	}
	email := r.URL.Query().Get("email")
	c.Errors["email"] = validate_email(id_int, email, contacts)

	w.Header().Set("Content-Type", "text/html")

	templ.Handler(error_email(c.Errors["email"]), templ.WithErrorHandler(templ_error)).ServeHTTP(w, r)
}

// /contacts/archive
func (app *app) post_archive_handler(w http.ResponseWriter, r *http.Request) {

	time.Sleep(500 * time.Millisecond)

	// initialize archiver for user
	myArchiver.Start = time.Now()
	myArchiver.State = "running"

	w.Header().Set("Content-Type", "text/html")
	templ.Handler(archive_ui(myArchiver), templ.WithErrorHandler(templ_error)).ServeHTTP(w, r)
}

// /contacts/archive
func (app *app) get_archive_handler(w http.ResponseWriter, r *http.Request) {

	templ.Handler(archive_ui(myArchiver), templ.WithErrorHandler(templ_error)).ServeHTTP(w, r)
}

// DELETE /contacts/archive
func (app *app) delete_archive_handler(w http.ResponseWriter, r *http.Request) {

	myArchiver.Reset()
	templ.Handler(archive_ui(myArchiver), templ.WithErrorHandler(templ_error)).ServeHTTP(w, r)
}

// /contacts/archive/file
func (app *app) archive_file_handler(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Disposition", `attachment; filename="contacts.json"`)
	w.Header().Set("Content-Type", "application/json")

	contacts, err := get_user_contacts(app.mongo_client, get_username(r.Context()))
	if err != nil {
		http.Error(w, "Error getting contacts", http.StatusInternalServerError)
		log.Error("archive_file_handler: error in get_contacts_from_user_contacts_db", "error", err)
		return
	}

	json_file := myArchiver.Archive_file(contacts)

	_, err = w.Write(json_file.([]byte))
	if err != nil {
		http.Error(w, "Error writing response", http.StatusInternalServerError)
		log.Error("archive_file_handler: error in w.Write(json_data)", "error", err)
		return
	}
}

// AUTH HANDELRS

// POST /sign-in
func (app *app) sign_in_handler(w http.ResponseWriter, r *http.Request) {

	username := r.FormValue("username")
	password := r.FormValue("password")

	expected_password, ok := users[username]
	if !ok || expected_password != password {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		log.Error("sign_in_handler: invalid credentials for user " + username)
		return
	}

	// create session
	session_token := uuid.NewString()
	expires_at := time.Now().Add(1 * time.Minute)
	sessions[session_token] = session{
		username: username,
		expiry:   expires_at,
	}

	// tell browser
	http.SetCookie(w, &http.Cookie{
		Name:    "session_token",
		Value:   session_token,
		Expires: expires_at,
	})

	contacts_list, err := app.get_contact_page(1, username)
	if err != nil {
		http.Error(w, "Error getting contacts", http.StatusInternalServerError)
		log.Error("sign_in_handler: error in get_contact_list", "error", err)
		return
	}

	templ.Handler(con_boton_tema(index(contacts_list, "", 1, myArchiver, ""), profile_card(username)), templ.WithErrorHandler(templ_error)).ServeHTTP(w, r)
}

// POST /logout
func (app *app) logout_handler(w http.ResponseWriter, r *http.Request) {
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

	templ.Handler(con_boton_tema(register(), auth_dialog()), templ.WithErrorHandler(templ_error)).ServeHTTP(w, r)
}

// GET /register
func (app *app) get_register_handler(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "text/html")

	if r.Header.Get("HX-Request") == "true" {
		templ.Handler(con_boton_tema(register(), get_auth_or_profile(r)), templ.WithErrorHandler(templ_error)).ServeHTTP(w, r)
	} else {
		templ.Handler(layout(con_boton_tema(register(), get_auth_or_profile(r))), templ.WithErrorHandler(templ_error)).ServeHTTP(w, r)
	}
}

// POST /register
func (app *app) post_register_handler(w http.ResponseWriter, r *http.Request) {

	username := r.FormValue("username")
	password := r.FormValue("password")

	// Check if user already exists
	user, err := find_user(app.mongo_client, username)
	if err != nil {
		http.Error(w, "Error checking user existence", http.StatusInternalServerError)
		log.Error("post_register_handler: error in find_user", "error", err)
		return
	}
	if user != nil {
		http.Error(w, "User already exists", http.StatusBadRequest)
		log.Error("post_register_handler: user already exists: " + username)
		return
	}

	// insert user in DB
	err = insert_user(app.mongo_client, username, password)
	if err != nil {
		http.Error(w, "Error registering user", http.StatusInternalServerError)
		log.Error("post_register_handler: error in insert_user_db", "error", err)
		return
	}

	users[username] = password

	log.Info("User registered successfully: " + username)

	// Send cookie to browser
	session_token := uuid.NewString()
	expires_at := time.Now().Add(120 * time.Second)
	sessions[session_token] = session{username: username, expiry: expires_at}

	http.SetCookie(w, &http.Cookie{
		Name:    "session_token",
		Value:   session_token,
		Expires: expires_at,
	})

	contact_list, err := app.get_contact_page(1, username)
	if err != nil {
		http.Error(w, "Error getting contacts", http.StatusInternalServerError)
		log.Error("post_register_handler: error in get_contact_list", "error", err)
		return
	}
	templ.Handler(con_boton_tema(index(contact_list, "", 1, myArchiver, "Account created successfully"), profile_card(username)), templ.WithErrorHandler(templ_error)).ServeHTTP(w, r)
}

//------------------------------------------------------------------------------
// JSON Api
//------------------------------------------------------------------------------

type success_response struct {
	Message string `json:"message"`
}

type error_response struct {
	Message string            `json:"message"`
	Errors  map[string]string `json:"errors"`
}

// GET /api/v1/contacts
func (app *app) get_contacts_handler(w http.ResponseWriter, r *http.Request) {

	contacts, err := get_user_contacts(app.mongo_client, get_username(r.Context()))
	if err != nil {
		http.Error(w, "Error getting contacts", http.StatusInternalServerError)
		log.Error("get_contacts_handler: error in get_user_contacts", "error", err)
		return
	}

	jsonData, err := json.Marshal(contacts)
	if err != nil {
		http.Error(w, "Error converting contacts into JSON", http.StatusInternalServerError)
		log.Error("get_contacts_handler: error in json.Marshal(contacts)", "error", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(jsonData)
	if err != nil {
		http.Error(w, "Error writing response", http.StatusInternalServerError)
		log.Error("get_contacts_handler: error in w.Write(jsonData)", "error", err)
		return
	}
}

// POST /api/v1/contacts
func (app *app) post_contacts_handler(w http.ResponseWriter, r *http.Request) {

	user_contacts_db, err := get_user_contacts_db(app.mongo_client, get_username(r.Context()))
	if err != nil {
		http.Error(w, "Error getting contacts", http.StatusInternalServerError)
		log.Error("post_contacts_handler: error in get_contacts_from_user", "error", err)
		return
	}

	// Get form values
	c := Contact{
		ID:     user_contacts_db[len(user_contacts_db)-1].ID + 1,
		First:  r.FormValue("first_name"),
		Last:   r.FormValue("last_name"),
		Email:  r.FormValue("email"),
		Phone:  r.FormValue("phone"),
		Errors: make(map[string]string),
	}

	// We need to convert from Contact_db to Contact in order to use the validate_email function
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

	email_error := validate_email(-1, c.Email, contacts)
	if email_error != "" {
		// We must check this in order to keep the map length to zero when
		// no errors are found
		c.Errors["email"] = email_error
	}
	if c.First == "" {
		c.Errors["first"] = "First name is required"
	}
	if c.Last == "" {
		c.Errors["last"] = "Last name is required"
	}
	if c.Phone == "" {
		c.Errors["phone"] = "Phone is required"
	}

	w.Header().Set("Content-Type", "application/json")

	if len(c.Errors) == 0 {
		err := insert_contact(app.mongo_client, c, get_username(r.Context()))
		if err != nil {
			http.Error(w, "Error inserting contact in DB", http.StatusInternalServerError)
			log.Error("post_contacts_handler: error in insert_contact()", "error", err)
			return
		}

		// Inform about the request's success
		log.Info("Contact added successfully")
		s := success_response{
			"Contact added successfully",
		}
		json_success_response, _ := json.Marshal(s)
		_, err = w.Write(json_success_response)
		if err != nil {
			http.Error(w, "Could not show error on screen", http.StatusInternalServerError)
			log.Error("post_contacts_handler: error in w.Write(json_success_response)", "error", err)
			return
		}
		return

	} else {

		// Inform about the response's failure
		e := error_response{
			"Could not add contact due to incorrect format",
			c.Errors,
		}

		json_error_response, _ := json.Marshal(e)
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write(json_error_response)
		if err != nil {
			http.Error(w, "Could not show error on screen", http.StatusBadRequest)
			log.Error("post_contacts_handler: error in w.Write(json_error_response)", "error", err)
			return
		}

		log.Error("post_contacts_handler: wrong contact format: ", "errors", json_error_response)
	}
}

// /GET /api/v1/contacts/{id}
func (app *app) get_contact_handler(w http.ResponseWriter, r *http.Request) {

	id_string := r.PathValue("id")

	// Parse id
	id_int, err := strconv.Atoi(id_string)
	if err != nil {
		http.Error(w, "Error, id must be an integer", http.StatusBadRequest)
		log.Error("get_contact_handler: error in strconv.Atoi(id)", "error", err)
		return
	}

	// Search for specific contact
	c, err := find_contact_id(app.mongo_client, get_username(r.Context()), id_int)
	if err != nil {
		http.Error(w, "Error, contact not found", http.StatusBadRequest)
		log.Error("get_contact_handler: error in find_contact", "error", err)
		return
	}

	// Show contact information
	w.Header().Set("Content-Type", "application/json")

	jsonData, _ := json.Marshal(c)
	w.WriteHeader(http.StatusCreated)
	_, err = w.Write(jsonData)
	if err != nil {
		http.Error(w, "Error showing contact information", http.StatusInternalServerError)
		log.Error("get_contact_handler: error in w.Write(jsonData)", "error", err)
		return
	}
}

// PUT /api/v1/contacts
func (app *app) put_contact_handler(w http.ResponseWriter, r *http.Request) {

	id_string := r.PathValue("id")
	// Parse id
	id_int, err := strconv.Atoi(id_string)
	if err != nil {
		http.Error(w, "Error, id must be an integer", http.StatusBadRequest)
		log.Error("put_contact_handler: error in strconv.Atoi(id)", "error", err)
		return
	}

	// Get form values
	c := Contact{
		ID:     id_int,
		First:  r.FormValue("first_name"),
		Last:   r.FormValue("last_name"),
		Email:  r.FormValue("email"),
		Phone:  r.FormValue("phone"),
		Errors: make(map[string]string),
	}

	contacts, err := get_user_contacts(app.mongo_client, get_username(r.Context()))
	if err != nil {
		http.Error(w, "Error getting contacts", http.StatusInternalServerError)
		log.Error("put_contact_handler: error in get_contacts_from_user_contacts_db", "error", err)
		return
	}

	email_error := validate_email(-1, c.Email, contacts)
	if email_error != "" {
		c.Errors["email"] = email_error
	}
	if c.First == "" {
		c.Errors["first"] = "First name is required"
	}
	if c.Last == "" {
		c.Errors["last"] = "Last name is required"
	}
	if c.Phone == "" {
		c.Errors["phone"] = "Phone is required"
	}

	w.Header().Set("Content-Type", "application/json")

	if len(c.Errors) == 0 {
		// Search for c to edit
		contact, err := find_contact_id(app.mongo_client, get_username(r.Context()), id_int)
		if err != nil {
			http.Error(w, "Error, contact not found", http.StatusBadRequest)
			log.Error("put_contact_handler: error in find_contact", "error", err)
			return
		}

		// Replace with editted data
		contact.Email = c.Email
		contact.First = c.First
		contact.Last = c.Last
		contact.Phone = c.Phone
		contact.Errors = c.Errors

		// Inform about the request's success
		log.Info("Contact edited successfully")

		s := success_response{
			"Contact edited successfully",
		}
		json_success_response, _ := json.Marshal(s)
		w.WriteHeader(http.StatusBadRequest)
		_, err = w.Write(json_success_response)
		if err != nil {
			http.Error(w, "Could not show error on screen", http.StatusInternalServerError)
			log.Error("put_contacts_handler: error in w.Write(json_success_response)", "error", err)
			return
		}
		return

	} else {

		// Inform about the response's failure
		e := error_response{
			"Could not edit contact due to incorrect format",
			c.Errors,
		}

		json_error_response, _ := json.Marshal(e)
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write(json_error_response)
		if err != nil {
			http.Error(w, "Could not show error on screen", http.StatusBadRequest)
			log.Error("put_contact_handler: error in w.Write(json_error_response)", "error", err)
			return
		}

		log.Error("put_contact_handler: wrong contact format: ", "errors", json_error_response)
	}
}

// DELETE /api/v1/contacts/{id}
func (app *app) delete_contact_handler_json(w http.ResponseWriter, r *http.Request) {

	id_string := r.PathValue("id")
	// Parse id
	id_int, err := strconv.Atoi(id_string)
	if err != nil {
		http.Error(w, "Error, id must be an integer", http.StatusBadRequest)
		log.Error("delete_contact_handler: error in strconv.Atoi(id)", "error", err)
		return
	}

	// Delete contact
	contact, err := find_contact_id(app.mongo_client, get_username(r.Context()), id_int)
	if err != nil {
		http.Error(w, "Error finding contact", http.StatusInternalServerError)
		log.Error("delete_contact_handler: error in find_contact_id", "error", err)
		return
	}

	if contact != nil {
		err = delete_contact(app.mongo_client, get_username(r.Context()), id_int)
		if err != nil {
			http.Error(w, "Error deleting contact", http.StatusInternalServerError)
			log.Error("delete_contact_handler: error in delete_contact", "error", err)
			return
		}

		// Inform the user of the request's success
		log.Info("Contact deleted succesfully")
		s := success_response{
			"Contact deleted succesfully",
		}
		json_success_response, _ := json.Marshal(s)
		_, err := w.Write(json_success_response)
		if err != nil {
			http.Error(w, "Could not show error on screen", http.StatusBadRequest)
			log.Error("delete_contact_handler: error in w.Write(json_success_response)", "error", err)
			return
		}

		w.WriteHeader(http.StatusOK)
		return
	}

	// Inform about the response's failure
	e := error_response{
		"Could not delete contact",
		nil,
	}

	json_error_response, _ := json.Marshal(e)
	w.WriteHeader(http.StatusBadRequest)
	_, err = w.Write(json_error_response)
	if err != nil {
		http.Error(w, "Could not show error on screen", http.StatusInternalServerError)
		log.Error("delete_contact_handler: error in w.Write(jsonData)", "error", err)
		return
	}

	http.Error(w, "Error deleting contact", http.StatusInternalServerError)
	log.Error("delete_contact: contact not found")
}
