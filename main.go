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

func newTemplate() *Templates {
	return &Templates{
		templates: template.Must(template.ParseGlob("templates/*.html")),
	}
}

func main() {

	//init logger
	log = slog.New(slog.NewJSONHandler(os.Stdout, nil))

	//load contacts
	err := load_contacts()
	if err != nil {
		log.Error("Error in load_contacts()", "error", err)
	}

	app := app{newTemplate()}

	mux := http.NewServeMux()

	//Define endpoints
	mux.HandleFunc("GET /", redirect_handler)

	mux.HandleFunc("GET /contacts", app.contact_handler)

	mux.HandleFunc("GET /contacts/new", app.add_contact_get_handler)

	mux.HandleFunc("POST /contacts/new", app.add_contact_post_handler)

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
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone"`
}

type PageData struct {
	Contacts []Contact
	Query    string
}

// @TODO: divide into two handlers?
// /contacts/{id}
func (app *app) contact_handler(w http.ResponseWriter, r *http.Request) {

	id_query := r.URL.Query().Get("q")

	if id_query == "" {

		//Show contact information response
		w.Header().Set("Content-Type", "text/html")

		err := app.Templates.Render(w, "index", PageData{contacts, ""})
		if err != nil {
			http.Error(w, "Error providing contact information", http.StatusInternalServerError)
			log.Error("contact_handler: error in app.Templates.Render()", "error", err)
			return
		}

		return
	}

	//Parse id
	id_int, err := strconv.Atoi(id_query)
	if err != nil {
		http.Error(w, "Error, id must be an integer", http.StatusBadRequest)
		log.Error("contact_handler: error in strconv.Atoi(id)", "error", err)
		return
	}

	//Search for specific contact
	var contact *Contact = nil
	for _, c := range contacts {
		if c.ID == id_int {
			contact = &c
			break
		}
	}
	if contact == nil {
		http.Error(w, "Error, contact not found", http.StatusBadRequest)
		log.Error("contact_handler: contact not found")
		return
	}

	data := PageData{[]Contact{*contact}, id_query}

	//Show contact information
	err = app.Templates.Render(w, "index", data)
	if err != nil {
		http.Error(w, "Error finding contact", http.StatusBadRequest)
		log.Error("contact_handler: error in app.Templates.Render()", "error", err)
		return
	}
}

type Form_data struct {
	Email      string
	First_name string
	Last_name  string
	Phone      string
	Errors     map[string]string
}

func (app *app) add_contact_post_handler(w http.ResponseWriter, r *http.Request) {

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

	// Add contact to contacts
	contacts = append(contacts, Contact{
		// depends on initial json file
		len(contacts) + 3,
		first + " " + last,
		email,
		phone,
	})

	form_data := Form_data{
		Email:      email,
		First_name: first,
		Last_name:  last,
		Phone:      phone,
		Errors:     errors,
	}

	err := app.Templates.Render(w, "new", form_data)
	if err != nil {
		http.Error(w, "Error, could not render page", http.StatusInternalServerError)
		log.Error("add_contact_post_handler: error in app.Templates.Render()", "error", err)
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

// Auxiliar functions
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
		return fmt.Errorf("contact_handler: error in osReadFile: %w", err)
	}

	err = json.Unmarshal(contacts_data, &contacts)
	if err != nil {
		return fmt.Errorf("contact_handler: error in json.Unmarhsall: %w", err)
	}

	return nil
}
