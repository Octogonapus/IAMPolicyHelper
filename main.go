package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"code.rocketnine.space/tslocum/cview"
	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly"
	"github.com/lithammer/fuzzysearch/fuzzy"
	"github.com/olekukonko/tablewriter"
)

type ResourceTypeReference struct {
	Name     string
	Required bool
}

type Action struct {
	Name                   string
	Description            string
	AccessLevel            string
	ResourceTypeReferences []*ResourceTypeReference
	ConditionKeys          []string
	DependentActions       []string
}

type ResourceType struct {
	Name          string
	ARN           string
	ConditionKeys []string
}

type ConditionKey struct {
	Name        string
	Description string
	Type        string
}

type Service struct {
	URL           string
	Name          string
	Prefix        string
	Actions       []*Action
	ResourceTypes []*ResourceType
	ConditionKeys []*ConditionKey
}

type Cell struct {
	Rowspan int
	Colspan int
	Text    string
}

type ServiceCells struct {
	Name               string
	Prefix             string
	ActionsCells       [][]Cell
	ResourcesCells     [][]Cell
	ConditionKeysCells [][]Cell
}

const VERSION_TAG = "v0.1.1"
const PROJECT_DIR = ".iampolicyhelper"
const RAW_DATA_PATH = "rawData.json"
const VERSION_PATH = "version.txt"

func main() {
	err := maybeCrawl(getProjectDir())
	if err != nil {
		panic(err)
	}

	services, err := loadRawData(getProjectDir())
	if err != nil {
		panic(err)
	}

	actionNames := buildActionNames(services)

	app := cview.NewApplication()
	app.EnableMouse(true)

	textView := cview.NewTextView()
	textView.SetDynamicColors(true)
	textView.SetRegions(true)
	textView.SetMaxLines(0)
	textView.SetScrollBarVisibility(cview.ScrollBarAuto)
	textView.SetChangedFunc(func() {
		app.Draw()
	})

	inputField := cview.NewInputField()
	inputField.SetFieldWidth(0)
	inputField.SetChangedFunc(func(text string) {
		matches := stringWithBestMatch(text, actionNames)
		if len(matches) > 0 {
			service, actions := lookupByFullActionName(matches[0].Target, services)
			action := mergeActions(actions)
			if service != nil {
				message := renderBody(action, service)
				textView.SetText(message)
				textView.ScrollToBeginning()
			} else {
				textView.SetText("No match")
			}
		} else {
			textView.SetText("No match")
		}
	})

	flex := cview.NewFlex()
	flex.SetDirection(cview.FlexRow)
	flex.AddItem(inputField, 1, 0, true)
	flex.AddItem(textView, 0, 1, false)

	app.SetRoot(flex, true)
	if err := app.Run(); err != nil {
		panic(err)
	}
}

func eachResourceType(service *Service, action *Action, f func(*ResourceType)) {
	for _, actionResourceTypeName := range action.ResourceTypeReferences {
		for _, resourceType := range service.ResourceTypes {
			if actionResourceTypeName.Name == resourceType.Name {
				f(resourceType)
			}
		}
	}
}

func eachConditionKey(service *Service, conditionKeyNames []string, f func(*ConditionKey)) {
	for _, relevantConditionKeyName := range conditionKeyNames {
		for _, conditionKey := range service.ConditionKeys {
			if relevantConditionKeyName == conditionKey.Name {
				f(conditionKey)
			}
		}
	}
}

func unique(array []string) []string {
	occurred := map[string]bool{}
	out := []string{}
	for _, it := range array {
		if !occurred[it] {
			occurred[it] = true
			out = append(out, it)
		}
	}
	return out
}

