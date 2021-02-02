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
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	ct "github.com/daviddengcn/go-colortext"
	"github.com/fsnotify/fsnotify"
)

/* ---------------------- Constants/Types/Variables ------------------ */

var (
	isFile      bool
	testFuncs   = map[string]string{}
	startTime   time.Time
	watcher     = &fsnotify.Watcher{}
	args        []string
	totalFails  int
	lastLine    string
	lastFunc    string
	fileLine    string
	verbose     bool
	oldGo       bool
	testRunning string
)

/* -------------------------- Methods/Functions ---------------------- */

/*
main is the bootstrap of the application
*/
func main() {
	var exitCode int
	watcher, _ = fsnotify.NewWatcher()
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

	endless := true
	for endless {
		exitCode = run()
		if lastArg == "loop" {
			totalFails = 0
			stopLoop := monitorChanges()
			if stopLoop {
				endless = false
			}
		} else {
			endless = false
		}
	}

	watcher.Close()
	os.Exit(exitCode)
}

/*
run starts to test all files
*/
func run() int {
	startTime = time.Now().Local()
	ct.ResetColor()
	println("gotest v.1.15")

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

	read, write := io.Pipe()
	defer write.Close()

	args = append([]string{"test"}, args...)
	cmd := exec.Command("go", args...)
	cmd.Stderr = write
	cmd.Stdout = write
	cmd.Env = os.Environ()

	go consume(&wg, read)

	if err := cmd.Run(); err != nil {
		if wstatus, ok := cmd.ProcessState.Sys().(syscall.WaitStatus); ok {
			return wstatus.ExitStatus()
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

	switch {
	case strings.HasPrefix(trimmed, "=== RUN"):
		colorWhite()
		testRunning = strings.TrimSpace(strings.ReplaceAll(trimmed, "=== RUN", "")) + ": "
	case strings.HasPrefix(trimmed, "?"):
		colorCyan()
	// success
	case strings.HasPrefix(trimmed, "--- PASS"):
		colorGreen()
	case strings.HasPrefix(trimmed, "ok"):
		colorGreen()
	case strings.HasPrefix(trimmed, "PASS"):
		colorGreen()
	// failure
	case strings.Contains(trimmed, "--- FAIL"):
		parts := strings.Split(trimmed, "--- FAIL")
		trimmed = "--- FAIL" + parts[1]
		line = trimmed
		totalFails++
		isNextFile = true
		fileLine = lastLine
		lastFunc = getFuncName(trimmed)
		colorRed()
	case strings.HasPrefix(trimmed, "FAIL"):
		colorRed()
	}

	if isFile {
		isFile = false
		if !verbose || oldGo {
			showFileLink(trimmed)
		}
	}

	if testRunning != "" && strings.HasPrefix(trimmed, testRunning) {
		fileLine = trimmed
		if verbose && !oldGo {
			showFileLink(trimmed)
		}
	}

	fmt.Printf("%s\n", line)
	lastLine = line
	ct.ResetColor()

	if isNextFile {
		isFile = true
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
	colorYellow()
	var file []string
	var fileName string

	file = strings.Split(line, ": ")
	if !verbose || oldGo {
		fileName = file[0]
	} else {
		fileName = file[1]
	}

	fileParts := strings.Split(fileName, ".go")
	fileName = fileParts[0] + ".go"

	print(testFuncs[fileName+"_"+lastFunc] + "/" + fileName)

	if len(fileParts) > 1 {
		print(fileParts[1])
	}
	println()

	colorRed()
}

/*
colorRed changes to output color to red
*/
func colorRed() {
	ct.ChangeColor(ct.Red, true, ct.None, false)
}

/*
colorWhite changes to output color to white
*/
func colorWhite() {
	ct.ChangeColor(ct.White, false, ct.None, false)
}

/*
colorGreen changes to output color to green
*/
func colorGreen() {
	ct.ChangeColor(ct.Green, false, ct.None, false)
}

/*
colorCyan changes to output color to cyan
*/
func colorCyan() {
	ct.ChangeColor(ct.Cyan, false, ct.None, false)
}

/*
colorYellow changes to output color to yellow
*/
func colorYellow() {
	ct.ChangeColor(ct.Yellow, false, ct.None, false)
}

/*
colorMagenta changes to output color to magenta
*/
func colorMagenta() {
	ct.ChangeColor(ct.Magenta, false, ct.None, false)
}

/*
findTestFiles finds all testfiles
*/
func findTestFiles() {

	dir, err := filepath.Abs(filepath.Dir("."))
	if err != nil {
		log.Fatal(err)
	}

	_ = walker(dir, "*_test.go", 10)
}

/*
walker gets all files in the filterDir and directories below
*/
func walker(filterDir, filter string, depth int) error {
	orgSlashes := strings.Count(filterDir, "/") + 1 + depth
	watchDirs := ""

	err := filepath.Walk(filterDir, func(path string, fileInfo os.FileInfo, walkError error) (err error) {
		if walkError != nil {
			return walkError
		}

		if fileInfo.IsDir() {
			return nil
		}

		matched, err := filepath.Match(filter, fileInfo.Name())
		if err == nil && matched {
			path = strings.Replace(path, "\\", "/", -1) // for windows
			if depth >= 0 {
				slashes := strings.Count(path, "/")
				if slashes > orgSlashes {
					return nil
				}
			}

			if !strings.Contains(path, "/vendor/") {
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
						functionName := file + "_" + funcDecl.Name.Name
						testFuncs[functionName] = dir
					}
				}

				if !strings.Contains(watchDirs, dir+",") {
					watchDirs += dir + ","
					_ = watcher.Add(dir)
				}
			}
		}

		return nil
	})

	return err
}

/*
monitorChanges monitors if a change happens
*/
func monitorChanges() bool {

	var wg sync.WaitGroup
	stopLoop := false
	endless := true

	wg.Add(1)
	go func() {
		for endless {
			select {
			case event := <-watcher.Events:
				if event.Op.String() == "WRITE" {
					defer wg.Done()
					endless = false
				}

			case err := <-watcher.Errors:
				fmt.Print("ERROR: ", err)
			}
		}
	}()
	wg.Wait()

	return stopLoop
}
