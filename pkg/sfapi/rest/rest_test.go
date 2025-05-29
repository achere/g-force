package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/achere/g-force/pkg/sfapi"
	"github.com/stretchr/testify/assert"
)

func TestQuery(t *testing.T) {
	const (
		recordCount = 101
		errorCode   = "MALFORMED_QUERY"
	)
	tests := []struct {
		name        string
		mockHandler http.HandlerFunc
		verify      func(any, error)
	}{
		{
			name: "Success",
			mockHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)

				records := make([]Record, recordCount)
				for i := range recordCount {
					records[i] = Record{
						Attributes: Record_Attributes{Type: "Account"},
						Fields:     map[string]any{"Name": fmt.Sprintf("Test%d", i)},
					}
				}

				json.NewEncoder(w).Encode(QueryResponseSuccess{Records: records})
			},
			verify: func(resp any, err error) {
				res, ok := resp.([]Record)
				assert.NoError(t, err)
				assert.Truef(t, ok, "Incorrect response format, expected []Record, got: %v", resp)
				assert.Equal(t, recordCount, len(res))
			},
		},
		{
			name: "Fail",
			mockHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode([]QueryResponseError{{ErrorCode: errorCode}})
			},
			verify: func(a any, err error) {
				assert.Error(t, err)
				assert.ErrorContains(t, err, errorCode)
			},
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mockServer := httptest.NewServer(respondWithToken(tt.mockHandler))
			defer mockServer.Close()

			c := &sfapi.Connection{
				BaseUrl:    mockServer.URL,
				ApiVersion: "60.0",
				HttpClient: mockServer.Client(),
			}

			tt.verify(Query(c, ctx, "query"))
		})
	}
}

func respondWithToken(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/services/oauth2/token" {
			h.ServeHTTP(w, r)
			return
		}

		body := sfapi.TokenResponse{AccessToken: "token"}
		res, _ := json.Marshal(body)
		w.Write(res)
		w.Header().Add("Content-Type", "application/json")
	})
}
