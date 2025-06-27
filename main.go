package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"hypermedia/archiver"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
)

var log *slog.Logger

var contacts []Contact
var contacts_data []byte
var myArchiver archiver.Archiver

type App struct {
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
		"mult": func(a float64, b float64) float64 {
			return a * b
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

	// Set default archvier status for user
	user_id := uuid.NewString()
	myArchiver = *archiver.GetArchiverForUser(user_id)

	app := App{newTemplate()}

	mux := http.NewServeMux()

	// TODO: fix serving spinning circles
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// hypermedia api
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

	mux.HandleFunc("POST /contacts/archive", app.archive_post_handler)

	mux.HandleFunc("GET /contacts/archive", app.archive_get_handler)

	mux.HandleFunc("DELETE /contacts/archive", app.archive_delete_handler)

	mux.HandleFunc("GET /contacts/archive/file", app.archive_file_handler)

	// json api
	mux.HandleFunc("GET /api/v1/contacts", get_contacts_handler)

	mux.HandleFunc("POST /api/v1/contacts", post_contacts_handler)

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

type form_data struct {
	Contact Contact
	Errors  map[string]string
}

// /contacts?q={id} and /contacts/{id}
func (app *App) contact_query_handler(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "text/html")

	id_string := r.URL.Query().Get("q")

	if id_string == "" {

		// Show first 10 contacts
		page_string := r.URL.Query().Get("page")
		page, _ := strconv.Atoi(page_string)

		if page <= 0 {
			page = 1
		}

		//	TODO: fix writeHeader superfluous call
		err := app.Templates.Render(w, "index", PageData{get_contact_list(page), "", page, myArchiver})
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
	c, err := find_contact(id_int)
	if err != nil {
		http.Error(w, "Error, contact not found", http.StatusBadRequest)
		log.Error("contact_query_handler: error in find_contact", "error", err)
		return
	}

	data := PageData{[]Contact{*c}, id_string, 0, myArchiver}

	// Show contact information
	err = app.Templates.Render(w, "index", data)
	if err != nil {
		http.Error(w, "Error finding contact", http.StatusBadRequest)
		log.Error("contact_query_handler: error in app.Templates.Render()", "error", err)
		return
	}
}

// /contacts/{id}
func (app *App) contact_id_handler(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "text/html")

	id_string := r.PathValue("id")

	if id_string == "" {

		page_string := r.URL.Query().Get("page")
		page, _ := strconv.Atoi(page_string)
		if page <= 0 {
			page = 1
		}

		var err error
		if r.Header.Get("HX-Trigger") == "search" {
			err = app.Templates.Render(w, "rows", PageData{get_contact_list(page), "", page, myArchiver})

		} else {
			err = app.Templates.Render(w, "index", PageData{get_contact_list(page), "", page, myArchiver})
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
	c, err := find_contact(id_int)
	if err != nil {
		http.Error(w, "Error, contact not found", http.StatusBadRequest)
		log.Error("contact_id_handler: error in find_contact", "error", err)
		return
	}

	// Show contact information
	err = app.Templates.Render(w, "show", c)
	if err != nil {
		http.Error(w, "Error showing contact", http.StatusBadRequest)
		log.Error("contact_id_handler: error in app.Templates.Render()", "error", err)
		return
	}
}

func (app *App) add_contact_get_handler(w http.ResponseWriter, r *http.Request) {

	var form_data form_data
	err := app.Templates.Render(w, "new", form_data)
	if err != nil {
		http.Error(w, "Error, could not render page", http.StatusInternalServerError)
		log.Error("add_contact_get_handler: error in app.Templates.Render()", "error", err)
		return
	}
}

// /contacts/new
func (app *App) add_contact_post_handler(w http.ResponseWriter, r *http.Request) {

	success := true

	// Initialize the error map
	errors := make(map[string]string)

	// Get form values
	email := r.FormValue("email")
	first := r.FormValue("first_name")
	last := r.FormValue("last_name")
	phone := r.FormValue("phone")

	// TODO: verify fields are valid
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

	c := Contact{
		// Depends on initial json file
		contacts[len(contacts)-1].ID + 1,
		first,
		last,
		email,
		phone,
		nil,
	}

	if success {
		// Add contact to contacts
		contacts = append(contacts, c)

		// TODO: show message to the user
		// 		document.body.addEventListener('htmx:beforeSwap', evt => { <1>
		//   if (evt.detail.xhr.status === 404) { <2>
		//     showNotFoundError();
		//   }
		// });

		log.Info("Contact successfully added")
		w.Header().Set("HX-Redirect", "/contacts/"+strconv.Itoa(c.ID))
		w.WriteHeader(http.StatusOK)
		return
	}

	// We cannot add contact
	form_data := form_data{
		Contact: c,
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
func (app *App) edit_contact_get_handler(w http.ResponseWriter, r *http.Request) {

	// Parse id
	id_string := r.PathValue("id")
	id_int, err := strconv.Atoi(id_string)
	if err != nil {
		http.Error(w, "Error, id must be an integer", http.StatusBadRequest)
		log.Error("edit_contact_post_handler: error in strconv.Atoi(id)", "error", err)
		return
	}
	// Search for contact to edit
	c, err := find_contact(id_int)
	if err != nil {
		http.Error(w, "Error, contact not found", http.StatusBadRequest)
		log.Error("edit_contact_get_handler: error in find_contact", "error", err)
		return
	}

	var form_data form_data
	form_data.Contact = *c
	err = app.Templates.Render(w, "edit", form_data)
	if err != nil {
		http.Error(w, "Error, could not render page", http.StatusInternalServerError)
		log.Error("edit_contact_get_handler: error in app.Templates.Render()", "error", err)
		return
	}
}

// /contacts/{id}/edit
func (app *App) edit_contact_post_handler(w http.ResponseWriter, r *http.Request) {

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

	// TODO: verify fields are valid
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
		// Search for c to edit
		c, err := find_contact(id_int)
		if err != nil {
			http.Error(w, "Error, contact not found", http.StatusBadRequest)
			log.Error("edit_contact_post_handler: error in find_contact", "error", err)
			return
		}

		// Replace with editted data
		c.Email = email
		c.First = first
		c.Last = last
		c.Phone = phone

		// TODO: show message to the user
		log.Info("Contact edited successfully")
		w.Header().Set("HX-Redirect", "/contacts/"+strconv.Itoa(id_int))
		w.WriteHeader(http.StatusFound)
		return
	}

	// We cannot edit contact
	form_data := form_data{
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
func (app *App) delete_contact_handler(w http.ResponseWriter, r *http.Request) {

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

	// TODO: verify fields are valid
	errors["email"] = validate_email(id_int, email)
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

			// TODO: show message to the user
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
	form_data := form_data{
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

func (app *App) count_contacts_handler(w http.ResponseWriter, r *http.Request) {

	count := len(contacts)
	_, err := w.Write([]byte(strconv.Itoa(count) + " total Contacts"))
	if err != nil {
		http.Error(w, "Error, could not write response", http.StatusInternalServerError)
		log.Error("count_contacts_handler: error in  w.Write()", "error", err)
		return
	}
}

// DELETE /contacts
func (app *App) delete_multiple_contacts_handler(w http.ResponseWriter, r *http.Request) {

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

	err = app.Templates.Render(w, "index", PageData{get_contact_list(1), "", 1, myArchiver})
	if err != nil {
		http.Error(w, "Error, could not render page", http.StatusInternalServerError)
		log.Error("delete_multiple_contacts_handler: error in app.Templates.Render()", "error", err)
		return
	}
}

// /contacts/{id}/{email}
func (app *App) validate_email_handler(w http.ResponseWriter, r *http.Request) {

	id_string := r.PathValue("id")
	// Parse id
	id_int, err := strconv.Atoi(id_string)
	if err != nil {
		http.Error(w, "Error, id must be an integer", http.StatusBadRequest)
		log.Error("validate_email_handler: error in strconv.Atoi()", "error", err)
		return
	}

	c, err := find_contact(id_int)
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

	form_data := form_data{
		*c,
		Errors,
	}

	err = app.Templates.Render(w, "error_email", form_data)
	if err != nil {
		http.Error(w, "Error, could not render page", http.StatusInternalServerError)
		log.Error("validate_email_handler: error in app.Templates.Render()", "error", err)
		return
	}
}

// /contacts/archive
func (app *App) archive_post_handler(w http.ResponseWriter, r *http.Request) {

	// Concurrent processing
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		myArchiver.Run()
	}()

	time.Sleep(500 * time.Millisecond)

	err := app.Templates.Render(w, "archive_ui", myArchiver)
	if err != nil {
		http.Error(w, "Error processing archive", http.StatusInternalServerError)
		log.Error("archive_post_handler: error in app.Templates.Render()", "error", err)
		return
	}
}

// /contacts/archive
func (app *App) archive_get_handler(w http.ResponseWriter, r *http.Request) {

	err := app.Templates.Render(w, "archive_ui", myArchiver)
	if err != nil {
		http.Error(w, "Error processing archive", http.StatusInternalServerError)
		log.Error("archive_get_handler: error in app.Templates.Render()", "error", err)
		return
	}
}

// DELETE /contacts/archive
func (app *App) archive_delete_handler(w http.ResponseWriter, r *http.Request) {

	myArchiver.Reset()
	err := app.Templates.Render(w, "archive_ui", myArchiver)
	if err != nil {
		http.Error(w, "Error processing archive", http.StatusInternalServerError)
		log.Error("archive_delete_handler: error in app.Templates.Render()", "error", err)
		return
	}

}

// /contacts/archive/file
func (app *App) archive_file_handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Disposition", `attachment; filename="contacts.json"`)

	// Serve the file
	http.ServeFile(w, r, "contacts.json")
}

// GET /api/v1/contacts
func get_contacts_handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	http.ServeFile(w, r, "contacts.json")
}

// POST /api/v1/contacts
func post_contacts_handler(w http.ResponseWriter, r *http.Request) {
	// Initialize the error map
	errors := make(map[string]string)

	// Get form values
	c := Contact{
		ID:     contacts[len(contacts)-1].ID + 1,
		First:  r.FormValue("first_name"),
		Last:   r.FormValue("last_name"),
		Email:  r.FormValue("email"),
		Phone:  r.FormValue("phone"),
		Errors: nil,
	}

	// TODO: verify fields are valid
	errors["email"] = validate_email(-1, c.Email)
	if c.First == "" {
		errors["first"] = "First name is required"
	}
	if c.Last == "" {
		errors["last"] = "Last name is required"
	}
	if c.Phone == "" {
		errors["phone"] = "Phone is required"
	}

	if len(errors) == 0 {
		contacts = append(contacts, c)
		// Return success response
		jsonData, _ := json.Marshal(c)
		w.WriteHeader(http.StatusCreated)
		_, err := w.Write(jsonData)
		if err != nil {
			http.Error(w, "Could not show created contact on screen", http.StatusInternalServerError)
			log.Error("post_contacts_handler: error in w.Write(jsonData), successfull response", "error", err)
		}
	} else {
		// Convert errors map to JSON and return
		errorResponse := map[string]interface{}{
			"message": "Could not add contact due to incorrect format",
			"errors":  errors,
		}
		jsonData, _ := json.Marshal(errorResponse)
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write(jsonData)
		if err != nil {
			http.Error(w, "Could not show error on screen", http.StatusInternalServerError)
			log.Error("post_contacts_handler: error in w.Write(jsonData)", "error", err)
		}

		log.Error("post_contacts_handler: wrong contact format: ", "errors", jsonData)
	}

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
