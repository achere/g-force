package main

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
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

func main() {
	configArg := flag.String(
		"config",
		"config.json",
		"Path to SF org authentication information - config.json",
	)
	packagesArg := flag.String(
		"packages",
		"package.xml",
		"Comma-separated list of paths to manifest files - package.xml",
	)
	strategyArg := flag.String(
		"strategy",
		"MaxCoverage",
		`Choose the strategy of getting coverage:
	- "MaxCoverage" to ouput all tests that provide coverage for the passed in Apex
	- "MaxCoverageWithDeps" to output all tests for the passed in Apex and its dependencies`,
	)

	flag.Parse()

	if *strategyArg != coverage.StratMaxCoverage &&
		*strategyArg != coverage.StratMaxCoverageWithDeps {
		fmt.Fprintf(
			os.Stderr,
			`unsupported strategy provided: %v; list of supported values: 
	- MaxCoverage
	- MaxCoverageWithDeps
`,
			*strategyArg,
		)
		os.Exit(1)
	}

	cfg, err := loadConfig(*configArg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading config: %v\n", err.Error())
		os.Exit(1)
	}

	classes, triggers, err := loadApex(*packagesArg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading apex from package: %v\n", err.Error())
		os.Exit(1)
	}

	if len(classes) == 0 && len(triggers) == 0 {
		os.Exit(0)
	}

	con := &sfapi.Connection{
		ApiVersion:   cfg.ApiVersion,
		BaseUrl:      cfg.BaseUrl,
		ClientId:     cfg.ClientId,
		ClientSecret: cfg.ClientSecret,
	}

	tests, err := coverage.RequestTestsWithStrategy(context.Background(), *strategyArg, con, classes, triggers)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error requesting coverage: %v\n", err.Error())
		os.Exit(1)
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

	if cfg.ApiVersion == "" || cfg.BaseUrl == "" || cfg.ClientId == "" || cfg.ClientSecret == "" {
		return config{}, errors.New("missing required parameters in config " + pathToCfg)
	}

	return cfg, nil
}

func loadApex(pathToPkg string) ([]string, []string, error) {
	var (
		classMap   = make(map[string]bool)
		triggerMap = make(map[string]bool)
	)

	paths := strings.Split(pathToPkg, ",")
	for _, p := range paths {

		pkgFile, err := os.Open(p)
		if err != nil {
			return []string{}, []string{}, fmt.Errorf("os.Open: %w", err)
		}
		defer pkgFile.Close()

		var pkg struct {
			Types []struct {
				Members []string `xml:"members"`
				Name    string   `xml:"name"`
			} `xml:"types"`
		}
		if err := xml.NewDecoder(pkgFile).Decode(&pkg); err != nil {
			return []string{}, []string{}, fmt.Errorf("xml.Decoder.Decode: %w", err)
		}

		for _, t := range pkg.Types {
			switch t.Name {
			case "ApexClass":
				for _, v := range t.Members {
					classMap[v] = true
				}
			case "ApexTrigger":
				for _, v := range t.Members {
					triggerMap[v] = true
				}
			}
		}
	}

	classes, triggers := make([]string, 0), make([]string, 0)
	for c := range classMap {
		classes = append(classes, c)
	}
	for c := range triggerMap {
		triggers = append(triggers, c)
	}
	return classes, triggers, nil
}
