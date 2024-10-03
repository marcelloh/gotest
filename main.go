// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Gotest is a tiny program that shells out to `go test`
// and prints the output in colour.
package main

/* ------------------------------- Imports --------------------------- */

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/enescakir/emoji"
	"github.com/iskaa02/qalam/bbcode"
)

/* ---------------------- Constants/Types/Variables ------------------ */

const (
	errColour = "red"
	colonSep  = ": "
)

var (
	isFile       bool
	skipDirs     = map[string]bool{}
	testDirs     = map[string]bool{}
	testFuncs    = map[string]string{}
	startTime    time.Time
	args         []string
	totalSkips   int
	totalFails   int
	totalNoTests int
	lastLine     string
	lastFunc     string
	fileLine     string //nolint:unused // false positive
	verbose      bool
	oldGo        bool
	testRunning  string
	firstLine    string

	// Used for filterWalker.
	filter     string
	rootDir    string
	module     string
	moduleRoot string
)

/* -------------------------- Methods/Functions ---------------------- */

/*
main is the bootstrap of the application.
*/
func main() {
	goVersion := runtime.Version()
	if goVersion < "go1.14" {
		oldGo = true
	}

	args = os.Args
	lastArg := args[len(args)-1]

	if lastArg == "loop" {
		args = args[:len(args)-1]
	}

	for _, value := range args {
		if value == "-v" {
			verbose = true
		}
	}

	bbcode.Printf("[white]%s[/white]", "gotest v1.19.22")
	println()

	if len(args) < 2 {
		bbcode.Printf("[red]%s[/red]", "no argument was given")
		println()
		os.Exit(0)
	}

	// only works when running in debug mode
	if strings.Contains(args[1], "-cd ") {
		parts := strings.Split(args[1], "-cd ")
		changeDir := parts[1]

		err := os.Chdir(changeDir)
		if err != nil {
			log.Print(err)
		}

		copy(args[1:], args[2:]) // Shift a[i+1:] left one index.
		args[len(args)-1] = ""   // Erase last element (write zero value).
		args = args[:len(args)-1]

		println("CD to ", changeDir)
	}

	rootDir, _ = os.Getwd()

	getModuleRoot(rootDir)

	os.Exit(run(lastArg))
}

/*
run starts to test all files.
*/
func run(lastArg string) int {
	startTime = time.Now().Local()

	findTestFiles(lastArg)

	exitCode := gotest(args[1:])

	busy := time.Since(startTime).String()
	println("Busy:", busy)

	if totalFails > 0 {
		bbcode.Printf("[red]%s %d [/red]", "Total fails:", totalFails)
		sadSmiley()
		println()
	} else {
		bbcode.Printf("[green]%s[/green]", "No fails")
		happySmiley()
		println()
	}

	if totalSkips > 0 {
		bbcode.Printf("[blue]%s [/blue]", "Total skips:")
		bbcode.Printf("[red]%d [/red]", totalSkips)
		println()
	}

	if totalNoTests > 0 {
		bbcode.Printf("[cyan]%s [/cyan]", "Total packages without tests:")
		bbcode.Printf("[red]%d[/red]", totalNoTests)
		println()
	}

	return exitCode
}

func getModuleRoot(dirpath string) {
	fileName := dirpath + "/go.mod"
	goFile := ""

	dataBytes, err := os.ReadFile(filepath.Clean(fileName))
	if err == nil {
		goFile = string(dataBytes)
	}

	lines := strings.Split(goFile, "\n")
	module = strings.ReplaceAll(lines[0], "module ", "")
	moduleRoot = filepath.Dir(module)
	module = strings.ReplaceAll(module, moduleRoot+"/", "")
}

/*
sadSmiley shows a bad status.
*/
func sadSmiley() {
	fmt.Printf(" %v", emoji.ThumbsDown)
}

/*
happySmiley shows a good status.
*/
func happySmiley() {
	fmt.Printf(" %v", emoji.ThumbsUp)
}

/*
gotest runs the necessary tests.
*/
func gotest(args []string) int {
	var waitgroup sync.WaitGroup

	waitgroup.Add(1)
	defer waitgroup.Wait()

	reader, writer := io.Pipe()
	defer func() {
		_ = writer.Close()
	}()

	cmd, err := runCLI(args, writer)
	if err != nil {
		log.Print(err)
		waitgroup.Done()

		return 1
	}

	go consume(&waitgroup, reader)

	sigc := make(chan os.Signal, 1)
	done := make(chan struct{})

	defer func() {
		done <- struct{}{}
	}()

	signal.Notify(sigc, os.Interrupt)

	go func() {
		for {
			select {
			case sig := <-sigc:
				_ = cmd.Process.Signal(sig)
			case <-done:
				return
			}
		}
	}()

	if err := cmd.Wait(); err != nil {
		if ws, ok := cmd.ProcessState.Sys().(syscall.WaitStatus); ok {
			return ws.ExitStatus()
		}

		return 1
	}

	return 0
}

