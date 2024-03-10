package coverage

import (
	"errors"
	"fmt"
	"strings"

	"github.com/achere/g-force/pkg/sfapi"
)

type Apex struct {
	Id        string
	IsTrigger bool
	Name      string
	Lines     int
	Coverage  map[string][]bool
}

type Test struct {
	Id       string
	Name     string
	Coverage map[string][]bool
	covLines int
}

func (t *Test) GetCovLines() int {
	if tCovLines := t.covLines; tCovLines != 0 {
		return tCovLines
	}
	var res int
	for _, c := range t.Coverage {
		for _, l := range c {
			if l {
				res++
			}
		}
	}
	return res
}

func RequestTestsMaxCoverage(
	con *sfapi.Connection,
	apexClasses []string,
	apexTrigers []string,
) ([]string, error) {
	coverages, err := con.GetCoverage(append(apexClasses, apexTrigers...))
	if err != nil {
		return []string{}, fmt.Errorf("con.GetCoverage: %w", err)
	}

	testMap, apexMap := ParseCoverage(coverages)
	tests, err := GetTestsMaxCoverage(testMap, apexMap, apexClasses, apexTrigers)
	if err != nil {
		return []string{}, fmt.Errorf("GetTestsMaxCoverage: %w", err)
	}

	return tests, nil
}

func ParseCoverage(data []sfapi.ApexCodeCoverage) (map[string]Test, map[string]Apex) {
	var (
		testMap = make(map[string]Test)
		apexMap = make(map[string]Apex)
	)

	for _, c := range data {
		var (
			testName      = c.ApexTestClass.Name
			testId        = c.ApexTestClass.Id
			apexName      = c.ApexClassOrTrigger.Name
			apexId        = c.ApexClassOrTrigger.Id
			lenCovLines   = len(c.Coverage.CoveredLines)
			lenUncovLines = len(c.Coverage.UncoveredLines)
			maxCovLine    int
			maxUncovLine  int
			maxLine       int
		)
		if lenCovLines > 0 {
			maxCovLine = c.Coverage.CoveredLines[lenCovLines-1]
		}
		if lenUncovLines > 0 {
			maxUncovLine = c.Coverage.UncoveredLines[lenUncovLines-1]
		}

		if maxCovLine > maxUncovLine {
			maxLine = maxCovLine
		} else {
			maxLine = maxUncovLine
		}

		cov := make([]bool, maxLine)
		for _, lineNum := range c.Coverage.CoveredLines {
			cov[lineNum-1] = true
		}

		apex, ok := apexMap[apexId]
		if !ok {
			apex = Apex{
				Id:        apexId,
				IsTrigger: c.ApexClassOrTrigger.Attributes.Type == "ApexTrigger",
				Name:      apexName,
			}
			apex.Lines = lenCovLines + lenUncovLines

			covMap := make(map[string][]bool)
			apex.Coverage = covMap
		}

		_, ok = apex.Coverage[testId]
		if !ok {
			apex.Coverage[testId] = make([]bool, maxLine)
		}
		apex.Coverage[testId] = mergeCoverage(apex.Coverage[testId], cov)
		apexMap[apexId] = apex

		test, ok := testMap[testId]
		if !ok {
			test = Test{Id: testId, Name: testName}

			covMap := make(map[string][]bool)
			test.Coverage = covMap
		}

		_, ok = test.Coverage[apexId]
		if !ok {
			test.Coverage[apexId] = make([]bool, maxLine)
		}
		test.Coverage[apexId] = mergeCoverage(test.Coverage[apexId], cov)

		testMap[testId] = test
	}

	return testMap, apexMap
}

func mergeCoverage(cov1, cov2 []bool) []bool {
	res := make([]bool, len(cov1))
	for i, l1 := range cov1 {
		res[i] = l1 || cov2[i]
	}
	return res
}

func GetTestsMaxCoverage(
	testMap map[string]Test,
	apexMap map[string]Apex,
	apexClasses []string,
	apexTriggers []string,
) ([]string, error) {
	var (
		res              []string
		triggerSet       = make(map[string]bool)
		triggerTestedSet = make(map[string]bool)
	)

	for _, v := range apexTriggers {
		triggerSet[v] = true
	}

	var linesTotal, linesCovered int
	for _, apex := range apexMap {
		var (
			isClass   = contains(apexClasses, apex.Name)
			isTrigger = contains(apexTriggers, apex.Name)
		)
		if isClass || isTrigger {
			linesTotal += apex.Lines
			if isTrigger {
				triggerTestedSet[apex.Name] = true
			}

			for testId := range apex.Coverage {
				test := testMap[testId]
				res = append(res, test.Name)

				linesCovered += test.GetCovLines()
			}
		}
	}

	if len(triggerTestedSet) < len(triggerSet) {
		untestedTriggers := make([]string, 0)
		for t := range triggerSet {
			if !triggerTestedSet[t] {
				untestedTriggers = append(untestedTriggers, t)
			}
			msg := "untested triggers: " + strings.Join(untestedTriggers, ", ")
			return []string{}, errors.New(msg)
		}
	}
	if float64(linesCovered)/float64(linesTotal) < 0.75 {
		return []string{}, errors.New("coverage less than 75%")
	}

	return res, nil
}

func contains[T comparable](elems []T, v T) bool {
	for _, s := range elems {
		if v == s {
			return true
		}
	}
	return false
}
