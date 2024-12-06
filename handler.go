package servers

import (
	"encoding/json"
	"net/http"

	"github.com/bool64/ctxd"
	v3 "github.com/swaggest/swgui/v3"
)

// NewRestRootHandler creates a handler for an endpoint to response on / path.
func NewRestRootHandler(serviceName string, links ...any) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		rw.Header().Set("Content-Type", "text/html")

		body := "Welcome to " + serviceName

		if links != nil {
			// Add links to the body
			body += "<br><br>Links:<br>"

			// paar text and link
			for i := 0; i < len(links); i += 2 {
				body += "<a href=\"" + links[i+1].(string) + "\">" + links[i].(string) + "</a><br>"
			}
		}

		_, err := rw.Write([]byte(body))
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
		}
	})
}

// NewRestVersionHandler creates a handler for an endpoint to response on /version path to show the version of the api.
func NewRestVersionHandler() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		rw.Header().Set("Content-Type", "application/json")

		err := json.NewEncoder(rw).Encode(Info())
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
		}
	})
}

// NewRestAPIDocsHandlers creates a handler for an endpoint to response on /docs path to show the api documentation.
// It returns a map of handlers for the pattern and the handler.
func NewRestAPIDocsHandlers(serviceName, basePath, swaggerPath string, swaggerJSON []byte) map[string]http.Handler {
	// handler root path
	swh := v3.NewHandler(serviceName, swaggerPath, basePath)

	return map[string]http.Handler{
		"/docs/service.swagger.json": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			_, err := w.Write(swaggerJSON)
			if err != nil {
				panic(ctxd.WrapError(r.Context(), err, "failed to load /docs/service.swagger.json file"))
			}
		}),
		"/docs": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			swh.ServeHTTP(w, r)
		}),
		"/docs/swagger-ui-bundle.js": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			swh.ServeHTTP(w, r)
		}),
		"/docs/swagger-ui-standalone-preset.js": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			swh.ServeHTTP(w, r)
		}),
		"/docs/swagger-ui.css": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			swh.ServeHTTP(w, r)
		}),
	}
}