func runCLI(args []string, writer *io.PipeWriter) (*exec.Cmd, error) {
	args = append([]string{"test"}, args...)
	cmd := exec.Command("go", args...)
	cmd.Stderr = writer
	cmd.Stdout = writer
	cmd.Env = os.Environ()

	err := cmd.Start()
	if err != nil {
		err = fmt.Errorf("runCLI %w", err)
	}

	return cmd, err
}

/*
consume gets the output from the tests.
*/
func consume(waitgroup *sync.WaitGroup, r io.Reader) {
	defer waitgroup.Done()

	reader := bufio.NewReader(r)

	for {
		line, _, err := reader.ReadLine()
		if err == io.EOF {
			break
		}

		if err != nil {
			log.Print(err)

			break
		}

		consumeLine(string(line))
		// break
	}
}

func consumeLine(lineText string) {
	parts := strings.Split(lineText, "\t")

	if firstLine == "" && len(parts) > 1 {
		firstLine = parts[1]
	}

	if len(parts) == 3 {
		checkDir := strings.ReplaceAll(parts[1], firstLine, "")
		if checkDir != "" && checkDir[0:1] == "/" {
			if _, found := testDirs[checkDir]; !found {
				return
			}
		}
	}

	parse(lineText)
}

/*
parse handles the output line by line.
*/
func parse(line string) {
	line = strings.ReplaceAll(line, moduleRoot, "")

	colour := ""
	trimmed := strings.TrimSpace(line)
	isNextFile := false

	colour = statusYellowGreenSkip(colour, trimmed)
	colour = statusUnknown(colour, trimmed)
	colour = statusAddFail(colour, trimmed)
	colour, trimmed, line, isNextFile = statusFail(colour, trimmed, line, isNextFile)

	// see if we can skip the mentioning of this
	if strings.Contains(line, "[no test files]") {
		superTrim := strings.Split(line, "\t")[1]
		if _, found := skipDirs[superTrim]; found {
			colour = "green"
			line = strings.ReplaceAll(line, "[no test files]", "[nothing to test]")
		} else {
			totalNoTests++
		}
	}

	if isFile {
		isFile = false

		showFileLink(trimmed)
	}

	if testRunning != "" && strings.HasPrefix(trimmed, testRunning) {
		fileLine = trimmed
		showFileLink(trimmed)
	}

	if colour == "" {
		fmt.Printf("%s\n", line)
	} else {
		bbcode.Printf("[%s]%s\n[/%s]", colour, line, colour)
	}

	lastLine = line

	if isNextFile {
		isFile = true
	}
}

func statusAddFail(colour, trimmed string) string {
	testRunning = ""

	switch {
	case strings.Contains(trimmed, "[build failed]"):
		colour = errColour
		totalFails++
	case strings.HasPrefix(trimmed, "# "):
		colour = errColour
		totalFails++
	}

	return colour
}

func statusFail(colourIn, trimmedIn, lineIn string, isNextFileIn bool) (colour, trimmed, line string, isNextFile bool) {
	colour = colourIn
	if strings.HasPrefix(trimmedIn, "FAIL") {
		colour = errColour
	}

	if !strings.Contains(trimmedIn, "--- FAIL") {
		return colour, trimmedIn, lineIn, isNextFileIn
	}

	parts := strings.Split(trimmedIn, "--- FAIL")
	trimmed = "--- FAIL" + parts[1]
	line = trimmed

	isNextFile = true
	fileLine = lastLine
	lastFunc = getFuncName(trimmed)
	colour = errColour

	return colour, trimmed, line, isNextFile
}

func statusUnknown(colour, trimmed string) string {
	if strings.HasPrefix(trimmed, "?") {
		colour = "cyan"
	}

	return colour
}

func statusYellowGreenSkip(colour, trimmed string) string {
	testRunning = ""

	switch {
	case strings.HasPrefix(trimmed, "--- SKIP"):
		colour = "blue"
		totalSkips++
		testRunning = strings.TrimSpace(strings.ReplaceAll(trimmed, "--- SKIP", "")) + colonSep
	case strings.HasPrefix(trimmed, "=== RUN"):
		testRunning = strings.TrimSpace(strings.ReplaceAll(trimmed, "=== RUN", "")) + colonSep
		colour = "yellow"
	case strings.HasPrefix(trimmed, "=== PAUSE"):
		testRunning = strings.TrimSpace(strings.ReplaceAll(trimmed, "=== PAUSE", "")) + colonSep
		colour = "yellow"
	case strings.HasPrefix(trimmed, "=== CONT"):
		testRunning = strings.TrimSpace(strings.ReplaceAll(trimmed, "=== CONT", "")) + colonSep
		colour = "yellow"
	case strings.HasPrefix(trimmed, "--- PASS"):
		colour = "green"
	case strings.HasPrefix(trimmed, "ok"):
		colour = "green"
	case strings.HasPrefix(trimmed, "PASS"):
		colour = "green"
	}

	return colour
}

