package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hypermedia/archiver"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/google/uuid"
)

var log *slog.Logger

var contacts []Contact
var contacts_data []byte
var myArchiver archiver.Archiver

func templ_error(r *http.Request, err error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: checkpr wrap
		log.Error("error", "error", err)
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusInternalServerError)
	})
}

func main() {

	// Init logger
	log = slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// Load contacts
	err := load_contacts()
	if err != nil {
		log.Error("Error in load_contacts()", "error", err)
	}

	// Set default archvier status for user
	myArchiver = *archiver.Get()

	mux := http.NewServeMux()

	// TODO: fix serving spinning circles
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// hypermedia api
	mux.HandleFunc("GET /", redirect_handler)

	mux.HandleFunc("GET /contacts", contact_query_handler)

	mux.HandleFunc("GET /contacts/{id}", contact_id_handler)

	mux.HandleFunc("GET /contacts/new", get_add_contact_handler)

	mux.HandleFunc("POST /contacts/new", post_add_contact_handler)

	mux.HandleFunc("GET /contacts/{id}/edit", get_edit_contact_handler)

	mux.HandleFunc("POST /contacts/{id}/edit", post_edit_contact_handler)

	mux.HandleFunc("DELETE /contacts/{id}", delete_contact_handler)

	mux.HandleFunc("DELETE /contacts", delete_multiple_contacts_handler)

	mux.HandleFunc("GET /contacts/count", count_contacts_handler)

	mux.HandleFunc("GET /contacts/{id}/email", validate_email_handler)

	mux.HandleFunc("POST /contacts/archive", post_archive_handler)

	mux.HandleFunc("GET /contacts/archive", get_archive_handler)

	mux.HandleFunc("DELETE /contacts/archive", delete_archive_handler)

	mux.HandleFunc("GET /contacts/archive/file", archive_file_handler)

	// json api
	mux.HandleFunc("GET /api/v1/contacts", get_contacts_handler)

	mux.HandleFunc("POST /api/v1/contacts", post_contacts_handler)

	mux.HandleFunc("GET /api/v1/contacts/{id}", get_contact_handler)

	mux.HandleFunc("PUT /api/v1/contacts/{id}", put_contact_handler)

	mux.HandleFunc("DELETE /api/v1/contacts/{id}", delete_contact_handler_json)

	// Start server
	server := http.Server{
		Addr:         ":8080",
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 90 * time.Second,
		Handler:      logging(mux),
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

func redirect_handler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/contacts", http.StatusFound)
}

type Contact struct {
	ID     int               `json:"id"`
	First  string            `json:"first"`
	Last   string            `json:"last"`
	Email  string            `json:"email"`
	Phone  string            `json:"phone"`
	Errors map[string]string `json:"errors"`
}

type PageData struct {
	Contacts []Contact
	Query    string
	Page     int
	Archiver archiver.Archiver
}

func ForceError() (string, error) {
	return "", errors.New("error forzado desde helpers")
}

var SUCCESS = "Contact added successfully"
var DELETE = "Contact deleted successfully"

// /contacts?q={id}
func contact_query_handler(w http.ResponseWriter, r *http.Request) {

	q := r.URL.Query().Get("q")

	if q == "" {

		// Show first 10 contacts
		page_string := r.URL.Query().Get("page")
		page, _ := strconv.Atoi(page_string)

		if page <= 0 {
			page = 1
		}
		contact_list := get_contact_list(page)

		if r.Header.Get("HX-Request") == "true" {
			templ.Handler(index(contact_list, "", page, myArchiver, ""), templ.WithErrorHandler(templ_error)).ServeHTTP(w, r)
		} else {
			templ.Handler(layout(con_boton_tema(index(contact_list, "", page, myArchiver, ""))), templ.WithErrorHandler(templ_error)).ServeHTTP(w, r)
		}
		return
	}

	// Search for specific contact
	contact_results, err := find_contacts(q)
	if err != nil {
		http.Error(w, "Error, contact not found", http.StatusBadRequest)
		log.Error("contact_query_handler: error in find_contact", "error", err)
		return
	}

	// Show contact information
	templ.Handler(index(contact_results, q, 0, myArchiver, ""), templ.WithErrorHandler(templ_error)).ServeHTTP(w, r)
}

// /contacts/{id}
func contact_id_handler(w http.ResponseWriter, r *http.Request) {

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
		if r.Header.Get("HX-Trigger") == "search" {
			templ.Handler(rows(get_contact_list(page)), templ.WithErrorHandler(templ_error)).ServeHTTP(w, r)
		} else {
			templ.Handler(index(get_contact_list(page), "", page, myArchiver, ""), templ.WithErrorHandler(templ_error)).ServeHTTP(w, r)
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
	c, err := find_contact_id(id_int)
	if err != nil {
		http.Error(w, "Error, contact not found", http.StatusBadRequest)
		log.Error("contact_id_handler: error in find_contact", "error", err)
		return
	}

	// Show contact information
	if r.Header.Get("HX-Request") == "true" {
		templ.Handler(layout(show(*c)), templ.WithErrorHandler(templ_error)).ServeHTTP(w, r)
	} else {
		templ.Handler(layout(con_boton_tema(show(*c))), templ.WithErrorHandler(templ_error)).ServeHTTP(w, r)
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
func get_add_contact_handler(w http.ResponseWriter, r *http.Request) {

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
func post_add_contact_handler(w http.ResponseWriter, r *http.Request) {

	// Compute new id
	max_id := 0
	for _, c := range contacts {
		if c.ID > max_id {
			max_id = c.ID
		}
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

	email_error := validate_email(-1, c.Email)
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
		// Add contact to contacts
		contacts = append(contacts, c)

		log.Info("Contact added successfully")

		// Inform user
		templ.Handler(index(get_contact_list(1), "", 1, myArchiver, SUCCESS), templ.WithErrorHandler(templ_error)).ServeHTTP(w, r)

		return
	}

	// We cannot add contact
	templ.Handler(new_contact(c), templ.WithErrorHandler(templ_error)).ServeHTTP(w, r)

}

// GET /contacts/{id}/edit
func get_edit_contact_handler(w http.ResponseWriter, r *http.Request) {

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
	c, err := find_contact_id(id_int)
	if err != nil {
		http.Error(w, "Error, contact not found", http.StatusBadRequest)
		log.Error("edit_contact_get_handler: error in find_contact", "error", err)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		templ.Handler(edit_contact(*c), templ.WithErrorHandler(templ_error)).ServeHTTP(w, r)
	} else {
		templ.Handler(layout(con_boton_tema(edit_contact(*c))), templ.WithErrorHandler(templ_error)).ServeHTTP(w, r)
	}
}

// POST /contacts/{id}/edit
func post_edit_contact_handler(w http.ResponseWriter, r *http.Request) {

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

	email_error := validate_email(id_int, c.Email)
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
		// Search for c to edit
		contact, err := find_contact_id(id_int)
		if err != nil {
			http.Error(w, "Error, contact not found", http.StatusBadRequest)
			log.Error("post_edit_contact_handler: error in find_contact", "error", err)
			return
		}

		// Replace with editted data
		contact.Email = c.Email
		contact.First = c.First
		contact.Last = c.Last
		contact.Phone = c.Phone
		contact.Errors = c.Errors

		log.Info("Contact edited successfully")

		templ.Handler(show(*contact), templ.WithErrorHandler(templ_error)).ServeHTTP(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	templ.Handler(edit_contact(c), templ.WithErrorHandler(templ_error)).ServeHTTP(w, r)
}

// DELETE /contacts/{id}/edit
func delete_contact_handler(w http.ResponseWriter, r *http.Request) {

	id_string := r.PathValue("id")
	// Parse id
	id_int, err := strconv.Atoi(id_string)
	if err != nil {
		http.Error(w, "Error, id must be an integer", http.StatusBadRequest)
		log.Error("delete_contact_handler: error in strconv.Atoi(id)", "error", err)
		return
	}

	// Delete contact

	for i, c := range contacts {
		if c.ID == id_int {
			// Remove the contact at index i
			contacts = append(contacts[:i], contacts[i+1:]...)

			log.Info("Contact deleted successfully")

			// Tell htmx clients that contacts changed so widgets like the count can refresh.
			w.Header().Set("HX-Trigger", "contacts-changed")

			if r.Header.Get("HX-Trigger") == "delete-btn" {
				templ.Handler(index(get_contact_list(1), "", 1, myArchiver, DELETE), templ.WithErrorHandler(templ_error)).ServeHTTP(w, r)
				http.Redirect(w, r, "/contacts", http.StatusSeeOther)
			}

			// We do not want to render anything
			return
		}
	}

	// We cannot delete contact, which should always be possible form this endpoint
	http.Error(w, "Error deleting contact", http.StatusInternalServerError)
	log.Error("delete_contact_handler: contact not found")
}

// /contacts/count
func count_contacts_handler(w http.ResponseWriter, r *http.Request) {

	time.Sleep(1 * time.Second)
	count := len(contacts)
	_, err := w.Write([]byte(strconv.Itoa(count) + " total Contacts"))
	if err != nil {
		http.Error(w, "Error, could not write response", http.StatusInternalServerError)
		log.Error("count_contacts_handler: error in  w.Write()", "error", err)
		return
	}
}

// DELETE /contacts
func delete_multiple_contacts_handler(w http.ResponseWriter, r *http.Request) {

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
		for i, c := range contacts {
			if c.ID == id_int {
				// Remove the contact at index i
				contacts = append(contacts[:i], contacts[i+1:]...)
			}
		}
	}

	w.Header().Set("Content-Type", "text/html")
	templ.Handler(index(get_contact_list(1), "", 1, myArchiver, ""), templ.WithErrorHandler(templ_error)).ServeHTTP(w, r)
}

// /contacts/{id}/{email}
func validate_email_handler(w http.ResponseWriter, r *http.Request) {

	id_string := r.PathValue("id")
	// Parse id
	id_int, err := strconv.Atoi(id_string)
	if err != nil {
		http.Error(w, "Error, id must be an integer", http.StatusBadRequest)
		log.Error("validate_email_handler: error in strconv.Atoi()", "error", err)
		return
	}

	c, err := find_contact_id(id_int)
	if err != nil {
		http.Error(w, "Error, contact not found", http.StatusBadRequest)
		log.Error("validate_email_handler: error in find_contact", "error", err)
		return
	}
	// Check email is unique
	email := r.URL.Query().Get("email")
	c.Errors["email"] = validate_email(id_int, email)

	w.Header().Set("Content-Type", "text/html")

	templ.Handler(error_email(c.Errors["email"]), templ.WithErrorHandler(templ_error)).ServeHTTP(w, r)
}

// /contacts/archive
func post_archive_handler(w http.ResponseWriter, r *http.Request) {

	time.Sleep(500 * time.Millisecond)

	// initialize archiver for user
	myArchiver.Start = time.Now()
	myArchiver.State = "running"

	w.Header().Set("Content-Type", "text/html")
	templ.Handler(archive_ui(myArchiver), templ.WithErrorHandler(templ_error)).ServeHTTP(w, r)
}

// /contacts/archive
func get_archive_handler(w http.ResponseWriter, r *http.Request) {

	templ.Handler(archive_ui(myArchiver), templ.WithErrorHandler(templ_error)).ServeHTTP(w, r)
}

// DELETE /contacts/archive
func delete_archive_handler(w http.ResponseWriter, r *http.Request) {

	myArchiver.Reset()
	templ.Handler(archive_ui(myArchiver), templ.WithErrorHandler(templ_error)).ServeHTTP(w, r)
}

// /contacts/archive/file
func archive_file_handler(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Disposition", `attachment; filename="contacts.json"`)
	w.Header().Set("Content-Type", "application/json")

	json_file := myArchiver.Archive_file(contacts)

	_, err := w.Write(json_file.([]byte))
	if err != nil {
		http.Error(w, "Error writing response", http.StatusInternalServerError)
		log.Error("archive_file_handler: error in w.Write(json_data)", "error", err)
		return
	}
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
func get_contacts_handler(w http.ResponseWriter, r *http.Request) {

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
func post_contacts_handler(w http.ResponseWriter, r *http.Request) {

	// Get form values
	c := Contact{
		ID:     contacts[len(contacts)-1].ID + 1,
		First:  r.FormValue("first_name"),
		Last:   r.FormValue("last_name"),
		Email:  r.FormValue("email"),
		Phone:  r.FormValue("phone"),
		Errors: make(map[string]string),
	}

	email_error := validate_email(-1, c.Email)
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
		contacts = append(contacts, c)

		// Inform about the request's success
		log.Info("Contact added successfully")
		s := success_response{
			"Contact added successfully",
		}
		json_success_response, _ := json.Marshal(s)
		_, err := w.Write(json_success_response)
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
func get_contact_handler(w http.ResponseWriter, r *http.Request) {

	id_string := r.PathValue("id")

	// Parse id
	id_int, err := strconv.Atoi(id_string)
	if err != nil {
		http.Error(w, "Error, id must be an integer", http.StatusBadRequest)
		log.Error("get_contact_handler: error in strconv.Atoi(id)", "error", err)
		return
	}

	// Search for specific contact
	c, err := find_contact_id(id_int)
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
func put_contact_handler(w http.ResponseWriter, r *http.Request) {

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

	email_error := validate_email(-1, c.Email)
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
		contact, err := find_contact_id(id_int)
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
func delete_contact_handler_json(w http.ResponseWriter, r *http.Request) {

	id_string := r.PathValue("id")
	// Parse id
	id_int, err := strconv.Atoi(id_string)
	if err != nil {
		http.Error(w, "Error, id must be an integer", http.StatusBadRequest)
		log.Error("delete_contact_handler: error in strconv.Atoi(id)", "error", err)
		return
	}

	// Delete contact
	for i, c := range contacts {
		if c.ID == id_int {
			// Remove the contact at index i
			contacts = append(contacts[:i], contacts[i+1:]...)

			// Inform the user of the request's success
			log.Info("Contact deleted succesfully")
			s := success_response{
				"Contact deleted succesfully",
			}
			json_success_response, _ := json.Marshal(s)
			w.WriteHeader(http.StatusBadRequest)
			_, err := w.Write(json_success_response)
			if err != nil {
				http.Error(w, "Could not show error on screen", http.StatusBadRequest)
				log.Error("delete_contact_handler: error in w.Write(json_success_response)", "error", err)
				return
			}
			return
		}
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

// -----------------------------------------------------------------------------
// AUXILIARY FUNCTIONS

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

func load_contacts() error {

	var err error
	contacts_data, err = os.ReadFile("contacts.json")
	if err != nil {
		return fmt.Errorf("load_contacts: error in osReadFile: %w", err)
	}

	err = json.Unmarshal(contacts_data, &contacts)
	if err != nil {
		return fmt.Errorf("load_contacts: error in json.Unmarhsall: %w", err)
	}

	return nil
}

func find_contact_id(id int) (*Contact, error) {
	for i := range contacts {
		if contacts[i].ID == id {
			return &contacts[i], nil
		}
	}
	return nil, fmt.Errorf("find_contact_id: contact not found")
}

func find_contacts(q string) ([]Contact, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil, fmt.Errorf("find_contacts: empty query")
	}

	var results []Contact

	// If q is an integer, try ID match first
	if id, err := strconv.Atoi(q); err == nil {
		for i := range contacts {
			if contacts[i].ID == id {
				results = append(results, contacts[i])
			}
		}
		return results, nil
	}

	// perform string search
	qLower := strings.ToLower(q)
	for i := range contacts {
		c := contacts[i]
		if strings.Contains(strings.ToLower(c.First), qLower) ||
			strings.Contains(strings.ToLower(c.Last), qLower) ||
			strings.Contains(strings.ToLower(c.Email), qLower) ||
			strings.Contains(strings.ToLower(c.Phone), qLower) {

			results = append(results, c)
		}
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("find_contacts: contact not found")
	}
	return results, nil
}

func get_contact_list(page int) []Contact {

	p := page - 1
	limit := p*10 + 10
	var contact_set []Contact
	for i := p * 10; i < limit && i < len(contacts); i++ {
		contact_set = append(contact_set, contacts[i])
	}

	return contact_set
}

func validate_email(id int, email string) string {

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
