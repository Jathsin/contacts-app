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

var contacts_set []Contact
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

	// Extract id from request query
	id_string := r.PathValue("id")

	if id_string == "" {
		//Default response
		w.Header().Set("Content-Type", "text/html")

		err := app.Templates.Render(w, "index", contacts_set)
		//_, err := w.Write(contacts_data)
		if err != nil {
			http.Error(w, "Error providing contacts", http.StatusInternalServerError)
			log.Error("contact_handler: error in default w.Write()", "error", err)
			return
		}
		return
	}

	//Parse id
	id_int, err := strconv.Atoi(id_string)
	if err != nil {
		http.Error(w, "Invalid id", http.StatusBadRequest)
		log.Info("contact_handler: could not parse id", "error", err)

		return
	}
	//Search for specific contact
	var c Contact
	for _, contact := range contacts_set {
		if contact.ID == id_int {
			c = contact
			break
		}
	}
	if c.ID == -1 {
		http.Error(w, "Error, contact not found in database", http.StatusNotFound)
		log.Info("contact_handler: contact not found")
		return
	}

	// Parse response
	contact_data, err := json.Marshal(c)
	if err != nil {
		http.Error(w, "Error providing contact information", http.StatusInternalServerError)
		log.Error("contact_handler: error in json.Marshall", "error", err)
		return
	}

	// Show response
	w.Header().Set("Content-Type", "application/json")

	_, err = w.Write(contact_data)
	if err != nil {
		http.Error(w, "Error providing contact information", http.StatusInternalServerError)
		log.Error("contact_handler: error in w.Write()", "error", err)
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

	err = json.Unmarshal(contacts_data, &contacts_set)
	if err != nil {
		return fmt.Errorf("contact_handler: error in json.Unmarhsall: %w", err)
	}

	return nil
}
