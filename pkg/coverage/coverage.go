package coverage

import (
	"errors"
	"fmt"
	"math"

	"github.com/achere/g-force/pkg/sfapi"
)

type Apex struct {
	Id           string
	IsTrigger    bool
	Name         string
	Lines        int
	Coverage     map[string][]bool
	linesCovered int
	maxLine      int
}

func (a *Apex) GetCovLines() int {
	if aCovLines := a.linesCovered; aCovLines != 0 {
		return aCovLines
	}
	totalCov := make([]bool, a.maxLine)
	for _, c := range a.Coverage {
		totalCov = mergeCoverage(totalCov, c)
	}

	for _, l := range totalCov {
		if l {
			a.linesCovered++
		}
	}
	return a.linesCovered
}

type Test struct {
	Id           string
	Name         string
	Coverage     map[string][]bool
	linesCovered int
}

func (t *Test) GetCovLines() int {
	if tCovLines := t.linesCovered; tCovLines != 0 {
		return tCovLines
	}
	for _, c := range t.Coverage {
		for _, l := range c {
			if l {
				t.linesCovered++
			}
		}
	}
	return t.linesCovered
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
				maxLine:   maxLine,
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
		res             []string
		classSet        = make(map[string]bool)
		classCoverage   = make(map[string]float64)
		triggerSet      = make(map[string]bool)
		triggerCoverage = make(map[string]float64)
	)

	for _, v := range apexTriggers {
		triggerSet[v] = true
	}
	for _, v := range apexClasses {
		classSet[v] = true
	}

	var linesTotal, linesCoveredTotal int
	for _, apex := range apexMap {
		linesTotal += apex.Lines
		isTrigger := contains(apexTriggers, apex.Name)

		coveredLines := apex.GetCovLines()
		linesCoveredTotal += coveredLines
		for testId := range apex.Coverage {
			test := testMap[testId]
			res = append(res, test.Name)
		}

		coverage := math.Ceil(float64(coveredLines)/float64(apex.Lines)*100) / 100
		if isTrigger {
			triggerCoverage[apex.Name] = coverage
		} else {
			classCoverage[apex.Name] = coverage
		}
	}

	var errorMsg string
	for t := range triggerSet {
		coverage, ok := triggerCoverage[t]
		if !ok {
			errorMsg += "untested trigger " + t + "\n"
		}
		if coverage < 0.75 {
			errorMsg += "coverage of trigger " + t + " is less than 75%: " + fmt.Sprintf("%.2f%%", coverage*100) + "\n"
		}
	}
	for c := range classSet {
		coverage, ok := classCoverage[c]
		if !ok {
			errorMsg += "untested class " + c + "\n"
		}
		if coverage < 0.75 {
			errorMsg += "coverage of class " + c + " is less than 75%: " + fmt.Sprintf("%.2f%%", coverage*100) + "\n"
		}
	}

	if len(errorMsg) > 0 {
		return []string{}, errors.New(errorMsg)
	}

	totalCov := math.Ceil(float64(linesCoveredTotal)/float64(linesTotal)*100) / 100
	if totalCov < 0.75 {
		return []string{}, errors.New("total coverage is less than 75%: " + fmt.Sprintf("%.2f%%", totalCov*100))
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
