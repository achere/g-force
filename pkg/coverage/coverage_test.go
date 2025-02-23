package coverage

import (
	"context"
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

	requestEmptyApexClasses := func(ctx context.Context, s []string) ([]sfapi.ApexClass, error) { return []sfapi.ApexClass{}, nil }

	data := []struct {
		name            string
		requestCoverage func(context.Context, []string) ([]sfapi.ApexCodeCoverage, error)
		tests           []string
		mustErr         bool
	}{
		{
			"success",
			func(ctx context.Context, apexNames []string) ([]sfapi.ApexCodeCoverage, error) { return goodCov, nil },
			[]string{"Class1_Test", "Trigger1_Test"},
			false,
		},
		{
			"missing class",
			func(ctx context.Context, apexNames []string) ([]sfapi.ApexCodeCoverage, error) { return missClass, nil },
			[]string{},
			true,
		},
		{
			"insufficient coverage",
			func(ctx context.Context, apexNames []string) ([]sfapi.ApexCodeCoverage, error) { return insuffCov, nil },
			[]string{},
			true,
		},
	}

	ctx := context.Background()
	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			ts := RequesterStub{requestCoverage: d.requestCoverage, requestApexClasses: requestEmptyApexClasses}

			tests, err := RequestTestsWithStrategy(ctx, StratMaxCoverage, ts, []string{"Class1"}, []string{"Trigger1"})

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
	requestApexDependencies := func(ctx context.Context, metadataComponentTypes []string) ([]sfapi.MetadataComponentDependency, error) {
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
	requestApexClasses := func(ctx context.Context, s []string) ([]sfapi.ApexClass, error) { return []sfapi.ApexClass{}, nil }

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

	ctx := context.Background()
	data := []struct {
		name            string
		requestCoverage func(context.Context, []string) ([]sfapi.ApexCodeCoverage, error)
		tests           []string
		mustErr         bool
	}{
		{
			"success",
			func(ctx context.Context, apexNames []string) ([]sfapi.ApexCodeCoverage, error) {
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

			tests, err := RequestTestsWithStrategy(ctx, StratMaxCoverageWithDeps, ts, []string{}, []string{"Trigger1"})

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
	requestCoverage         func(context.Context, []string) ([]sfapi.ApexCodeCoverage, error)
	requestApexDependencies func(context.Context, []string) ([]sfapi.MetadataComponentDependency, error)
	requestApexClasses      func(context.Context, []string) ([]sfapi.ApexClass, error)
}

func (ts RequesterStub) RequestCoverage(ctx context.Context, apexNames []string) ([]sfapi.ApexCodeCoverage, error) {
	return ts.requestCoverage(ctx, apexNames)
}

func (ts RequesterStub) RequestApexDependencies(ctx context.Context, metadataComponentTypes []string) ([]sfapi.MetadataComponentDependency, error) {
	return ts.requestApexDependencies(ctx, metadataComponentTypes)
}

func (ts RequesterStub) RequestApexClasses(ctx context.Context, names []string) ([]sfapi.ApexClass, error) {
	return ts.requestApexClasses(ctx, names)
}
