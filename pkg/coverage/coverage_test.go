package coverage

import (
	"testing"

	"github.com/achere/g-force/pkg/sfapi"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestRequestTestsMaxCoverage(t *testing.T) {
	goodCov := []sfapi.ApexCodeCoverage{
		{
			ApexTestClass: sfapi.ApexCodeCoverage_ApexTestClass{Id: "test1", Name: "Class1_Test"},
			ApexClassOrTrigger: sfapi.ApexCodeCoverage_ApexClassOrTrigger{
				Attributes: struct {
					Type string `json:"type"`
				}{Type: "ApexClass"},
				Name: "Class1",
				Id:   "class1",
			},
			Coverage: sfapi.ApexCodeCoverage_Coverage{CoveredLines: []int{1, 2}, UncoveredLines: []int{3, 5, 8, 13, 21, 34, 55, 89}},
		},
		{
			ApexTestClass: sfapi.ApexCodeCoverage_ApexTestClass{Id: "test2", Name: "Trigger1_Test"},
			ApexClassOrTrigger: sfapi.ApexCodeCoverage_ApexClassOrTrigger{
				Attributes: struct {
					Type string `json:"type"`
				}{Type: "ApexTrigger"},
				Name: "Trigger1",
				Id:   "trigger1",
			},
			Coverage: sfapi.ApexCodeCoverage_Coverage{CoveredLines: []int{1, 2, 8, 13, 21, 34, 55, 89}, UncoveredLines: []int{3, 5}},
		},
		{
			ApexTestClass: sfapi.ApexCodeCoverage_ApexTestClass{Id: "test2", Name: "Trigger1_Test"},
			ApexClassOrTrigger: sfapi.ApexCodeCoverage_ApexClassOrTrigger{
				Attributes: struct {
					Type string `json:"type"`
				}{Type: "ApexClass"},
				Name: "Class1",
				Id:   "class1",
			},
			Coverage: sfapi.ApexCodeCoverage_Coverage{CoveredLines: []int{8, 13, 21, 34, 55, 89}, UncoveredLines: []int{1, 2, 3, 5}},
		},
	}

	missClass := []sfapi.ApexCodeCoverage{
		{
			ApexTestClass: sfapi.ApexCodeCoverage_ApexTestClass{Id: "test2", Name: "Trigger1_Test"},
			ApexClassOrTrigger: sfapi.ApexCodeCoverage_ApexClassOrTrigger{
				Attributes: struct {
					Type string `json:"type"`
				}{Type: "ApexTrigger"},
				Name: "Trigger1",
				Id:   "trigger1",
			},
			Coverage: sfapi.ApexCodeCoverage_Coverage{CoveredLines: []int{1, 2, 8, 13, 21, 34, 55, 89}, UncoveredLines: []int{3, 5}},
		},
	}

	insuffCov := []sfapi.ApexCodeCoverage{
		{
			ApexTestClass: sfapi.ApexCodeCoverage_ApexTestClass{Id: "test1", Name: "Class1_Test"},
			ApexClassOrTrigger: sfapi.ApexCodeCoverage_ApexClassOrTrigger{
				Attributes: struct {
					Type string `json:"type"`
				}{Type: "ApexClass"},
				Name: "Class1",
				Id:   "class1",
			},
			Coverage: sfapi.ApexCodeCoverage_Coverage{CoveredLines: []int{1, 2}, UncoveredLines: []int{3, 5, 8, 13, 21, 34, 55, 89}},
		},
		{
			ApexTestClass: sfapi.ApexCodeCoverage_ApexTestClass{Id: "test2", Name: "Trigger1_Test"},
			ApexClassOrTrigger: sfapi.ApexCodeCoverage_ApexClassOrTrigger{
				Attributes: struct {
					Type string `json:"type"`
				}{Type: "ApexTrigger"},
				Name: "Trigger1",
				Id:   "trigger1",
			},
			Coverage: sfapi.ApexCodeCoverage_Coverage{CoveredLines: []int{1, 2, 8, 13, 21, 34, 55, 89}, UncoveredLines: []int{3, 5}},
		},
	}

	requestEmptyApexClasses := func(s []string) ([]sfapi.ApexClass, error) { return []sfapi.ApexClass{}, nil }

	data := []struct {
		name            string
		requestCoverage func([]string) ([]sfapi.ApexCodeCoverage, error)
		tests           []string
		mustErr         bool
	}{
		{
			"success",
			func(apexNames []string) ([]sfapi.ApexCodeCoverage, error) { return goodCov, nil },
			[]string{"Class1_Test", "Trigger1_Test"},
			false,
		},
		{
			"missing class",
			func(apexNames []string) ([]sfapi.ApexCodeCoverage, error) { return missClass, nil },
			[]string{},
			true,
		},
		{
			"insufficient coverage",
			func(apexNames []string) ([]sfapi.ApexCodeCoverage, error) { return insuffCov, nil },
			[]string{},
			true,
		},
	}

	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			ts := RequesterStub{requestCoverage: d.requestCoverage, requestApexClasses: requestEmptyApexClasses}

			tests, err := RequestTestsWithStrategy(StratMaxCoverage, ts, []string{"Class1"}, []string{"Trigger1"})

			if d.mustErr && err == nil {
				t.Errorf("Expected error, got %v\n", tests)
			} else if !d.mustErr && err != nil {
				t.Errorf("Expected result %v, got error %s\n", d.tests, err.Error())
			} else if !slicesEqualIgnoreOrder(d.tests, tests) {
				t.Errorf("Unexpected result: expected %v, got %v\n", d.tests, tests)
			}
		})
	}
}

