package coverage

import (
	"errors"
	"fmt"
	"math"
	"slices"

	"github.com/achere/g-force/pkg/sfapi"
)

const (
	StratMaxCoverage         = "MaxCoverage"
	StratMaxCoverageWithDeps = "MaxCoverageWithDependencies"
)

var strategyToGetterMap = map[string]testNamesRequester{
	StratMaxCoverage:         RequestTestsMaxCoverage,
	StratMaxCoverageWithDeps: RequestTestsMaxCoverageWithDeps,
}

type testNamesRequester func(*sfapi.Connection, []string, []string) ([]string, error)

func RequestTestsWithStrategy(
	strategy string,
	con *sfapi.Connection,
	classes []string,
	triggers []string,
) ([]string, error) {
	f := strategyToGetterMap[strategy]

	if f == nil {
		return []string{}, errors.New("Unsupported strategy provided: " + strategy)
	}

	return f(con, classes, triggers)
}

func RequestTestsMaxCoverage(
	con *sfapi.Connection,
	classes []string,
	triggers []string,
) ([]string, error) {
	coverages, err := con.GetCoverage(slices.Concat(classes, triggers))
	if err != nil {
		return []string{}, fmt.Errorf("con.GetCoverage: %w", err)
	}

	testMap, apexMap := ParseCoverage(coverages)
	tests, err := GetTestsMaxCoverage(testMap, apexMap, classes, triggers)
	if err != nil {
		return []string{}, fmt.Errorf("GetTestsMaxCoverage: %w", err)
	}

	return tests, nil
}

func RequestTestsMaxCoverageWithDeps(
	con *sfapi.Connection,
	classes []string,
	triggers []string,
) ([]string, error) {
	deps, err := con.GetApexDependencies([]string{"ApexTrigger", "ApexClass"})
	if err != nil {
		return []string{}, fmt.Errorf("con.GetCoverage: %w", err)
	}

	apexDeps := ParseDependencies(deps, classes, triggers)

	coverages, err := con.GetCoverage(
		slices.Concat(classes, triggers, apexDeps),
	)
	if err != nil {
		return []string{}, fmt.Errorf("con.GetCoverage: %w", err)
	}

	testMap, apexMap := ParseCoverage(coverages)

	tests, err := GetTestsMaxCoverage(testMap, apexMap, classes, triggers)
	if err != nil {
		return []string{}, fmt.Errorf("GetTestsMaxCoverage: %w", err)
	}

	return tests, nil
}

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

func ParseDependencies(
	mcd []sfapi.MetadataComponentDependency,
	classes, triggers []string,
) []string {
	type Node struct {
		value  Apex
		isRoot bool
		deps   []string
	}

	depMap := make(map[string]Node)
	for _, d := range mcd {
		_, ok := depMap[d.RefId]
		if !ok {
			apexRef := Apex{
				Id:        d.RefId,
				IsTrigger: d.RefType == "ApexTrigger",
				Name:      d.RefName,
			}
			nodeRef := Node{value: apexRef}
			depMap[d.RefId] = nodeRef
		}

		node, ok := depMap[d.Id]
		if !ok {
			isTrigger := d.Type == "ApexTrigger"
			apex := Apex{
				Id:        d.Id,
				IsTrigger: isTrigger,
				Name:      d.Name,
			}
			var (
				isRootTrigger = isTrigger && slices.Contains(triggers, d.Name)
				isRootClass   = !isTrigger && slices.Contains(classes, d.Name)
			)
			node = Node{
				value:  apex,
				isRoot: isRootTrigger || isRootClass,
				deps:   []string{d.RefId},
			}
		} else {
			node.deps = append(node.deps, d.RefId)
		}
		depMap[d.Id] = node
	}

	var walk func(string, *[]string)
	walk = func(id string, seen *[]string) {
		if slices.Contains(*seen, id) {
			return
		}
		*seen = append(*seen, id)

		node := depMap[id]
		if len(node.deps) == 0 {
			return
		}

		for _, v := range node.deps {
			walk(v, seen)
		}
	}

	depIds := make([]string, 0)
	for id, node := range depMap {
		if !node.isRoot {
			continue
		}
		walk(id, &depIds)
	}

	res := make([]string, len(depIds))
	for _, id := range depIds {
		node := depMap[id]
		if node.isRoot {
			continue
		}
		res = append(res, node.value.Name)
	}
	return res
}

func GetTestsMaxCoverage(
	testMap map[string]Test,
	apexMap map[string]Apex,
	classes, triggers []string,
) ([]string, error) {
	var (
		res             []string
		classSet        = make(map[string]bool)
		classCoverage   = make(map[string]float64)
		triggerSet      = make(map[string]bool)
		triggerCoverage = make(map[string]float64)
	)

	for _, v := range triggers {
		triggerSet[v] = true
	}
	for _, v := range classes {
		classSet[v] = true
	}

	var linesTotal, linesCoveredTotal int
	for _, apex := range apexMap {
		linesTotal += apex.Lines

		coveredLines := apex.GetCovLines()
		linesCoveredTotal += coveredLines
		for testId := range apex.Coverage {
			test := testMap[testId]
			res = append(res, test.Name)
		}

		coverage := math.Ceil(float64(coveredLines)/float64(apex.Lines)*100) / 100
		if slices.Contains(triggers, apex.Name) {
			triggerCoverage[apex.Name] = coverage
		} else if slices.Contains(classes, apex.Name) {
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
