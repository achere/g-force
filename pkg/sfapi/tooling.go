package sfapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type apexCodeCoverageResponse struct {
	Records []ApexCodeCoverage `json:"records"`
}

type ApexCodeCoverage struct {
	ApexTestClass struct {
		Name string `json:"Name"`
		Id   string `json:"Id"`
	} `json:"ApexTestClass"`
	TestMethodName     string `json:"TestMethodName"`
	ApexClassOrTrigger struct {
		Attributes struct {
			Type string `json:"type"`
		} `json:"attributes"`
		Name string `json:"Name"`
		Id   string `json:"Id"`
	} `json:"ApexClassOrTrigger"`
	Coverage struct {
		CoveredLines   []int `json:"coveredLines"`
		UncoveredLines []int `json:"uncoveredLines"`
	} `json:"Coverage"`
}

type executeAnonymousResponse struct {
	Line                int    `json:"line"`
	Column              int    `json:"column"`
	Compiled            bool   `json:"compiled"`
	Success             bool   `json:"success"`
	CompileProblem      string `json:"compileProblem"`
	ExceptionStackTrace string `json:"exceptionStackTrace"`
	ExceptionMessage    string `json:"exceptionMessage"`
}

type ClassCoverage struct {
	ClassName string
}

func (c *Connection) GetCoverage(apexNames []string) ([]ApexCodeCoverage, error) {
	baseUrl := c.BaseUrl + "/services/data/v" + c.ApiVersion + "/tooling/query/?q="

	query := "SELECT+ApexTestClass.Name,ApexTestClass.Id,TestMethodName,ApexClassOrTrigger.Name,ApexClassOrTrigger.Id,Coverage+FROM+ApexCodeCoverage+WHERE+ApexClassOrTrigger.Name+IN+('"
	query += strings.Join(apexNames, "','")
	query += "')"

	req, err := http.NewRequest("GET", baseUrl+query, nil)
	if err != nil {
		return []ApexCodeCoverage{}, fmt.Errorf("http.NewRequest: %w", err)
	}

	respBody, err := c.makeRequest(req)
	if err != nil {
		return []ApexCodeCoverage{}, fmt.Errorf("c.makeRequest: %w", err)
	}

	var parsedResponse apexCodeCoverageResponse
	err = json.Unmarshal(respBody, &parsedResponse)
	if err != nil {
		return []ApexCodeCoverage{}, fmt.Errorf("json.Unmarshal: %w", err)
	}

	return parsedResponse.Records, nil
}

func (c *Connection) ExecuteAnonymousRest(body string) error {
	strippedBody := url.QueryEscape(strings.Replace(body, "\n", " ", -1))
	url := c.BaseUrl + "/services/data/v" + c.ApiVersion + "/tooling/executeAnonymous/?anonymousBody=" + strippedBody

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("http.NewRequest: %w", err)
	}

	respBody, err := c.makeRequest(req)
	if err != nil {
		return fmt.Errorf("c.makeRequest: %w", err)
	}

	var parsedResponse executeAnonymousResponse
	err = json.Unmarshal(respBody, &parsedResponse)
	if err != nil {
		return fmt.Errorf("json.Unmarshal: %w", err)
	}

	if parsedResponse.Success {
		return nil
	}

	if parsedResponse.Compiled {
		msg := "Error on line " +
			strconv.Itoa(parsedResponse.Line) + ":" + strconv.Itoa(parsedResponse.Column) + " - " +
			parsedResponse.ExceptionMessage + " " + parsedResponse.ExceptionStackTrace
		return errors.New(msg)
	}

	return errors.New("Didn't compile: " + parsedResponse.CompileProblem)
}
