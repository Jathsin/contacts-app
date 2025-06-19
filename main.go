package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"
)

var log *slog.Logger

var contacts []Contact
var contacts_data []byte

type app struct {
	Templates *Templates
}

// Template utils
type Templates struct {
	templates *template.Template
}

func (t *Templates) Render(w io.Writer, name string, data interface{}) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

// Extend template parsing understanding
func newTemplate() *Templates {
	tmpl := template.New("layout").Funcs(template.FuncMap{
		"add": func(a, b int) int {
			return a + b
		},
	})
	return &Templates{
		templates: template.Must(tmpl.ParseGlob("templates/*.html")),
	}
}

func main() {

	// Init logger
	log = slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// Load contacts
	err := load_contacts()
	if err != nil {
		log.Error("Error in load_contacts()", "error", err)
	}

	app := app{newTemplate()}

	mux := http.NewServeMux()

	// @TODO: fix serving spinning circles
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Define endpoints
	mux.HandleFunc("GET /", redirect_handler)

	mux.HandleFunc("GET /contacts", app.contact_query_handler)

	mux.HandleFunc("GET /contacts/{id}", app.contact_id_handler)

	mux.HandleFunc("GET /contacts/new", app.add_contact_get_handler)

	mux.HandleFunc("POST /contacts/new", app.add_contact_post_handler)

	mux.HandleFunc("GET /contacts/{id}/edit", app.edit_contact_get_handler)

	mux.HandleFunc("POST /contacts/{id}/edit", app.edit_contact_post_handler)

	mux.HandleFunc("DELETE /contacts/{id}", app.delete_contact_handler)

	mux.HandleFunc("DELETE /contacts", app.delete_multiple_contacts_handler)

	mux.HandleFunc("GET /contacts/count", app.count_contacts_handler)

	mux.HandleFunc("GET /contacts/{id}/email", app.validate_email_handler)

	mux.HandleFunc("POST /contacts/archive", app.archive_contact_handler)

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

// @TODO: should it be an app method?
func redirect_handler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/contacts", http.StatusFound)
}

type Contact struct {
	ID    int    `json:"id"`
	First string `json:"first"`
	Last  string `json:"last"`
	Email string `json:"email"`
	Phone string `json:"phone"`
}

type PageData struct {
	Contacts []Contact
	Query    string
	Page     int
}

type Form_data struct {
	Contact Contact
	Errors  map[string]string
}

// /Contacts?q={id} and /contacts/{id}
func (app *app) contact_query_handler(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "text/html")

	id_string := r.URL.Query().Get("q")

	if id_string == "" {

		// Show first 10 contacts
		page_string := r.URL.Query().Get("page")
		page, _ := strconv.Atoi(page_string)

		if page <= 0 {
			page = 1
		}

		err := app.Templates.Render(w, "index", PageData{get_contact_list(page), "", page})
		if err != nil {
			http.Error(w, "Error providing contact information", http.StatusInternalServerError)
			log.Error("contact_query_handler: error in app.Templates.Render()", "error", err)
			return
		}

		return
	}

	// Parse id
	id_int, err := strconv.Atoi(id_string)
	if err != nil {
		http.Error(w, "Error, id must be an integer", http.StatusBadRequest)
		log.Error("contact_query_handler: error in strconv.Atoi(id)", "error", err)
		return
	}

	// Search for specific contact
	contact, err := find_contact(id_int)
	if err != nil {
		http.Error(w, "Error, contact not found", http.StatusBadRequest)
		log.Error("contact_query_handler: error in find_contact", "error", err)
		return
	}

	data := PageData{[]Contact{*contact}, id_string, 0}

	// Show contact information
	err = app.Templates.Render(w, "index", data)
	if err != nil {
		http.Error(w, "Error finding contact", http.StatusBadRequest)
		log.Error("contact_query_handler: error in app.Templates.Render()", "error", err)
		return
	}
}

