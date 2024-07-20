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

type Tooling interface {
	RequestCoverage(apexNames []string) ([]ApexCodeCoverage, error)
	RequestApexDependencies(metadataComponentTypes []string) ([]MetadataComponentDependency, error)
	RequestApexClasses(names []string) ([]ApexClass, error)
	ExecuteAnonymousRest(body string) error
}

type ToolingApiObject interface {
	ApexCodeCoverage | MetadataComponentDependency | ApexClass
}

type ApexCodeCoverage struct {
	ApexTestClass      ApexCodeCoverage_ApexTestClass      `json:"ApexTestClass"`
	ApexClassOrTrigger ApexCodeCoverage_ApexClassOrTrigger `json:"ApexClassOrTrigger"`
	Coverage           ApexCodeCoverage_Coverage           `json:"Coverage"`
}

type ApexCodeCoverage_ApexTestClass struct {
	Name string `json:"Name"`
	Id   string `json:"Id"`
}

type ApexCodeCoverage_ApexClassOrTrigger struct {
	Attributes struct {
		Type string `json:"type"`
	} `json:"attributes"`
	Name string `json:"Name"`
	Id   string `json:"Id"`
}

type ApexCodeCoverage_Coverage struct {
	CoveredLines   []int `json:"coveredLines"`
	UncoveredLines []int `json:"uncoveredLines"`
}

type ApexClass struct {
	Id          string                `json:"Id"`
	Name        string                `json:"Name"`
	IsValid     string                `json:"IsValid"`
	Body        string                `json:"Body"`
	SymbolTable ApexClass_SymbolTable `json:"SymbolTable"`
}

type ApexClass_SymbolTable struct {
	TableDeclaration struct {
		Annotations []struct {
			Name string `json:"name"`
		} `json:"annotations"`
		Modifiers []string `json:"modifiers"`
	} `json:"tableDeclaration"`
}

type MetadataComponentDependency struct {
	Name    string `json:"MetadataComponentName"`
	Id      string `json:"MetadataComponentId"`
	Type    string `json:"MetadataComponentType"`
	RefType string `json:"RefMetadataComponentType"`
	RefName string `json:"RefMetadataComponentName"`
	RefId   string `json:"RefMetadataComponentId"`
}

func (c *Connection) RequestCoverage(apexNames []string) ([]ApexCodeCoverage, error) {
	query := "SELECT+ApexTestClass.Name,ApexTestClass.Id,ApexClassOrTrigger.Name,ApexClassOrTrigger.Id,Coverage+FROM+ApexCodeCoverage+WHERE+ApexClassOrTrigger.Name+IN+('"
	query += strings.Join(apexNames, "','")
	query += "')"

	return QueryToolingApi[ApexCodeCoverage](c, query)
}

func (c *Connection) RequestApexDependencies(metadataComponentTypes []string) ([]MetadataComponentDependency, error) {
	query := "SELECT+MetadataComponentName,MetadataComponentId,MetadataComponentType,RefMetadataComponentType,RefMetadataComponentName,RefMetadataComponentId+FROM+MetadataComponentDependency+WHERE+RefMetadataComponentType+IN+('ApexClass','ApexTrigger')+AND+MetadataComponentType+IN+('"
	query += strings.Join(metadataComponentTypes, "','")
	query += "')"

	return QueryToolingApi[MetadataComponentDependency](c, query)
}

func (c *Connection) RequestApexClasses(names []string) ([]ApexClass, error) {
	query := "SELECT+Id,Name,SymbolTable+FROM+ApexClass+WHERE+Name+IN+('"
	query += strings.Join(names, "','")
	query += "')"

	return QueryToolingApi[ApexClass](c, query)
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

	var parsedResponse struct {
		Line                int    `json:"line"`
		Column              int    `json:"column"`
		Compiled            bool   `json:"compiled"`
		Success             bool   `json:"success"`
		CompileProblem      string `json:"compileProblem"`
		ExceptionStackTrace string `json:"exceptionStackTrace"`
		ExceptionMessage    string `json:"exceptionMessage"`
	}

	err = json.Unmarshal(respBody, &parsedResponse)
	if err != nil {
		return fmt.Errorf("json.Unmarshal: %w", err)
	}

	if parsedResponse.Success {
		return nil
	}

	if parsedResponse.Compiled {
		msg := "error on line " +
			strconv.Itoa(parsedResponse.Line) + ":" + strconv.Itoa(parsedResponse.Column) + " - " +
			parsedResponse.ExceptionMessage + " " + parsedResponse.ExceptionStackTrace
		return errors.New(msg)
	}

	return errors.New("didn't compile: " + parsedResponse.CompileProblem)
}

func QueryToolingApi[T ToolingApiObject](c *Connection, query string) ([]T, error) {
	baseUrl := c.BaseUrl + "/services/data/v" + c.ApiVersion + "/tooling/query/?q="
	req, err := http.NewRequest("GET", baseUrl+query, nil)
	if err != nil {
		return []T{}, fmt.Errorf("http.NewRequest: %w", err)
	}

	respBody, err := c.makeRequest(req)
	if err != nil {
		return []T{}, fmt.Errorf("c.makeRequest: %w", err)
	}

	var parsedResponse struct {
		Records []T `json:"records"`
	}
	err = json.Unmarshal(respBody, &parsedResponse)
	if err != nil {
		return []T{}, fmt.Errorf("json.Unmarshal: %w", err)
	}

	return parsedResponse.Records, nil
}
