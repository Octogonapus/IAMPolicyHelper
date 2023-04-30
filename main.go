package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly"
	"github.com/rivo/tview"
)

type Action struct {
	Name             string
	Description      string
	AccessLevel      string
	ResourceTypes    []string
	ConditionKeys    []string
	DependentActions []string
}

type Resource struct {
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
	Resources     []*Resource
	ConditionKeys []*ConditionKey
}

type Cell struct {
	rowspan int
	colspan int
	text    string
}

type ServiceCells struct {
	name               string
	prefix             string
	actionsCells       [][]Cell
	resourcesCells     [][]Cell
	conditionKeysCells [][]Cell
}

func main() {
	// app := tview.NewApplication()

	// inputField := tview.NewInputField().
	// 	SetFieldWidth(100).
	// 	SetDoneFunc(func(key tcell.Key) {
	// 		app.Stop()
	// 	})

	// textView := tview.NewTextView().
	// 	SetDynamicColors(true).
	// 	SetRegions(true).
	// 	SetChangedFunc(func() {
	// 		app.Draw()
	// 	})
	// go mainTextRenderLoop(textView)

	// flex := tview.NewFlex().
	// 	AddItem(inputField, 0, 1, true).
	// 	AddItem(textView, 0, 1, false).
	// 	SetDirection(tview.FlexRow)

	// if err := app.SetRoot(flex, true).Run(); err != nil {
	// 	panic(err)
	// }

	services, err := loadRawData(getRawDataPath())
	if err != nil {
		panic(err)
	}
	for _, x := range services {
		fmt.Printf("%+v\n", *x)
	}

	fullActionNames := make([]string, 0)
	for _, service := range services {
		for _, action := range service.Actions {
			fullActionName := fmt.Sprintf("%s:%s", service.Prefix, action.Name)
			fullActionName = strings.ToLower(fullActionName)
			fullActionNames = append(fullActionNames, fullActionName)
		}
	}
	fmt.Println(fullActionNames)
}

func mainTextRenderLoop(textView *tview.TextView) {
	for {
		textView.Clear()
		textView.SetText("Done!")
		time.Sleep(10 * time.Millisecond)
	}
}

func loadRawData(path string) ([]*Service, error) {
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
func maybeCrawl(path string) error {
	_, err := os.Open(path)
	if os.IsNotExist(err) {
		data, err := crawl()
		if err != nil {
			return err
		}

		err = saveCrawl(data, path)
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

func crawl() ([]*Service, error) {
	c := colly.NewCollector(
		colly.MaxDepth(2),
		colly.Async(true),
	)

	c.Limit(&colly.LimitRule{Parallelism: 2})

	// URL : Service
	serviceDataMutex := sync.Mutex{}
	serviceData := make(map[string]*ServiceCells)

	getServiceCells := func(url string) *ServiceCells {
		serviceDataMutex.Lock()
		defer serviceDataMutex.Unlock()
		return serviceData[url]
	}

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
			serviceCells := getServiceCells(url)
			serviceCells.name = strings.Trim(strings.Split(h.Text, "(")[0], " ")
			serviceCells.prefix = h.ChildText("code")
		}
	})

	c.OnHTML(".table-container", func(e *colly.HTMLElement) {
		url := e.Request.AbsoluteURL(e.Request.URL.String())
		serviceCells := getServiceCells(url)
		headerText := strings.ToLower(e.ChildText("table tr th"))
		if strings.HasPrefix(headerText, "actions") {
			e.ForEach("table tbody tr", func(i int, h *colly.HTMLElement) {
				rows := crawlTableRows(h)
				serviceCells.actionsCells = append(serviceCells.actionsCells, rows)
			})
		} else if strings.HasPrefix(headerText, "resource types") {
			e.ForEach("table tbody tr", func(i int, h *colly.HTMLElement) {
				rows := crawlTableRows(h)
				serviceCells.resourcesCells = append(serviceCells.resourcesCells, rows)
			})
		} else if strings.HasPrefix(headerText, "condition keys") {
			e.ForEach("table tbody tr", func(i int, h *colly.HTMLElement) {
				rows := crawlTableRows(h)
				serviceCells.conditionKeysCells = append(serviceCells.conditionKeysCells, rows)
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

		actionTable := htmlTableTo2D(serviceCells.actionsCells)
		resourcesTable := htmlTableTo2D(serviceCells.resourcesCells)
		conditionKeysTable := htmlTableTo2D(serviceCells.conditionKeysCells)

		actions := actionsFromTable(actionTable)
		resources := resourcesFromTable(resourcesTable)
		conditionKeys := conditionKeysFromTable(conditionKeysTable)

		service := &Service{
			URL:           url,
			Name:          serviceCells.name,
			Prefix:        serviceCells.prefix,
			Actions:       actions,
			Resources:     resources,
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

		rows = append(rows, Cell{rowspan: rowspan, colspan: colspan, text: s.Text()})
	})

	return rows
}

// Removes leading and trailing whitespace.
// Removes empty strings.
func cleanupHTMLStringList(ss []string) []string {
	out := make([]string, 0)
	for _, s := range ss {
		s = strings.Trim(s, " \n\t")
		if len(s) > 0 {
			out = append(out, s)
		}
	}
	return out
}

func actionsFromTable(table [][]string) []*Action {
	actions := make([]*Action, len(table))
	for rowI, row := range table {
		action := &Action{
			Name:             strings.Trim(row[0], " \n\t"),
			Description:      strings.Trim(row[1], " \n\t"),
			AccessLevel:      strings.Trim(row[2], " \n\t"),
			ResourceTypes:    cleanupHTMLStringList(strings.Split(row[3], "\n")),
			ConditionKeys:    cleanupHTMLStringList(strings.Split(row[4], "\n")),
			DependentActions: cleanupHTMLStringList(strings.Split(row[5], "\n")),
		}
		actions[rowI] = action
	}
	return actions
}

func resourcesFromTable(table [][]string) []*Resource {
	resources := make([]*Resource, len(table))
	for rowI, row := range table {
		resource := &Resource{
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
			colspans[i] = cell.colspan
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
			if cell.rowspan == 0 {
				theseRowspans[i] = rowcount - rowI
			} else {
				theseRowspans[i] = cell.rowspan
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
			rowspan := cell.rowspan
			if rowspan == 0 {
				rowspan = rowcount - rowI
			}
			rowspansMap[colI] = rowspan

			colspan := cell.colspan
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
						table[testrow][testcol] = cell.text
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

func removeSpaces(ss []string) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = removeSpace(s)
	}
	return out
}

func getRawDataPath() string {
	homedir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}

	projectDir := path.Join(homedir, ".iampolicyhelper")
	return path.Join(projectDir, "rawData.json")
}
