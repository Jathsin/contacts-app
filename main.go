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

	mux.HandleFunc("GET /contact", app.contact_handler)

	// mux.HandleFunc("POST /contact", app.add_contact_handler)

	mux.HandleFunc("GET /contact/{id}", app.contact_handler)

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
	http.Redirect(w, r, "/contact", http.StatusFound)
}

type Contact struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone"`
}

// @TODO: divide into two handlers?
// /contacts/{id}
func (app *app) contact_handler(w http.ResponseWriter, r *http.Request) {

	id_string := r.PathValue("id")

	var contact_search []Contact

	if id_string == "" {
		contact_search = contacts
	} else {
		c := Contact{}
		err := find_contact(id_string, &c)
		if err != nil {
			//@TODO: preguntar a Pablo cómo distinguir código de estatus
			http.Error(w, "Error finding contact: "+err.Error(), http.StatusBadRequest)
			log.Error("contact_handler: error in app.Templates.Render()", "error", err)
			return
		}

		contact_search = append(contact_search, c)
	}

	//Show contact information response
	w.Header().Set("Content-Type", "text/html")

	err := app.Templates.Render(w, "index", contact_search)
	if err != nil {
		http.Error(w, "Error providing contact information", http.StatusInternalServerError)
		log.Error("contact_handler: error in app.Templates.Render()", "error", err)
		return
	}
}

// func (app *app) add_contact_handler(w http.ResponseWriter, r *http.Request) {}

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

func find_contact(id string, contact *Contact) error {

	//Parse id
	id_int, err := strconv.Atoi(id)
	if err != nil {
		return fmt.Errorf("find_contact: error in strconv.Atoi(id): %w", err)
	}
	//Search for specific contact
	for _, c := range contacts {
		if c.ID == id_int {
			*contact = c
			return nil
		}
	}
	return fmt.Errorf("find_contact: contact not found")
}
