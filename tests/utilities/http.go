package utilities

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	sharedInfrastructure "github.com/PChaparro/serpentarius/internal/modules/shared/infrastructure"
)

type AuthOptions struct {
	Token string // optional, if empty uses env
	Skip  bool   // if true, do not set Authorization header
}

type PostAPIRequest struct {
	Router  http.Handler
	URL     string
	Body    string
	Auth    AuthOptions
	Headers map[string]string
}

// PostToAPI sends a POST request to the API with custom body and headers
func PostToAPI(req PostAPIRequest) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("POST", req.URL, strings.NewReader(req.Body))

	r.Header.Set("Content-Type", "application/json")

	if !req.Auth.Skip {
		token := req.Auth.Token
		if token == "" {
			token = sharedInfrastructure.GetEnvironment().AuthSecret
		}
		r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	}

	for headerName, headerContent := range req.Headers {
		if headerName == "Content-Type" || headerName == "Authorization" {
			continue
		}
		r.Header.Set(headerName, headerContent)
	}

	req.Router.ServeHTTP(w, r)
	return w
}

// ParseJSONResponse parses the response body as JSON and returns the result as an any and an error.
func ParseJSONResponse(w *httptest.ResponseRecorder) (any, error) {
	var resp any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	return resp, err
}
