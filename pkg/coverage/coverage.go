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
	StratMaxCoverageWithDeps = "MaxCoverageWithDeps"
)

var strategyToGetterMap = map[string]testNamesRequester{
	StratMaxCoverage:         requestTestsMaxCoverage,
	StratMaxCoverageWithDeps: requestTestsMaxCoverageWithDeps,
}

type testNamesRequester func(sfapi.Tooling, []string, []string) ([]string, error)

func RequestTestsWithStrategy(
	strategy string,
	t sfapi.Tooling,
	classes []string,
	triggers []string,
) ([]string, error) {
	f := strategyToGetterMap[strategy]

	if f == nil {
		return []string{}, errors.New("Unsupported strategy provided: " + strategy)
	}

	return f(t, classes, triggers)
}

func requestTestsMaxCoverage(
	t sfapi.Tooling,
	classes []string,
	triggers []string,
) ([]string, error) {
	testMap, apexMap, tests, err := requestAndParseCoverage(t, slices.Concat(classes, triggers), classes)
	if err != nil {
		return []string{}, fmt.Errorf("requestAndParseCoverage: %w", err)
	}

	testNames, err := GetTestsMaxCoverage(testMap, apexMap, classes, triggers, tests)
	if err != nil {
		return []string{}, fmt.Errorf("GetTestsMaxCoverage: %w", err)
	}

	return testNames, nil
}

func requestTestsMaxCoverageWithDeps(
	t sfapi.Tooling,
	classes []string,
	triggers []string,
) ([]string, error) {
	deps, err := t.RequestApexDependencies([]string{"ApexTrigger", "ApexClass"})
	if err != nil {
		return []string{}, fmt.Errorf("t.RequestApexDependencies: %w", err)
	}
	apexDeps := ParseDependencies(deps, classes, triggers)

	testMap, apexMap, tests, err := requestAndParseCoverage(
		t,
		slices.Concat(classes, triggers, apexDeps),
		classes,
	)
	if err != nil {
		return []string{}, fmt.Errorf("requestAndParseCoverage: %w", err)
	}

	testNames, err := GetTestsMaxCoverage(testMap, apexMap, classes, triggers, tests)
	if err != nil {
		return []string{}, fmt.Errorf("GetTestsMaxCoverage: %w", err)
	}

	return testNames, nil
}

func requestAndParseCoverage(
	t sfapi.Tooling,
	apex []string,
	classes []string,
) (map[string]Test, map[string]Apex, []string, error) {
	var (
		chCov = make(chan []sfapi.ApexCodeCoverage)
		chCls = make(chan []sfapi.ApexClass)
		chErr = make(chan error)
	)
	defer close(chErr)

	go func() {
		defer close(chCov)
		coverages, err := t.RequestCoverage(apex)
		if err != nil {
			chErr <- fmt.Errorf("t.RequestCoverage: %w", err)
		} else {
			chCov <- coverages
		}
	}()

	go func() {
		defer close(chCls)
		apiClasses, err := t.RequestApexClasses(classes)
		if err != nil {
			chErr <- fmt.Errorf("t.RequestApexClasses: %w", err)
		} else {
			chCls <- apiClasses
		}
	}()

	var (
		coverages  []sfapi.ApexCodeCoverage
		apiClasses []sfapi.ApexClass
	)

loop:
	for {
		select {
		case err := <-chErr:
			return map[string]Test{}, map[string]Apex{}, []string{}, err
		case cov := <-chCov:
			coverages = append(coverages, cov...)
			if apiClasses != nil {
				break loop
			}
		case apiCls := <-chCls:
			apiClasses = append(apiClasses, apiCls...)
			if coverages != nil {
				break loop
			}
		}
	}

	testMap, apexMap := ParseCoverage(coverages)
	tests := findTestClasses(apiClasses)

	return testMap, apexMap, tests, nil
}

type Apex struct {
	Id           string
	IsTrigger    bool
	Name         string
	Lines        int
	Coverage     map[string][]bool
	LinesCovered int
	maxLine      int
}

type Test struct {
	Id           string
	Name         string
	Coverage     map[string][]bool
	LinesCovered int
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

	for apexId, apex := range apexMap {
		totalCov := make([]bool, apex.maxLine)
		for _, c := range apex.Coverage {
			totalCov = mergeCoverage(totalCov, c)
		}

		for _, l := range totalCov {
			if l {
				apex.LinesCovered++
			}
		}

		apexMap[apexId] = apex
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

func findTestClasses(classes []sfapi.ApexClass) []string {
	tests := make([]string, 0, len(classes))

outer:
	for _, c := range classes {
		for _, a := range c.SymbolTable.TableDeclaration.Annotations {
			if a.Name == "IsTest" {
				tests = append(tests, c.Name)
				continue outer
			}
		}
		for _, m := range c.SymbolTable.TableDeclaration.Modifiers {
			if m == "testMethod" {
				tests = append(tests, c.Name)
				break
			}
		}
	}

	return tests
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

	res := make([]string, 0, len(depIds))
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
	classes, triggers, tests []string,
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
		for testId := range apex.Coverage {
			test := testMap[testId]
			res = appendNoDups(res, test.Name)
		}

		var (
			isTrigger = slices.Contains(triggers, apex.Name)
			isClass   = slices.Contains(classes, apex.Name)
		)
		if isTrigger || isClass {
			linesTotal += apex.Lines
			coveredLines := apex.LinesCovered
			linesCoveredTotal += coveredLines
			coverage := math.Ceil(float64(coveredLines)/float64(apex.Lines)*100) / 100
			if isTrigger {
				triggerCoverage[apex.Name] = coverage
			} else if isClass {
				classCoverage[apex.Name] = coverage
			}
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
		if slices.Contains(tests, c) {
			continue
		}
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

func appendNoDups(ogSlice []string, item string) []string {
	m := make(map[string]bool)
	for _, v := range ogSlice {
		m[v] = true
	}

	if _, ok := m[item]; !ok {
		ogSlice = append(ogSlice, item)
	}

	return ogSlice
}