func renderBody(action *Action, service *Service) string {
	resouceTypesString := joinResourceTypeReferences(action.ResourceTypeReferences)
	conditionKeysString := joinConditionKeys(unique(action.ConditionKeys))
	message := fmt.Sprintf(
		`[::b]Service:[::-] %s
[::b]Action[::-]: %s
[::b]Description[::-]: %s
[::b]Access Level[::-]: %s
[::b]Resource Types[::-]: %s
[::b]Condition Keys[::-]: %s`,
		service.Name,
		fmt.Sprintf("%s:%s", service.Prefix, action.Name),
		action.Description,
		action.AccessLevel,
		resouceTypesString,
		conditionKeysString,
	)

	if len(action.ResourceTypeReferences) > 0 {
		tableString := &strings.Builder{}
		table := tablewriter.NewWriter(tableString)
		table.SetHeader([]string{"Resource Type", "ARN", "Condition Keys"})
		table.SetRowLine(true)
		table.SetRowSeparator("-")
		table.SetColWidth(100)
		eachResourceType(service, action, func(resourceType *ResourceType) {
			table.Append([]string{
				resourceType.Name,
				breakString(resourceType.ARN, 80),
				joinConditionKeys(resourceType.ConditionKeys),
			})
		})
		if table.NumLines() > 0 {
			table.Render()
			message += fmt.Sprintf("\n\n[::b]Relevant Resource Types[::-]\n%s", tableString)
		}
	}

	relevantConditionKeyNames := action.ConditionKeys
	eachResourceType(service, action, func(resourceType *ResourceType) {
		relevantConditionKeyNames = append(relevantConditionKeyNames, resourceType.ConditionKeys...)
	})
	relevantConditionKeyNames = unique(relevantConditionKeyNames)
	if len(relevantConditionKeyNames) > 0 {
		tableString := &strings.Builder{}
		table := tablewriter.NewWriter(tableString)
		table.SetHeader([]string{"Condition Key", "Description", "Type"})
		table.SetRowLine(true)
		table.SetRowSeparator("-")
		table.SetColWidth(100)
		eachConditionKey(service, relevantConditionKeyNames, func(conditionKey *ConditionKey) {
			table.Append([]string{
				conditionKey.Name,
				conditionKey.Description,
				conditionKey.Type,
			})
		})
		if table.NumLines() > 0 {
			table.Render()
			message += fmt.Sprintf("\n\n[::b]Relevant Condition Keys[::-]\n%s", tableString)
		}
	}

	// Add colors. Colors for text in a table must be done after the table, because the table generator doesn't account
	// for the color codes not getting rendered, so the column widths will be wrong.

	// service prefix (and service name)
	re := regexp.MustCompile(fmt.Sprintf("(%s):", service.Name))
	message = re.ReplaceAllString(message, "[#00ff80]$1[white]:")
	re = regexp.MustCompile(fmt.Sprintf("(%s):", service.Prefix))
	message = re.ReplaceAllString(message, "[#00ff80]$1[white]:")

	// aws prefix
	re = regexp.MustCompile("aws:")
	message = re.ReplaceAllString(message, "[#ffdf00]aws[white]:")

	// condition keys
	for _, it := range relevantConditionKeyNames {
		parts := strings.Split(it, ":") // we don't want to highlight the prefix, only the condition key name itself
		re = regexp.MustCompile(fmt.Sprintf("(%s)([ ,\n])", regexp.QuoteMeta(parts[1])))
		message = re.ReplaceAllString(message, "[#4000ff]$1[white]$2")
	}

	return message
}

func joinResourceTypeReferences(resourceTypeReferences []*ResourceTypeReference) string {
	resouceTypesString := ""
	for i, it := range resourceTypeReferences {
		if it.Required {
			resouceTypesString += fmt.Sprintf("%s (required)", it.Name)
		} else {
			resouceTypesString += it.Name
		}
		if i < len(resourceTypeReferences)-1 {
			resouceTypesString += ", "
		}
	}
	return resouceTypesString
}

func joinConditionKeys(conditionKeys []string) string {
	conditionKeysString := ""
	for i, it := range conditionKeys {
		conditionKeysString += it
		if i < len(conditionKeys)-1 {
			conditionKeysString += ", "
		}
	}
	return conditionKeysString
}

