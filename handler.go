package servers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/bool64/ctxd"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	v3 "github.com/swaggest/swgui/v3"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
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

// xhttpCodeResponseModifier is used to modify the Response status code using x-http-code header
// by setting a different code than 200 on success or 500 on failure.
func xhttpCodeResponseModifier() func(ctx context.Context, w http.ResponseWriter, _ proto.Message) error {
	return func(ctx context.Context, w http.ResponseWriter, _ proto.Message) error {
		w.Header().Del("Grpc-Metadata-Content-Type")

		md, ok := runtime.ServerMetadataFromContext(ctx)
		if !ok {
			return nil
		}

		// set http status code
		if vals := md.HeaderMD.Get("x-http-code"); len(vals) > 0 {
			code, err := strconv.Atoi(vals[0])
			if err != nil {
				return err
			}

			// delete the headers to not expose any grpc-metadata in http response
			delete(md.HeaderMD, "x-http-code")
			delete(w.Header(), "Grpc-Metadata-X-Http-Code")

			w.WriteHeader(code)
		}

		return nil
	}
}

type errorResponse struct {
	Code    int                    `json:"code"`
	Message string                 `json:"message,omitempty"`
	Error   string                 `json:"error,omitempty"`
	Details []errorResponseDetails `json:"details,omitempty"`
}

type errorResponseDetails struct {
	Field       string `json:"field,omitempty"`
	Description string `json:"description,omitempty"`
}

func customizeErrorHandler() func(context.Context, *runtime.ServeMux, runtime.Marshaler, http.ResponseWriter, *http.Request, error) {
	return func(_ context.Context, _ *runtime.ServeMux, marshaler runtime.Marshaler, w http.ResponseWriter, _ *http.Request, err error) {
		// Extract gRPC status error
		st, ok := status.FromError(err)
		if !ok {
			// Fallback for non-gRPC errors
			// There is no need to check if extract sinc we are sure that the error is a gRPC error
			st, _ = status.FromError(WrapError(codes.Internal, err, "ups, something went wrong!")) //nolint:contextcheck
		}

		// Default HTTP status code
		httpStatus := runtime.HTTPStatusFromCode(st.Code())

		// Transform google.rpc.BadRequest into a simplified structure
		var (
			details []errorResponseDetails
			errID   string
		)

		for _, d := range st.Details() {
			errInfo, ok := d.(*errdetails.ErrorInfo)
			if !ok {
				continue
			}

			for f, m := range errInfo.GetMetadata() {
				if f == "error_id" {
					errID = m

					continue
				}

				details = append(details, errorResponseDetails{
					Field:       f,
					Description: m,
				})
			}
		}

		// Delete the gRPC metadata from the response
		w.Header().Del("Grpc-Metadata-Content-Type")

		// Write the custom error response
		w.WriteHeader(httpStatus)

		_ = json.NewEncoder(w).Encode(errorResponse{ //nolint:errcheck
			Code:    httpStatus,
			Message: st.Message(),
			Error:   errID,
			Details: details,
		})
	}
}
