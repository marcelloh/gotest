// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// gotest is a tiny program that shells out to `go test`
// and prints the output in color.
package main

/* ------------------------------- Imports --------------------------- */

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/ioutil"
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

	ct "github.com/daviddengcn/go-colortext"
)

/* ---------------------- Constants/Types/Variables ------------------ */

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
	fileLine     string //nolint: unused
	verbose      bool
	oldGo        bool
	testRunning  string

	// used for filterWalker
	filter            string
	depth, orgSlashes int
	rootDir           string
)

/* -------------------------- Methods/Functions ---------------------- */

/*
main is the bootstrap of the application
*/
func main() {
	// var exitCode int
	goVersion := runtime.Version()
	if strings.Compare(goVersion, "go1.14") < 0 {
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
run starts to test all files
*/
func run() int {
	startTime = time.Now().Local()

	ct.ResetColor()
	println("gotest v1.19.6")

	findTestFiles()

	exitCode := gotest(args[1:])

	ct.ResetColor()

	busy := time.Since(startTime).String()
	println("Busy:", busy)

	if totalFails > 0 {
		colorRed()
		print("Total fails: ", totalFails, " ")
		ct.ResetColor()
		sadSmiley()
		println()
	} else {
		colorGreen()
		print("No fails ")
		happySmiley()
		println()
	}

	if totalSkips > 0 {
		colorBlue()
		print("Total skips: ")
		colorRed()
		print(totalSkips, " ")
		ct.ResetColor()
		println()
	}

	if totalNoTests > 0 {
		colorCyan()
		print("Total packages without tests: ")
		colorRed()
		print(totalNoTests, " ")
		ct.ResetColor()
		println()
	}

	return exitCode
}

/*
sadSmiley shows a bad status
*/
func sadSmiley() {
	colorMagenta()
	print(":-(")
	ct.ResetColor()
}

/*
happySmiley shows a good status
*/
func happySmiley() {
	colorMagenta()
	print(":-)")
	ct.ResetColor()
}

/*
gotest runs the necessary tests
*/
func gotest(args []string) int {
	var wg sync.WaitGroup

	wg.Add(1)
	defer wg.Wait()

	r, w := io.Pipe()
	defer w.Close()

	args = append([]string{"test"}, args...)
	cmd := exec.Command("go", args...)
	cmd.Stderr = w
	cmd.Stdout = w
	cmd.Env = os.Environ()

	if err := cmd.Start(); err != nil {
		log.Print(err)

		return 1
	}

	go consume(&wg, r)

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

/*
consume gets the output from the tests
*/
func consume(wg *sync.WaitGroup, r io.Reader) {
	defer wg.Done()

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
parse haldes the output line by line
*/
func parse(line string) {
	trimmed := strings.TrimSpace(line)
	isNextFile := false

	checkYellow(trimmed)
	// success
	checkGreen(trimmed)

	switch {
	case strings.HasPrefix(trimmed, "--- SKIP"):
		colorBlue()
		totalSkips++

		testRunning = strings.TrimSpace(strings.ReplaceAll(trimmed, "--- SKIP", "")) + ": "
	case strings.HasPrefix(trimmed, "?"):
		colorCyan()
		totalNoTests++
	// failure
	case strings.Contains(trimmed, "--- FAIL"):
		colorRed()

		parts := strings.Split(trimmed, "--- FAIL")
		trimmed = "--- FAIL" + parts[1]
		line = trimmed

		if !strings.Contains(line, "/") {
			totalFails++ // only count the main test that failed
		}

		isNextFile = true
		fileLine = lastLine
		lastFunc = getFuncName(trimmed)
	case strings.Contains(trimmed, "[build failed]"):
		addFail()
	case strings.HasPrefix(trimmed, "# "):
		addFail()
	case strings.HasPrefix(trimmed, "FAIL"):
		colorRed()
	}

	if isFile {
		isFile = false

		showFileLink(trimmed)
	}

	if testRunning != "" && strings.HasPrefix(trimmed, testRunning) {
		fileLine = trimmed
		showFileLink(trimmed)
	}

	fmt.Printf("%s\n", line)
	lastLine = line

	ct.ResetColor()

	if isNextFile {
		isFile = true
	}
}

func addFail() {
	colorRed()

	totalFails++
}

func checkYellow(trimmed string) {
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
		colorYellow()
	}
}

func checkGreen(trimmed string) {
	switch {
	case strings.HasPrefix(trimmed, "--- PASS"):
		colorGreen()
	case strings.HasPrefix(trimmed, "ok"):
		colorGreen()
	case strings.HasPrefix(trimmed, "PASS"):
		colorGreen()
	}
}

/*
getFuncName returns the current function name
*/
func getFuncName(text string) string {
	parts := strings.Split(text, " ")

	return parts[2]
}

/*
showFileLink shows a link to the current file
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

	colorYellow()
	print("." + dir + "/" + fileName)

	if len(fileParts) > 1 {
		print(fileParts[1])
	}

	println()

	colorRed()
}

/*
colorBlue changes to output color to blue
*/
func colorBlue() {
	ct.ChangeColor(ct.Blue, false, ct.None, false)
}

/*
colorCyan changes to output color to cyan
*/
func colorCyan() {
	ct.ChangeColor(ct.Cyan, false, ct.None, false)
}

/*
colorGreen changes to output color to green
*/
func colorGreen() {
	ct.ChangeColor(ct.Green, false, ct.None, false)
}

/*
colorMagenta changes to output color to magenta
*/
func colorMagenta() {
	ct.ChangeColor(ct.Magenta, false, ct.None, false)
}

/*
colorRed changes to output color to red
*/
func colorRed() {
	ct.ChangeColor(ct.Red, true, ct.None, false)
}

/*
colorYellow changes to output color to yellow
*/
func colorYellow() {
	ct.ChangeColor(ct.Yellow, false, ct.None, false)
}

/*
findTestFiles finds all testfiles
*/
func findTestFiles() {
	dir, err := filepath.Abs(filepath.Dir("."))
	if err != nil {
		log.Fatal(err)
	}

	_ = walker(dir, "*_test.go")
}

/*
walker gets all files in the filterDir and directories below
*/
func walker(filterDir, locFilter string) error {
	filter = locFilter
	depth = 10
	orgSlashes = strings.Count(filterDir, "/") + 1 + depth
	// watchDirs := ""

	err := filepath.Walk(filterDir, filepath.WalkFunc(walkerFilter))

	return err
}

/*
walkerFilter filtyers all files ojn only the wanted ones
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
		return nil //nolint: nilerr
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

	dataBytes, err := ioutil.ReadFile(path)
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
			//functionName := file + "_" + funcDecl.Name.Name
			//testFuncs[functionName] = dir
			functionName := file + "_" + funcDecl.Name.Name
			// log.Println(`main.go:527 functionName:`, functionName)

			testFuncs[functionName] = path
		}
	}
}