func breakString(s string, lim int) string {
	if len(s) > lim {
		lines := []string{}
		prev := 0
		next := 0
		for {
			next += lim
			if next >= len(s) {
				next = len(s) - 1
			}

			line := s[prev:next]
			lines = append(lines, line)
			prev = next
			if next == len(s)-1 {
				break
			}
		}

		out := ""
		for _, line := range lines {
			out += fmt.Sprintf("%s\n", line)
		}
		return out
	} else {
		return s
	}
}

func lookupByFullActionName(fullActionName string, services []*Service) (*Service, []*Action) {
	parts := strings.Split(fullActionName, ":")
	prefix := parts[0]
	actionName := parts[1]
	for _, service := range services {
		if service.Prefix == prefix {
			actions := make([]*Action, 0)
			for _, action := range service.Actions {
				if strings.ToLower(action.Name) == actionName {
					actions = append(actions, action)
				}
			}
			return service, actions
		}
	}
	return nil, nil
}

func mergeActions(actions []*Action) *Action {
	if len(actions) == 0 {
		return nil
	}

	action := &Action{
		Name:                   actions[0].Name,
		Description:            actions[0].Description,
		AccessLevel:            actions[0].AccessLevel,
		ResourceTypeReferences: make([]*ResourceTypeReference, len(actions[0].ResourceTypeReferences)),
		ConditionKeys:          make([]string, len(actions[0].ConditionKeys)),
		DependentActions:       make([]string, len(actions[0].DependentActions)),
	}
	copy(action.ResourceTypeReferences, actions[0].ResourceTypeReferences)
	copy(action.ConditionKeys, actions[0].ConditionKeys)
	copy(action.DependentActions, actions[0].DependentActions)
	for i := 1; i < len(actions); i++ {
		action.ResourceTypeReferences = append(action.ResourceTypeReferences, actions[i].ResourceTypeReferences...)
		action.ConditionKeys = append(action.ConditionKeys, actions[i].ConditionKeys...)
		action.DependentActions = append(action.DependentActions, actions[i].DependentActions...)
	}
	return action
}

func buildActionNames(services []*Service) []string {
	fullActionNames := make([]string, 0)
	for _, service := range services {
		for _, action := range service.Actions {
			fullActionName := fmt.Sprintf("%s:%s", service.Prefix, action.Name)
			fullActionName = strings.ToLower(fullActionName)
			fullActionNames = append(fullActionNames, fullActionName)
		}
	}
	return fullActionNames
}

func stringWithBestMatch(filter string, allStrings []string) fuzzy.Ranks {
	matches := fuzzy.RankFind(filter, allStrings)

	matchesWithPrefix := fuzzy.Ranks{}
	for i := len(matches) - 1; i >= 0; i-- {
		if strings.HasPrefix(matches[i].Target, filter) {
			matchesWithPrefix = append(matchesWithPrefix, matches[i])
		}
	}
	if len(matchesWithPrefix) > 0 {
		sort.Sort(matchesWithPrefix)
		return matchesWithPrefix
	}

	sort.Sort(matches)
	return matches
}

func loadRawData(projectDir string) ([]*Service, error) {
	path := filepath.Join(projectDir, RAW_DATA_PATH)
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var data []*Service
	err = json.Unmarshal(body, &data)
	return data, err
}

// TODO add cmdline flag to recrawl
func maybeCrawl(projectDir string) error {
	shouldCrawl := false

	// If the raw data file does not exist, we should crawl
	rawDataPath := filepath.Join(projectDir, RAW_DATA_PATH)
	f, err := os.Open(rawDataPath)
	if os.IsNotExist(err) {
		shouldCrawl = true
	} else if err != nil {
		return err
	}
	defer f.Close()

	// If the version file does not exist or has the wrong version in it, we should crawl
	versionPath := filepath.Join(projectDir, VERSION_PATH)
	version, err := os.ReadFile(versionPath)
	if os.IsNotExist(err) {
		shouldCrawl = true
	} else if err != nil {
		return err
	}
	shouldCrawl = shouldCrawl || strings.Trim(string(version), " \n") != VERSION_TAG

	if shouldCrawl {
		data, err := crawl()
		if err != nil {
			return err
		}

		err = saveCrawl(data, rawDataPath)
		if err != nil {
			return err
		}

		err = saveVersion(VERSION_TAG, projectDir)
		if err != nil {
			return err
		}
	}

	return nil
}