func TestRequestMaxCoverageWithDeps(t *testing.T) {
	requestApexDependencies := func(metadataComponentTypes []string) ([]sfapi.MetadataComponentDependency, error) {
		return []sfapi.MetadataComponentDependency{
			{
				Name:    "Trigger1",
				Id:      "trigger1",
				Type:    "ApexTrigger",
				RefType: "ApexClass",
				RefName: "Class1",
				RefId:   "class1",
			},
			{
				Name:    "Trigger2",
				Id:      "trigger2",
				Type:    "ApexTrigger",
				RefType: "ApexClass",
				RefName: "Class1",
				RefId:   "class1",
			},
			{
				Name:    "Class2",
				Id:      "class2",
				Type:    "ApexTrigger",
				RefType: "ApexClass",
				RefName: "Class3",
				RefId:   "class3",
			},
		}, nil
	}
	requestApexClasses := func(s []string) ([]sfapi.ApexClass, error) { return []sfapi.ApexClass{}, nil }

	goodCov := []sfapi.ApexCodeCoverage{
		{
			ApexTestClass: sfapi.ApexCodeCoverage_ApexTestClass{Id: "test1", Name: "Class1_Test"},
			ApexClassOrTrigger: sfapi.ApexCodeCoverage_ApexClassOrTrigger{
				Attributes: struct {
					Type string `json:"type"`
				}{Type: "ApexClass"},
				Name: "Class1",
				Id:   "class1",
			},
			Coverage: sfapi.ApexCodeCoverage_Coverage{CoveredLines: []int{1, 2}, UncoveredLines: []int{3, 5, 8, 13, 21, 34, 55, 89}},
		},
		{
			ApexTestClass: sfapi.ApexCodeCoverage_ApexTestClass{Id: "test2", Name: "Trigger1_Test"},
			ApexClassOrTrigger: sfapi.ApexCodeCoverage_ApexClassOrTrigger{
				Attributes: struct {
					Type string `json:"type"`
				}{Type: "ApexTrigger"},
				Name: "Trigger1",
				Id:   "trigger1",
			},
			Coverage: sfapi.ApexCodeCoverage_Coverage{CoveredLines: []int{1, 2, 8, 13, 21, 34, 55, 89}, UncoveredLines: []int{3, 5}},
		},
		{
			ApexTestClass: sfapi.ApexCodeCoverage_ApexTestClass{Id: "test2", Name: "Trigger1_Test"},
			ApexClassOrTrigger: sfapi.ApexCodeCoverage_ApexClassOrTrigger{
				Attributes: struct {
					Type string `json:"type"`
				}{Type: "ApexClass"},
				Name: "Class1",
				Id:   "class1",
			},
			Coverage: sfapi.ApexCodeCoverage_Coverage{CoveredLines: []int{8, 13, 21, 34, 55, 89}, UncoveredLines: []int{1, 2, 3, 5}},
		},
	}

	data := []struct {
		name            string
		requestCoverage func([]string) ([]sfapi.ApexCodeCoverage, error)
		tests           []string
		mustErr         bool
	}{
		{
			"success",
			func(apexNames []string) ([]sfapi.ApexCodeCoverage, error) {
				if !slicesEqualIgnoreOrder(apexNames, []string{"Class1", "Trigger1"}) {
					t.Errorf("Requested coverage with incorrect parameters: %v", apexNames)
				}
				return goodCov, nil
			},
			[]string{"Class1_Test", "Trigger1_Test"},
			false,
		},
	}

	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			ts := RequesterStub{
				requestCoverage:         d.requestCoverage,
				requestApexDependencies: requestApexDependencies,
				requestApexClasses:      requestApexClasses,
			}

			tests, err := RequestTestsWithStrategy(StratMaxCoverageWithDeps, ts, []string{}, []string{"Trigger1"})

			if d.mustErr && err == nil {
				t.Errorf("Expected error, got %v\n", tests)
			} else if !d.mustErr && err != nil {
				t.Errorf("Expected result %v, got error %s\n", d.tests, err.Error())
			} else if !slicesEqualIgnoreOrder(d.tests, tests) {
				t.Errorf("Unexpected result: expected %v, got %v\n", d.tests, tests)
			}
		})
	}
}

func slicesEqualIgnoreOrder(s1, s2 []string) bool {
	return cmp.Equal(s1, s2, cmpopts.SortSlices(func(e1, e2 string) bool { return e1 < e2 }))
}

type RequesterStub struct {
	requestCoverage         func([]string) ([]sfapi.ApexCodeCoverage, error)
	requestApexDependencies func([]string) ([]sfapi.MetadataComponentDependency, error)
	requestApexClasses      func([]string) ([]sfapi.ApexClass, error)
}

func (ts RequesterStub) RequestCoverage(apexNames []string) ([]sfapi.ApexCodeCoverage, error) {
	return ts.requestCoverage(apexNames)
}

func (ts RequesterStub) RequestApexDependencies(metadataComponentTypes []string) ([]sfapi.MetadataComponentDependency, error) {
	return ts.requestApexDependencies(metadataComponentTypes)
}

func (ts RequesterStub) RequestApexClasses(names []string) ([]sfapi.ApexClass, error) {
	return ts.requestApexClasses(names)
}
