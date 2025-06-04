package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/a-h/templ"
	templruntime "github.com/a-h/templ/runtime"
	"github.com/google/uuid"
)

var log *slog.Logger

var contacts_set []Contact
var contacts_data []byte

func main() {

	//init logger
	log = slog.New(slog.NewJSONHandler(os.Stdout, nil))

	//loaf contacts
	err := load_contacts()
	if err != nil {
		log.Error("Error in load_contacts()", "error", err)
	}

	//Define mux
	mux := http.NewServeMux()

	mux.Handle("GET /", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/contact", http.StatusFound)
	}))
	mux.Handle("GET /contact", http.HandlerFunc(contact_handler))
	mux.Handle("GET /contact/{id}", http.HandlerFunc(contact_handler))

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

type Contact struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone"`
}

// /contacts/{id}
func contact_handler(w http.ResponseWriter, r *http.Request) {

	// Extract id from request query
	id_string := r.PathValue("id")
	if id_string == "" {
		//Default response
		w.Header().Set("Content-Type", "text/html")
		_, err := w.Write(contacts_data)
		if err != nil {
			http.Error(w, "Error providing contacts", http.StatusInternalServerError)
			log.Error("contact_handler: error in default w.Write()", "error", err)
			return
		}
		com := hi("Pablo")
		com.Render(context.Background(), w)

	} else {
		//Parse id
		id_int, err := strconv.Atoi(id_string)
		if err != nil {
			http.Error(w, "Invalid id", http.StatusBadRequest)
			log.Info("contact_handler: could not parse id", "error", err)
			return
		}

		//Search specific contact
		var found = -1
		for _, contact := range contacts_set {

			if contact.ID == id_int {
				found = id_int
				contact_data, err := json.Marshal(contact)
				if err != nil {
					http.Error(w, "Error providing contact information", http.StatusInternalServerError)
					log.Error("contact_handler: error in json.Marshall", "error", err)
					return
				}

				// Show response
				w.Header().Set("Content-Type", "text/html")

				_, err = w.Write(contact_data)
				if err != nil {
					http.Error(w, "Error providing contact information", http.StatusInternalServerError)
					log.Error("contact_handler: error in w.Write()", "error", err)
					return
				}
				break
			}
		}

		if found == -1 {
			http.Error(w, "Error, contact not found in database", http.StatusNotFound)
			log.Info("contact_handler: contact not found")
			return
		}
	}
}

func hi(name string) templ.Component {
	return templruntime.GeneratedTemplate(func(templ_7745c5c3_Input templruntime.GeneratedComponentInput) (templ_7745c5c3_Err error) {
		templ_7745c5c3_W, ctx := templ_7745c5c3_Input.Writer, templ_7745c5c3_Input.Context
		if templ_7745c5c3_CtxErr := ctx.Err(); templ_7745c5c3_CtxErr != nil {
			return templ_7745c5c3_CtxErr
		}
		templ_7745c5c3_Buffer, templ_7745c5c3_IsBuffer := templruntime.GetBuffer(templ_7745c5c3_W)
		if !templ_7745c5c3_IsBuffer {
			defer func() {
				templ_7745c5c3_BufErr := templruntime.ReleaseBuffer(templ_7745c5c3_Buffer)
				if templ_7745c5c3_Err == nil {
					templ_7745c5c3_Err = templ_7745c5c3_BufErr
				}
			}()
		}
		ctx = templ.InitializeContext(ctx)
		templ_7745c5c3_Var1 := templ.GetChildren(ctx)
		if templ_7745c5c3_Var1 == nil {
			templ_7745c5c3_Var1 = templ.NopComponent
		}
		ctx = templ.ClearChildren(ctx)
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 1, "<div>Hellou ")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		var templ_7745c5c3_Var2 string
		templ_7745c5c3_Var2, templ_7745c5c3_Err = templ.JoinStringErrs(name)
		if templ_7745c5c3_Err != nil {
			return templ.Error{Err: templ_7745c5c3_Err, FileName: `hi.templ`, Line: 4, Col: 21}
		}
		_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var2))
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 2, "!</div>")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		return nil
	})
}

var _ = templruntime.GeneratedTemplate