// /contacts/{id}
func (app *app) contact_id_handler(w http.ResponseWriter, r *http.Request) {

	id_string := r.PathValue("id")

	if id_string == "" {

		// Show contact information response
		w.Header().Set("Content-Type", "text/html")

		page_string := r.URL.Query().Get("page")
		page, _ := strconv.Atoi(page_string)
		if page <= 0 {
			page = 1
		}

		var err error
		if r.Header.Get("HX-Trigger") == "search" {
			err = app.Templates.Render(w, "rows", PageData{get_contact_list(page), "", page})

		} else {
			err = app.Templates.Render(w, "index", PageData{get_contact_list(page), "", page})
		}
		if err != nil {
			http.Error(w, "Error providing contact information", http.StatusInternalServerError)
			log.Error("contact_id_handler: error in app.Templates.Render()", "error", err)
			return
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
	contact, err := find_contact(id_int)
	if err != nil {
		http.Error(w, "Error, contact not found", http.StatusBadRequest)
		log.Error("contact_id_handler: error in find_contact", "error", err)
		return
	}

	// Show contact information
	err = app.Templates.Render(w, "show", contact)
	if err != nil {
		http.Error(w, "Error showing contact", http.StatusBadRequest)
		log.Error("contact_id_handler: error in app.Templates.Render()", "error", err)
		return
	}
}

func (app *app) add_contact_get_handler(w http.ResponseWriter, r *http.Request) {

	var form_data Form_data
	err := app.Templates.Render(w, "new", form_data)
	if err != nil {
		http.Error(w, "Error, could not render page", http.StatusInternalServerError)
		log.Error("add_contact_get_handler: error in app.Templates.Render()", "error", err)
		return
	}
}

// /contacts/new
func (app *app) add_contact_post_handler(w http.ResponseWriter, r *http.Request) {

	success := true

	// Initialize the error map
	errors := make(map[string]string)

	// Get form values
	email := r.FormValue("email")
	first := r.FormValue("first_name")
	last := r.FormValue("last_name")
	phone := r.FormValue("phone")

	//@TODO: verify fields are valid
	if email == "" {
		errors["email"] = "Email is required"
		success = false
	}
	if first == "" {
		errors["first"] = "First name is required"
		success = false
	}
	if last == "" {
		errors["last"] = "Last name is required"
		success = false
	}
	if phone == "" {
		errors["phone"] = "Phone is required"
		success = false
	}

	contact := Contact{
		// Depends on initial json file
		contacts[len(contacts)-1].ID + 1,
		first,
		last,
		email,
		phone,
	}

	if success {
		// Add contact to contacts
		contacts = append(contacts, contact)

		// @TODO: show message to the user
		log.Info("Contact successfully added")
		w.Header().Set("HX-Redirect", "/contacts/"+strconv.Itoa(contact.ID))
		w.WriteHeader(http.StatusOK)
		return
	}

	// We cannot add contact
	form_data := Form_data{
		Contact: contact,
		Errors:  errors,
	}

	err := app.Templates.Render(w, "new", form_data)
	if err != nil {
		http.Error(w, "Error, could not render page", http.StatusInternalServerError)
		log.Error("add_contact_post_handler: error in app.Templates.Render()", "error", err)
		return
	}

}

// /contacts/{id}/edit
func (app *app) edit_contact_get_handler(w http.ResponseWriter, r *http.Request) {

	id_string := r.PathValue("id")
	// Parse id
	id_int, err := strconv.Atoi(id_string)
	if err != nil {
		http.Error(w, "Error, id must be an integer", http.StatusBadRequest)
		log.Error("edit_contact_post_handler: error in strconv.Atoi(id)", "error", err)
		return
	}
	// Search for contact to edit
	contact, err := find_contact(id_int)
	if err != nil {
		http.Error(w, "Error, contact not found", http.StatusBadRequest)
		log.Error("edit_contact_get_handler: error in find_contact", "error", err)
		return
	}

	var form_data Form_data
	form_data.Contact = *contact
	err = app.Templates.Render(w, "edit", form_data)
	if err != nil {
		http.Error(w, "Error, could not render page", http.StatusInternalServerError)
		log.Error("edit_contact_get_handler: error in app.Templates.Render()", "error", err)
		return
	}
}

// /contacts/{id}/edit
func (app *app) edit_contact_post_handler(w http.ResponseWriter, r *http.Request) {

	id_string := r.PathValue("id")
	// Parse id
	id_int, err := strconv.Atoi(id_string)
	if err != nil {
		http.Error(w, "Error, id must be an integer", http.StatusBadRequest)
		log.Error("edit_contact_post_handler: error in strconv.Atoi(id)", "error", err)
		return
	}

	// Initialize the error map
	errors := make(map[string]string)

	// Get form values
	email := r.FormValue("email")
	first := r.FormValue("first_name")
	last := r.FormValue("last_name")
	phone := r.FormValue("phone")

	success := true

	//@TODO: verify fields are valid
	errors["email"] = validate_email(id_int, email)
	if first == "" {
		errors["first"] = "First name is required"
		success = false
	}
	if last == "" {
		errors["last"] = "Last name is required"
		success = false
	}
	if phone == "" {
		errors["phone"] = "Phone is required"
		success = false
	}

	if errors["email"] == "" && success {
		// Search for contact to edit
		contact, err := find_contact(id_int)
		if err != nil {
			http.Error(w, "Error, contact not found", http.StatusBadRequest)
			log.Error("edit_contact_post_handler: error in find_contact", "error", err)
			return
		}

		// Replace with editted data
		contact.Email = email
		contact.First = first
		contact.Last = last
		contact.Phone = phone

		// @TODO: show message to the user
		log.Info("Contact edited successfully")
		w.Header().Set("HX-Redirect", "/contacts/"+strconv.Itoa(id_int))
		w.WriteHeader(http.StatusFound)
		return
	}

	// We cannot edit contact
	form_data := Form_data{
		Contact: Contact{
			ID:    id_int, //default
			Email: email,
			First: first,
			Last:  last,
			Phone: phone,
		},
		Errors: errors,
	}

	err = app.Templates.Render(w, "edit", form_data)
	if err != nil {
		http.Error(w, "Error, could not render page", http.StatusInternalServerError)
		log.Error("edit_contact_post_handler: error in app.Templates.Render()", "error", err)
		return
	}
}

// /contacts/{id}/edit
func (app *app) delete_contact_handler(w http.ResponseWriter, r *http.Request) {

	id_string := r.PathValue("id")
	// Parse id
	id_int, err := strconv.Atoi(id_string)
	if err != nil {
		http.Error(w, "Error, id must be an integer", http.StatusBadRequest)
		log.Error("delete_contact_handler: error in strconv.Atoi(id)", "error", err)
		return
	}

	// Initialize the error map
	errors := make(map[string]string)

	// Get form values
	email := r.FormValue("email")
	first := r.FormValue("first_name")
	last := r.FormValue("last_name")
	phone := r.FormValue("phone")

	//@TODO: verify fields are valid
	if email == "" {
		errors["email"] = "Email is required"
	}
	if first == "" {
		errors["first"] = "First name is required"
	}
	if last == "" {
		errors["last"] = "Last name is required"
	}
	if phone == "" {
		errors["phone"] = "Phone is required"
	}

	// Delete contact
	for i, c := range contacts {
		if c.ID == id_int {
			// Remove the contact at index i
			contacts = append(contacts[:i], contacts[i+1:]...)

			//@TODO: show message to the user
			log.Info("Contact deleted succesfully")
			if r.Header.Get("HX-Trigger") == "delete-btn" {
				http.Redirect(w, r, "/contacts", http.StatusSeeOther)
			}
			// We do not want to render anything
			return
		}
	}

	// We cannot delete contact
	log.Info("delete_contact: contact not found")
	form_data := Form_data{
		Contact: Contact{
			ID:    id_int, //default
			Email: email,
			First: first,
			Last:  last,
			Phone: phone,
		},
		Errors: errors,
	}

	err = app.Templates.Render(w, "edit", form_data)
	if err != nil {
		http.Error(w, "Error, could not render page", http.StatusInternalServerError)
		log.Error("delete_contact_handler: error in app.Templates.Render()", "error", err)
		return
	}
}

func (app *app) count_contacts_handler(w http.ResponseWriter, r *http.Request) {

	count := len(contacts)
	_, err := w.Write([]byte(strconv.Itoa(count) + " total Contacts"))
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

	fmt.Print("hellou")
	fmt.Print(ids)

	var ids_int []int

	// Parse array
	for i, id_string := range ids {
		id_int, err := strconv.Atoi(id_string)
		if err != nil {
			http.Error(w, "Error, id nº "+strconv.Itoa(i)+"must be an integer", http.StatusBadRequest)
			log.Error("delete_contact_handler: error in strconv.Atoi(id)", "error", err)
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

	err = app.Templates.Render(w, "index", PageData{get_contact_list(1), "", 1})
	if err != nil {
		http.Error(w, "Error, could not render page", http.StatusInternalServerError)
		log.Error("delete_multiple_contacts_handler: error in app.Templates.Render()", "error", err)
		return
	}
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

	contact, err := find_contact(id_int)
	if err != nil {
		http.Error(w, "Error, contact not found", http.StatusBadRequest)
		log.Error("validate_email_handler: error in find_contact", "error", err)
		return
	}
	// Check email is unique
	email := r.URL.Query().Get("email")
	error_msg := validate_email(id_int, email)

	Errors := make(map[string]string)
	Errors["email"] = error_msg

	form_data := Form_data{
		*contact,
		Errors,
	}

	err = app.Templates.Render(w, "error_email", form_data)
	if err != nil {
		http.Error(w, "Error, could not render page", http.StatusInternalServerError)
		log.Error("validate_email_handler: error in app.Templates.Render()", "error", err)
		return
	}
}

func (app *app) archive_contact_handler() {
	archiver := GetArchiverUser()
}

// AUXILIAR FUNCTIONS
func logging(f http.Handler) http.Handler {
	return (http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := uuid.New().String()
		log := log.With("request_id", id)

		log.Info("NewRequest",
			"method", r.Method, "url", r.URL.Path,
			"remoteAddress", r.RemoteAddr)

		ctx := context.WithValue(r.Context(), "log", log)
		r = r.WithContext(ctx)
		//calls actual handler
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

func find_contact(id int) (*Contact, error) {
	for i := range contacts {
		if contacts[i].ID == id {
			return &contacts[i], nil
		}
	}
	return nil, fmt.Errorf("find_contact: error, contact not found")
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

// Validation logic
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
