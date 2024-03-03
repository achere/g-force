package main

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/achere/g-force/pkg/coverage"
	"github.com/achere/g-force/pkg/sfapi"
)

type config struct {
	ApiVersion   string `json:"apiVersion"`
	BaseUrl      string `json:"baseUrl"`
	ClientId     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
}

type manifest struct {
	Types []struct {
		Members []string `xml:"members"`
		Name    string   `xml:"name"`
	} `xml:"types"`
}

func main() {
	cfgArg := flag.String("config", "config.json", "Path to SF org authorisation information (config.json)")
	pkgArg := flag.String("package", "package.xml", "Path to the deployment artifact (package.xml)")

	flag.Parse()

	cfg, err := loadConfig(*cfgArg)
	if err != nil {
		log.Fatal(err)
	}

	apexClasses, apexTriggers, err := loadApex(*pkgArg)
	if err != nil {
		log.Fatal(err)
	}

	if len(apexClasses) == 0 && len(apexTriggers) == 0 {
		os.Exit(0)
	}

	con := &sfapi.Connection{
		ApiVersion:   cfg.ApiVersion,
		BaseUrl:      cfg.BaseUrl,
		ClientId:     cfg.ClientId,
		ClientSecret: cfg.ClientSecret,
	}

	tests, err := coverage.RequestTestsMaxCoverage(con, apexClasses, apexTriggers)
	if err != nil {
		log.Fatal(err)
	}

	output := strings.Join(tests, " ")
	fmt.Print(output + " ")
	os.Exit(0)
}

func loadConfig(pathToCfg string) (config, error) {
	cfgFile, err := os.Open(pathToCfg)
	if err != nil {
		return config{}, fmt.Errorf("os.Open: %w", err)
	}
	defer cfgFile.Close()

	decoder := json.NewDecoder(cfgFile)
	var cfg config
	if err = decoder.Decode(&cfg); err != nil {
		return config{}, fmt.Errorf("decoder.Decode: %w", err)
	}

	if cfg.BaseUrl == "" || cfg.ClientId == "" || cfg.ClientSecret == "" {
		return config{}, errors.New("Missing required parameters in config " + pathToCfg)
	}

	return cfg, nil
}

func loadApex(pathToPkg string) ([]string, []string, error) {
	pkgFile, err := os.Open(pathToPkg)
	if err != nil {
		return []string{}, []string{}, fmt.Errorf("os.Open: %w", err)
	}
	defer pkgFile.Close()

	var pkg manifest
	if err := xml.NewDecoder(pkgFile).Decode(&pkg); err != nil {
		return []string{}, []string{}, fmt.Errorf("xml.Decoder.Decode: %w", err)
	}

	apexClasses, apexTriggers := make([]string, 0), make([]string, 0)
	for _, t := range pkg.Types {
		switch t.Name {
		case "ApexClass":
			apexClasses = append(apexClasses, t.Members...)
		case "ApexTrigger":
			apexTriggers = append(apexTriggers, t.Members...)
		}
	}

	return apexClasses, apexTriggers, nil
}
