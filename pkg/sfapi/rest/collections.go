package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/achere/g-force/pkg/sfapi"
	multierror "github.com/hashicorp/go-multierror"
	"golang.org/x/sync/errgroup"
)

type collectionsRequest struct {
	AllOrNone bool                `json:"allOrNone"`
	Records   []CollectionsRecord `json:"records"`
}

type CollectionsRecord struct {
	Attributes CollectionRecord_Attributes `json:"attributes"`
	Fields     map[string]any              `json:"-,inline"`
}

type CollectionRecord_Attributes struct {
	Type string `json:"type"`
}

func (r CollectionsRecord) MarshalJSON() ([]byte, error) {
	output := make(map[string]any)

	output["attributes"] = r.Attributes

	for key, value := range r.Fields {
		output[key] = value
	}

	return json.Marshal(output)
}

func (r *CollectionsRecord) UnmarshalJSON(data []byte) error {
	type temp struct {
		Attributes CollectionRecord_Attributes `json:"attributes"`
		Fields     map[string]any              `json:"-"`
	}

	var alias temp
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}

	r.Attributes = alias.Attributes
	r.Fields = make(map[string]any)

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	for key, value := range raw {
		if key == "attributes" {
			continue
		}

		if m, ok := value.(map[string]any); ok {
			if _, hasAttrs := m["attributes"]; hasAttrs {
				nestedBytes, err := json.Marshal(m)
				if err == nil {
					var nested CollectionsRecord
					if err := json.Unmarshal(nestedBytes, &nested); err == nil {
						r.Fields[key] = &nested
						continue
					}
				}
			}
		}

		r.Fields[key] = value
	}
	return nil
}

type CollectionsResponse struct {
	ID      string             `json:"id,omitempty"`
	Success bool               `json:"success"`
	Errors  []CollectionsError `json:"errors"`
}

type CollectionsError struct {
	StatusCode string   `json:"statusCode"`
	Message    string   `json:"message"`
	Fields     []string `json:"fields"`
}

// CollectionsCreate uses sObject Collections REST API to create records in batches of 200.
// Batches are sent in parrallel so there can't be dependencies between them.
// Warning: allOrNone parameter works only within each batch, if some of the batches were
// successful, they will not be rolled back.
func CollectionsCreate(
	c *sfapi.Connection,
	ctx context.Context,
	allOrNone bool,
	records []CollectionsRecord,
) ([][]CollectionsResponse, error) {
	var mu sync.Mutex
	var result *multierror.Error
	size := len(records)/200 + 1
	responses := make([][]CollectionsResponse, size)
	g, _ := errgroup.WithContext(ctx)

	for i := range size {
		from := i * 200
		to := min(i*200+200, len(records))
		batch := records[from:to]

		body := collectionsRequest{
			AllOrNone: allOrNone,
			Records:   batch,
		}

		jsonBody, err := json.Marshal(&body)
		if err != nil {
			return nil, fmt.Errorf("json.Marshall: %w", err)
		}

		cUrl := c.BaseUrl + "/services/data/v" + c.ApiVersion + "/composite/sobjects"
		req, err := http.NewRequest(http.MethodPost, cUrl, bytes.NewBuffer(jsonBody))
		if err != nil {
			return nil, fmt.Errorf("http.NewRequest: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")

		g.Go(func() error {
			resp, err := c.DoRequest(ctx, req)
			if err != nil {
				mu.Lock()
				result = multierror.Append(
					result,
					fmt.Errorf("batch from %v to %v failed with err: %v", from, to, err),
				)
				mu.Unlock()
				return nil
			}

			var batchResponses []CollectionsResponse
			if err := json.Unmarshal(resp, &batchResponses); err != nil {
				mu.Lock()
				result = multierror.Append(
					result,
					fmt.Errorf("batch from %v to %v decode failed: %w", from, to, err),
				)
				mu.Unlock()
				return nil
			}

			mu.Lock()
			responses[i] = batchResponses
			mu.Unlock()

			return nil
		})
	}

	g.Wait()

	return responses, result.ErrorOrNil()
}
