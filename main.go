// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// gotest is a tiny program that shells out to `go test`
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

const errColour = "red"

var (
	isFile       bool
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

	// used for filterWalker.
	filter            string
	depth, orgSlashes int
	rootDir           string
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

	rootDir, _ = os.Getwd()

	os.Exit(run())
}

/*
run starts to test all files.
*/
func run() int {
	startTime = time.Now().Local()

	bbcode.Printf("[white]%s[/white]", "gotest v1.19.13")
	println()
	findTestFiles()

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
			return
		}

		if err != nil {
			log.Print(err)

			return
		}

		parse(string(line))
	}
}

/*
parse handles the output line by line.
*/
func parse(line string) {
	colour := ""
	trimmed := strings.TrimSpace(line)
	isNextFile := false

	colour = statusYellow(colour, trimmed)
	colour = statusGreen(colour, trimmed) // success
	colour = statusSkip(colour, trimmed)
	colour = statusUnknown(colour, trimmed)
	colour = statusAddFail(colour, trimmed)
	colour, trimmed, line, isNextFile = statusFail(colour, trimmed, line, isNextFile)

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

func statusFail(colour, trimmedIn, lineIn string, isNextFileIn bool) (errColour, trimmed, line string, isNextFile bool) {
	if strings.HasPrefix(trimmed, "FAIL") {
		colour = errColour
	}

	if !strings.Contains(trimmedIn, "--- FAIL") {
		return colour, trimmedIn, lineIn, isNextFileIn
	}

	parts := strings.Split(trimmedIn, "--- FAIL")
	trimmed = "--- FAIL" + parts[1]
	line = trimmed

	if !strings.Contains(line, "/") {
		totalFails++
	}

	isNextFile = true
	fileLine = lastLine
	lastFunc = getFuncName(trimmed)

	return errColour, trimmed, line, isNextFile
}

func statusUnknown(colour, trimmed string) string {
	if strings.HasPrefix(trimmed, "?") {
		colour = "cyan"
		totalNoTests++
	}

	return colour
}

func statusSkip(colour, trimmed string) string {
	testRunning = ""

	if strings.HasPrefix(trimmed, "--- SKIP") {
		colour = "blue"
		totalSkips++

		testRunning = strings.TrimSpace(strings.ReplaceAll(trimmed, "--- SKIP", "")) + ": "
	}

	return colour
}

func statusYellow(colour, trimmed string) string {
	testRunning = ""

	switch {
	case strings.HasPrefix(trimmed, "=== RUN"):
		testRunning = strings.TrimSpace(strings.ReplaceAll(trimmed, "=== RUN", "")) + ": "
	case strings.HasPrefix(trimmed, "=== PAUSE"):
		testRunning = strings.TrimSpace(strings.ReplaceAll(trimmed, "=== PAUSE", "")) + ": "
	case strings.HasPrefix(trimmed, "=== CONT"):
		testRunning = strings.TrimSpace(strings.ReplaceAll(trimmed, "=== CONT", "")) + ": "
	}

	if testRunning != "" {
		colour = "yellow"
	}

	return colour
}

func statusGreen(colour, trimmed string) string {
	ret := false

	switch {
	case strings.HasPrefix(trimmed, "--- PASS"):
		ret = true
	case strings.HasPrefix(trimmed, "ok"):
		ret = true
	case strings.HasPrefix(trimmed, "PASS"):
		ret = true
	}

	if ret {
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

	file = strings.Split(line, ": ")
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
}

/*
findTestFiles finds all testfiles.
*/
func findTestFiles() {
	dir, err := filepath.Abs(filepath.Dir("."))
	if err != nil {
		log.Fatal(err)
	}

	_ = walker(dir, "*_test.go")
}

/*
walker gets all files in the filterDir and directories below.
*/
func walker(filterDir, locFilter string) error {
	filter = locFilter
	depth = 10
	orgSlashes = strings.Count(filterDir, "/") + 1 + depth
	err := filepath.Walk(filterDir, filepath.WalkFunc(walkerFilter))

	return fmt.Errorf("walker %w", err)
}

/*
walkerFilter filtyers all files ojn only the wanted ones.
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
	if depth >= 0 {
		slashes := strings.Count(path, "/")
		if slashes > orgSlashes {
			return nil
		}
	}

	if strings.Contains(path, "/vendor/") {
		return nil
	}

	addTestFuncs(path)

	return nil
}

func addTestFuncs(path string) {
	dir, _ := filepath.Abs(filepath.Dir(path))
	file := strings.ReplaceAll(path, dir+"/", "")

	memFile := ""

	dataBytes, err := os.ReadFile(filepath.Clean(path))
	if err == nil {
		memFile = string(dataBytes)
	}

	fileSet := token.NewFileSet()

	node, err := parser.ParseFile(fileSet, "", memFile, parser.ParseComments)
	if err != nil {
		log.Fatal(err)
	}

	for _, f := range node.Decls {
		funcDecl, ok := f.(*ast.FuncDecl)
		if ok {
			functionName := file + "_" + funcDecl.Name.Name
			testFuncs[functionName] = path
		}
	}
}