func saveCrawl(rawData []*Service, path string) error {
	jsonData, err := json.Marshal(rawData)
	if err != nil {
		return err
	}

	err = os.MkdirAll(filepath.Dir(path), 0777)
	if err != nil && !os.IsExist(err) {
		return err
	}
	rawDataFile, err := os.Create(path)
	if err != nil {
		return err
	}
	defer rawDataFile.Close()

	_, err = rawDataFile.Write(jsonData)
	return err
}

func saveVersion(versionTag string, projectDir string) error {
	path := filepath.Join(projectDir, VERSION_PATH)
	err := os.MkdirAll(filepath.Dir(path), 0777)
	if err != nil && !os.IsExist(err) {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write([]byte(versionTag))
	return err
}

func crawl() ([]*Service, error) {
	c := colly.NewCollector(
		colly.MaxDepth(2),
		colly.Async(true),
	)

	c.Limit(&colly.LimitRule{Parallelism: 2})

	// URL : Service
	serviceDataMutex := sync.Mutex{}
	serviceData := make(map[string]*ServiceCells)

	c.OnRequest(func(r *colly.Request) {
		url := r.AbsoluteURL(r.URL.String())
		serviceDataMutex.Lock()
		defer serviceDataMutex.Unlock()
		serviceData[url] = &ServiceCells{}
	})

	c.OnHTML(".highlights ul li a[href]", func(e *colly.HTMLElement) {
		url := e.Request.AbsoluteURL(e.Attr("href"))
		e.Request.Visit(url)
	})

	c.OnHTML("#main-content p", func(h *colly.HTMLElement) {
		if strings.Contains(h.Text, "service prefix") {
			url := h.Request.AbsoluteURL(h.Request.URL.String())
			serviceDataMutex.Lock()
			defer serviceDataMutex.Unlock()
			serviceCells := serviceData[url]
			serviceCells.Name = strings.Trim(strings.Split(h.Text, "(")[0], " ")
			serviceCells.Prefix = h.ChildText("code")
		}
	})

	c.OnHTML(".table-container", func(e *colly.HTMLElement) {
		url := e.Request.AbsoluteURL(e.Request.URL.String())
		serviceDataMutex.Lock()
		defer serviceDataMutex.Unlock()
		serviceCells := serviceData[url]
		headerText := strings.ToLower(e.ChildText("table tr th"))
		if strings.HasPrefix(headerText, "actions") {
			e.ForEach("table tbody tr", func(i int, h *colly.HTMLElement) {
				rows := crawlTableRows(h)
				serviceCells.ActionsCells = append(serviceCells.ActionsCells, rows)
			})
		} else if strings.HasPrefix(headerText, "resource types") {
			e.ForEach("table tbody tr", func(i int, h *colly.HTMLElement) {
				rows := crawlTableRows(h)
				serviceCells.ResourcesCells = append(serviceCells.ResourcesCells, rows)
			})
		} else if strings.HasPrefix(headerText, "condition keys") {
			e.ForEach("table tbody tr", func(i int, h *colly.HTMLElement) {
				rows := crawlTableRows(h)
				serviceCells.ConditionKeysCells = append(serviceCells.ConditionKeysCells, rows)
			})
		}
	})

	err := c.Visit("https://docs.aws.amazon.com/service-authorization/latest/reference/reference_policies_actions-resources-contextkeys.html")
	if err != nil {
		return nil, err
	}

	c.Wait()

	services := make([]*Service, 0)
	for url, serviceCells := range serviceData {
		if len(removeSpace(url)) == 0 {
			continue
		}

		actionTable := htmlTableTo2D(serviceCells.ActionsCells)
		resourcesTable := htmlTableTo2D(serviceCells.ResourcesCells)
		conditionKeysTable := htmlTableTo2D(serviceCells.ConditionKeysCells)

		actions := actionsFromTable(actionTable)
		resources := resourcesFromTable(resourcesTable)
		conditionKeys := conditionKeysFromTable(conditionKeysTable)

		service := &Service{
			URL:           url,
			Name:          serviceCells.Name,
			Prefix:        serviceCells.Prefix,
			Actions:       actions,
			ResourceTypes: resources,
			ConditionKeys: conditionKeys,
		}
		services = append(services, service)
	}

	sort.Slice(services, func(i, j int) bool {
		return services[i].Name < services[j].Name
	})

	return services, nil
}

func crawlTableRows(h *colly.HTMLElement) []Cell {
	rows := make([]Cell, 0)

	h.DOM.ChildrenFiltered("td").Each(func(i int, s *goquery.Selection) {
		rowspanStr, ok := s.Attr("rowspan")
		rowspan := 1
		if ok {
			var err error
			rowspan, err = strconv.Atoi(rowspanStr)
			if err != nil {
				panic(err)
			}
		}

		colspanStr, ok := s.Attr("colspan")
		colspan := 1
		if ok {
			var err error
			colspan, err = strconv.Atoi(colspanStr)
			if err != nil {
				panic(err)
			}
		}

		rows = append(rows, Cell{Rowspan: rowspan, Colspan: colspan, Text: s.Text()})
	})

	return rows
}

// Removes leading and trailing whitespace.
// Removes empty strings.
func cleanupHTMLStringList(ss []string) []string {
	out := make([]string, 0)
	for _, s := range ss {
		s = removeSpace(s)
		if len(s) > 0 {
			out = append(out, s)
		}
	}
	return out
}

func actionsFromTable(table [][]string) []*Action {
	actions := make([]*Action, len(table))
	for rowI, row := range table {
		resourceTypeStrings := cleanupHTMLStringList(strings.Split(row[3], "\n"))
		resourceTypes := make([]*ResourceTypeReference, 0)
		for _, it := range resourceTypeStrings {
			parts := strings.Split(it, "*")
			name := parts[0]
			required := false
			if len(parts) > 1 {
				required = true
			}
			resourceTypes = append(resourceTypes, &ResourceTypeReference{
				Name:     name,
				Required: required,
			})
		}
		action := &Action{
			Name:                   strings.Trim(row[0], " \n\t"),
			Description:            strings.Trim(row[1], " \n\t"),
			AccessLevel:            strings.Trim(row[2], " \n\t"),
			ResourceTypeReferences: resourceTypes,
			ConditionKeys:          cleanupHTMLStringList(strings.Split(row[4], "\n")),
			DependentActions:       cleanupHTMLStringList(strings.Split(row[5], "\n")),
		}
		actions[rowI] = action
	}
	return actions
}

func resourcesFromTable(table [][]string) []*ResourceType {
	resources := make([]*ResourceType, len(table))
	for rowI, row := range table {
		resource := &ResourceType{
			Name:          strings.Trim(row[0], " \n\t"),
			ARN:           strings.Trim(row[1], " \n\t"),
			ConditionKeys: cleanupHTMLStringList(strings.Split(row[2], "\n")),
		}
		resources[rowI] = resource
	}
	return resources
}

func conditionKeysFromTable(table [][]string) []*ConditionKey {
	conditionKeys := make([]*ConditionKey, len(table))
	for rowI, row := range table {
		conditionKey := &ConditionKey{
			Name:        strings.Trim(row[0], " \n\t"),
			Description: strings.Trim(row[1], " \n\t"),
			Type:        strings.Trim(row[2], " \n\t"),
		}
		conditionKeys[rowI] = conditionKey
	}
	return conditionKeys
}

// Converts a sparse HTML table represented by Cells to a dense table of strings
// This is a Go rewrite of the Python solution here https://stackoverflow.com/questions/48393253/how-to-parse-table-with-rowspan-and-colspan
func htmlTableTo2D(rows [][]Cell) [][]string {
	rowspans := make([]int, 0) // track pending rowspans
	rowcount := len(rows)

	// first scan, see how many columns we need
	colcount := 0
	for rowI, row := range rows {
		// count columns (including spanned).
		// add active rowspans from preceding rows
		// we *ignore* the colspan value on the last cell, to prevent
		// creating 'phantom' columns with no actual cells, only extended
		// colspans. This is achieved by hardcoding the last cell width as 1.
		// a colspan of 0 means “fill until the end” but can really only apply
		// to the last cell; ignore it elsewhere.
		colspans := make([]int, len(row)-1)
		for i, cell := range row {
			if i == len(row)-1 {
				// skip the last element
				break
			}
			colspans[i] = cell.Colspan
		}

		colspanSum := 0
		for _, x := range colspans {
			colspanSum += x
		}
		colspanSum += 1
		colspanSum += len(rowspans)

		colcount = int(math.Max(float64(colcount), float64(colspanSum)))

		// update rowspan bookkeeping; 0 is a span to the bottom.
		theseRowspans := make([]int, len(row))
		for i, cell := range row {
			if cell.Rowspan == 0 {
				theseRowspans[i] = rowcount - rowI
			} else {
				theseRowspans[i] = cell.Rowspan
			}
		}
		rowspans = append(rowspans, theseRowspans...)

		newRowspans := make([]int, 0)
		for _, it := range rowspans {
			if it > 1 {
				newRowspans = append(newRowspans, it-1)
			}
		}

		rowspans = newRowspans
	}

	// it doesn't matter if there are still rowspan numbers 'active'; no extra
	// rows to show in the table means the larger than 1 rowspan numbers in the
	// last table row are ignored.

	// build an empty matrix for all possible cells
	table := make([][]string, rowcount)
	for i := range table {
		table[i] = make([]string, colcount)
	}

	// fill matrix from row data
	rowspansMap := make(map[int]int) // track pending rowspans, column number mapping to count
	for rowI, row := range rows {
		spanOffset := 0 // how many columns are skipped due to row and colspans
		for colI, cell := range row {
			// adjust for preceding row and colspans
			colI += spanOffset
			for {
				cond, ok := rowspansMap[colI]
				if !ok || cond == 0 {
					break
				}
				spanOffset += 1
				colI += 1
			}

			// fill table data
			rowspan := cell.Rowspan
			if rowspan == 0 {
				rowspan = rowcount - rowI
			}
			rowspansMap[colI] = rowspan

			colspan := cell.Colspan
			if colspan == 0 {
				colspan = colcount - colI
			}

			// next column is offset by the colspan
			spanOffset += colspan - 1

			for drow := 0; drow < rowspan; drow++ {
				for dcol := 0; dcol < colspan; dcol++ {
					testrow := rowI + drow
					testcol := colI + dcol
					if testrow >= 0 && testrow < rowcount && testcol >= 0 && testcol < colcount {
						table[testrow][testcol] = cell.Text
						rowspansMap[testcol] = rowspan
					}
				}
			}
		}

		// update rowspan bookkeeping
		newRowspansMap := make(map[int]int)
		for c, s := range rowspansMap {
			if s > 1 {
				newRowspansMap[c] = s - 1
			}
		}
		rowspansMap = newRowspansMap
	}

	return table
}

func removeSpace(s string) string {
	rr := make([]rune, 0, len(s))
	for _, r := range s {
		if !unicode.IsSpace(r) {
			rr = append(rr, r)
		}
	}
	return string(rr)
}

func getProjectDir() string {
	homedir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	return path.Join(homedir, PROJECT_DIR)
}
