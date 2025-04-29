package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Key codes
const (
	KeyEnter     = 13
	KeyBackspace = 127
	KeyCtrlH     = 8

	KeyEscape  = 27
	KeyBracket = 91

	KeyArrowUp    = 65
	KeyArrowDown  = 66
	KeyArrowRight = 67
	KeyArrowLeft  = 68
)

// Escape sequences
const (
	EscSeqStart       = 27 // ESC
	EscSeqOpenBracket = 91 // '['
)

const maxStackSize = 1024 // Preallocate enough for very deep trees

type stop byte

const (
	ContinueAnyway stop = iota
	Conitinue
	StopAnyway
	Stop
)

func walkFast(root string, visit func(path string, name string, isDir bool) stop) error {
	stack := make([]string, 0, maxStackSize)
	stack = append(stack, root)

	for len(stack) > 0 {
		n := len(stack) - 1
		current := stack[n]
		stack = stack[:n]

		entries, err := os.ReadDir(current)
		if err != nil {
			continue // ignore unreadable dirs
		}

		var continueAnyway = false
		var continue_ = false
		var stopAnyway = false
		var stop_ = false
		for _, entry := range entries {
			s := visit(current, entry.Name(), entry.IsDir())
			switch s {
			case Conitinue:
				continue_ = true
			case ContinueAnyway:
				continueAnyway = true
			case StopAnyway:
				stopAnyway = true
			case Stop:
				stop_ = true
			}
		}
		goDeep := continue_ && !stop_
		if stopAnyway {
			goDeep = false
		} else if continueAnyway {
			goDeep = true
		}
		if goDeep {
			for i := len(entries) - 1; i >= 0; i-- { // Reverse order for proper DFS
				entry := entries[i]
				if entry.IsDir() {
					stack = append(stack, filepath.Join(current, entry.Name()))
				}
			}
		}
	}

	return nil
}

var projectMarkers = []string{
	"pom.xml", "go.mod", "package.json", "Cargo.toml", "Makefile", ".git", "main.js", "index.js",
}
var skipDirs = []string{
	"node_modules",
}

func findProjects(baseDirs []string) []string {
	var projects []string
	seen := make(map[string]struct{})

	for _, base := range baseDirs {
		walkFast(base, func(path, name string, isDir bool) stop {
			if slices.Contains(skipDirs, name) {
				return StopAnyway
			}
			if slices.Contains(projectMarkers, name) {
				if _, ok := seen[path]; !ok {
					projects = append(projects, path)
					seen[path] = struct{}{}
				}
				return Stop
			}

			if name == "go.work" {
				return ContinueAnyway
			}
			return Conitinue
		})
	}
	return projects
}

func fuzzyMatch(query, text string) (bool, int) {
	query = strings.ToLower(query)
	text = strings.ToLower(text)

	qIdx := len(query) - 1
	tIdx := len(text) - 1
	score := 0
	lastIdx := -1

	for qIdx >= 0 && tIdx >= 0 {
		if query[qIdx] == text[tIdx] {
			if lastIdx >= 0 {
				score += min(lastIdx-tIdx, 3)
			}
			lastIdx = tIdx
			qIdx--
		}
		tIdx--
	}
	if qIdx >= 0 {
		return false, 0
	}
	return true, score
}

type scored struct {
	project string
	score   int
}

func filterProjects(projects []string, query string) ([]string, []scored) {
	if query == "" {
		return projects, nil
	}

	var matches []scored
	for _, p := range projects {
		parts := strings.Split(p, "/")
		n := len(parts)
		var match bool
		var score int
		if n > 0 {
			last := parts[n-1]
			match, score = fuzzyMatch(query, last)
			if match && 1 == 2 {
				goto add
			}
		}
		match, score = fuzzyMatch(query, p)
	add:
		if match {
			matches = append(matches, scored{project: p, score: score})
		}
	}
	slices.SortFunc(matches, func(a, b scored) int {
		return a.score - b.score
	})

	var result = make([]string, len(matches))
	for i, m := range matches {
		result[i] = m.project
	}
	return result, matches
}

func Must[T any](val T, err error) T {
	return val
}

const Trims = "/Users/islombek/Projects/"
const CacheFile = "~/.cache/fuzzyprojectfind.json"

type Cache struct {
	Projects []string `json:"projects"`
}

func loadCache(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Cache
	err = json.Unmarshal(data, &c)
	if err != nil {
		return nil, err
	}
	return c.Projects, nil
}

func saveCache(path string, projects []string) error {
	c := Cache{Projects: projects}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func main() {

	baseDirs := []string{"/Users/islombek/Projects"}

	projects, _ := loadCache(CacheFile)

	find := func() {
		projects = findProjects(baseDirs)
		saveCache(CacheFile, projects)
	}
	if len(projects) == 0 {
		find()
	} else {
		go find()
	}

	if len(projects) == 0 {
		fmt.Println("No projects found.")
		os.Exit(0)
	}

	app := tview.NewApplication()

	// Create a text input field for the search query

	// Create a table to display the filtered projects
	projectList := tview.NewTable().
		SetBorders(false).
		SetSelectable(true, false)

	filter := Must(regexp.Compile("[a-zA-Z0-9\\-_+\\.#@$%^&*\\(\\)\u0400-\u04FF]"))
	var searchQuery []rune
	label := tview.NewTextView().
		SetText(string(searchQuery))

	var filteredProjects []string
	updateTable := func(query string) {
		var scores []scored
		filteredProjects, scores = filterProjects(projects, query)
		projectList.Clear()
		for i, project := range filteredProjects {
			var score = 0
			if len(scores) > i {
				score = scores[i].score
			}
			text := fmt.Sprintf("%02d:.%s", score, strings.Replace(project, Trims, "", 1))
			projectList.SetCell(i, 0, tview.NewTableCell(text))
		}
		projectList.ScrollToBeginning()
		projectList.Select(0, 0)
	}
	var selectedFolder *string = nil
	projectList.SetSelectedFunc(func(row, column int) {
		selectedFolder = &filteredProjects[row]
		app.Stop()
	})

	// Initially update the table with all projects
	updateTable("")

	// Handle text input changes and update table
	// Layout: place the search input and the project list in a flex layout
	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(projectList, 0, 1, true).
		AddItem(label, 1, 0, false)

	flex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if filter.Match([]byte(string(event.Rune()))) {
			searchQuery = append(searchQuery, event.Rune())
		} else {
			key := event.Key()
			switch key {
			case tcell.KeyBS, tcell.KeyDEL, tcell.KeyDelete:
				if len(searchQuery) > 0 {
					searchQuery = searchQuery[:len(searchQuery)-1]
				}
			case tcell.KeyCR, tcell.KeyUp, tcell.KeyDown:
				return event
			}
		}
		label.SetText(string(searchQuery))
		updateTable(string(searchQuery))
		return nil
	})

	// Run the application
	if err := app.SetRoot(flex, true).Run(); err != nil {
		fmt.Println("Error running application:", err)
		os.Exit(1)
	}

	if selectedFolder != nil {
		fmt.Print(*selectedFolder)
	} else {
		fmt.Println("No Selection")
	}
}
