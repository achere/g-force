package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/achere/g-force/pkg/sfapi"
	multierror "github.com/hashicorp/go-multierror"
)

type QueryResponseSuccess struct {
	TotalSize int      `json:"totalSize"`
	Done      bool     `json:"done"`
	Records   []Record `json:"records"`
}

type Record struct {
	Attributes Record_Attributes `json:"attributes"`
	Fields     map[string]any    `json:"-,inline"`
}

type Record_Attributes struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

type QueryResponseError struct {
	Message   string `json:"message"`
	ErrorCode string `json:"errorCode"`
}

func (r *Record) UnmarshalJSON(data []byte) error {
	type temp struct {
		Attributes Record_Attributes `json:"attributes"`
		Fields     map[string]any    `json:"-"`
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
					var nested Record
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

func Query(c *sfapi.Connection, ctx context.Context, query string) ([]Record, error) {
	qUrl := c.BaseUrl + "/services/data/v" + c.ApiVersion + "/query/?q=" + url.QueryEscape(query)

	req, err := http.NewRequest(http.MethodGet, qUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("http.NewRequest: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.DoRequest(ctx, req)
	if err != nil {
		if resp == nil {
			return nil, fmt.Errorf("connection.DoRequest: %w", err)
		}

		var bodyErr []QueryResponseError
		if errFail := json.Unmarshal(resp, &bodyErr); errFail != nil {
			var multiErr *multierror.Error
			multiErr = multierror.Append(multiErr, err, fmt.Errorf("json.Unmarshal: %w", errFail))

			return nil, multiErr.ErrorOrNil()
		}

		return nil, fmt.Errorf("connection.DoRequest: %s:%s", bodyErr[0].ErrorCode, bodyErr[0].Message)
	}

	var bodySucc QueryResponseSuccess
	if errSucc := json.Unmarshal(resp, &bodySucc); errSucc != nil {
		return nil, fmt.Errorf("json.Unmarshal: %w", errSucc)
	}

	return bodySucc.Records, nil
}
