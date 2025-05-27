package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/achere/g-force/pkg/sfapi"
	"github.com/stretchr/testify/assert"
)

func TestCreateCollection(t *testing.T) {
	tests := []struct {
		name                    string
		records                 []CollectionsRecord
		mockHandler             http.HandlerFunc
		expectedResponseLenghts []int
		expectError             bool
	}{
		{
			name:    "Single batch success",
			records: makeRecords(150),
			mockHandler: func(w http.ResponseWriter, r *http.Request) {
				var body collectionsRequest
				json.NewDecoder(r.Body).Decode(&body)
				resp := makeSuccessResponses(len(body.Records))
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(resp)
			},
			expectedResponseLenghts: []int{150},
			expectError:             false,
		},
		{
			name:    "Single batch error",
			records: makeRecords(100),
			mockHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintln(w, "Internal Server Error")
			},
			expectedResponseLenghts: []int{0},
			expectError:             true,
		},
		{
			name:    "Two batches",
			records: makeRecords(250),
			mockHandler: func(w http.ResponseWriter, r *http.Request) {
				var body collectionsRequest
				json.NewDecoder(r.Body).Decode(&body)
				resp := makeSuccessResponses(len(body.Records))
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(resp)
			},
			expectedResponseLenghts: []int{200, 50},
			expectError:             false,
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

			response, err := CollectionsCreate(ctx, c, false, tt.records)

			if tt.expectError {
				assert.Error(t, err, "Expected an error")
				if len(tt.records) > 200 {
					assert.Contains(t, err.Error(), "batch", "Error should reference failed batch")
				}
			} else {
				assert.NoError(t, err, "Expected no error")
			}

			resLenghts := make([]int, len(tt.expectedResponseLenghts))
			for i, r := range response {
				resLenghts[i] = len(r)
			}
			slices.SortFunc(resLenghts, func(a, b int) int { return b - a })

			assert.Equal(t, tt.expectedResponseLenghts, resLenghts, "Unexpected response lengths")
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

func makeRecords(n int) []CollectionsRecord {
	records := make([]CollectionsRecord, n)
	for i := range n {
		records[i] = CollectionsRecord{
			Attributes: CollectionRecord_Attributes{Type: "Account"},
			Fields:     map[string]any{"Name": fmt.Sprintf("Test%d", i)},
		}
	}
	return records
}

func makeSuccessResponses(n int) []CollectionsResponse {
	res := make([]CollectionsResponse, n)
	for i := range n {
		res[i] = CollectionsResponse{
			ID:      fmt.Sprintf("001%03d", i),
			Success: true,
			Errors:  []CollectionsError{},
		}
	}
	return res
}