/*
getFuncName returns the current function name.
*/
func getFuncName(text string) string {
	parts := strings.Split(text, " ")

	return parts[2]
}

/*
showFileLink shows a link to the current file.
*/
func showFileLink(line string) {
	if verbose && !oldGo {
		return
	}

	var (
		file     []string
		fileName string
	)

	file = strings.Split(line, colonSep)
	if !verbose || oldGo {
		fileName = file[0]
	} else {
		fileName = file[1]
	}

	if fileName == "--- FAIL" {
		return
	}

	fileParts := strings.Split(fileName, ".go")
	fileName = fileParts[0] + ".go"

	find := fileName + "_" + lastFunc
	split := strings.Split(find, "/.")
	find = split[0]

	dir := testFuncs[find]
	dir, _ = filepath.Abs(filepath.Dir(dir))
	dir = strings.ReplaceAll(dir, rootDir, "")
	dir = strings.ReplaceAll(dir, "\\", "/") // for windows

	text := fmt.Sprintf(".%s/%s", dir, fileName)

	if len(fileParts) > 1 {
		text = fmt.Sprintf("%s%s", text, fileParts[1])
	}

	if text[len(text)-1:] == ":" {
		text += "1:"
	}

	text = strings.TrimPrefix(text, "./")

	bbcode.Printf("[yellow]%s[/yellow]\n", text)
	println()

	totalFails++
}

/*
findTestFiles finds all testfiles.
*/
func findTestFiles(lastArg string) {
	dir, err := filepath.Abs(filepath.Dir(lastArg))
	if err != nil {
		log.Fatal(err)
	}

	_ = walker(dir, "*.go")
}

/*
walker gets all files in the filterDir and directories below.
*/
func walker(filterDir, locFilter string) error {
	filter = locFilter
	err := filepath.Walk(filterDir, filepath.WalkFunc(walkerFilter))

	for key, value := range skipDirs {
		if !value {
			delete(skipDirs, key)
		}
	}

	return fmt.Errorf("walker %w", err)
}

/*
walkerFilter filters all files on only the wanted ones.
*/
func walkerFilter(path string, fileInfo os.FileInfo, walkError error) (err error) {
	if walkError != nil {
		return walkError
	}

	if fileInfo.IsDir() {
		return nil
	}

	matched, err := filepath.Match(filter, fileInfo.Name())
	if err != nil || !matched {
		return nil //nolint:nilerr // everything is okay
	}

	path = strings.ReplaceAll(path, "\\", "/") // for windows

	if strings.Contains(path, "/vendor/") {
		return nil
	}

	memFile := ""

	dataBytes, err := os.ReadFile(filepath.Clean(path))
	if err == nil {
		memFile = string(dataBytes)
	}

	if strings.HasSuffix(path, "_test.go") {
		addTestFuncs(path, memFile)
		return nil
	}

	addSkipDir(path, memFile)

	return nil
}

func addTestFuncs(path, memFile string) {
	dir, _ := filepath.Abs(filepath.Dir(path))

	checkDir := strings.ReplaceAll(dir, rootDir, "")
	if _, found := testDirs[checkDir]; !found {
		testDirs[checkDir] = true
	}

	fileSet := token.NewFileSet()

	node, err := parser.ParseFile(fileSet, "", memFile, parser.ParseComments)
	if err != nil {
		log.Fatal(err)
	}

	file := filepath.Base(path)

	for _, f := range node.Decls {
		funcDecl, ok := f.(*ast.FuncDecl)
		if ok {
			functionName := file + "_" + funcDecl.Name.Name
			testFuncs[functionName] = path
		}
	}
}

// addSkipDir add full packages to the skipdir.
func addSkipDir(path, memFile string) {
	fset := token.NewFileSet()
	// parse the go source file, but only the package clause
	astFile, err := parser.ParseFile(fset, "", memFile, parser.PackageClauseOnly)
	if err != nil {
		return
	}

	if astFile.Name == nil {
		return
	}

	skip := true

	if strings.Contains(memFile, "func ") || strings.Contains(memFile, "func(") {
		if !strings.Contains(memFile, "DO NOT EDIT") {
			skip = false
		}
	}

	dir, _ := filepath.Abs(filepath.Dir(path))
	checkDir := strings.ReplaceAll(dir, rootDir, "/"+module)

	if skip {
		if _, found := skipDirs[checkDir]; !found {
			skipDirs[checkDir] = skip
		}
	} else {
		skipDirs[checkDir] = skip
	}
}
