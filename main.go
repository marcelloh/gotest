// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// gotest is a tiny program that shells out to `go test`
// and prints the output in color.
package main

import (
	"bufio"
	"fmt"
	"io"
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

var (
	isFile     bool
	testFiles  = map[string]string{}
	startTime  time.Time
	watcher    = &fsnotify.Watcher{}
	args       []string
	totalFails int
	lastLine   string
	fileLine   string
	verbose    bool
	oldGo      bool
)

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

func run() int {
	startTime = time.Now().Local()
	ct.ResetColor()
	println("gotest v.1.08")

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

func sadSmiley() {
	colorMagenta()
	print(":-(")
	ct.ResetColor()
}

func happySmiley() {
	colorMagenta()
	print(":-)")
	ct.ResetColor()
}

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

	go consume(&wg, r)

	if err := cmd.Run(); err != nil {
		if ws, ok := cmd.ProcessState.Sys().(syscall.WaitStatus); ok {
			return ws.ExitStatus()
		}

		return 1
	}

	return 0
}

func consume(wg *sync.WaitGroup, r io.Reader) {
	defer wg.Done()

	reader := bufio.NewReader(r)

	for {
		l, _, err := reader.ReadLine()
		if err == io.EOF {
			return
		}

		if err != nil {
			log.Print(err)
			return
		}

		parse(string(l))
	}
}

func parse(line string) {
	trimmed := strings.TrimSpace(line)
	isNextFile := false

	switch {
	case strings.HasPrefix(trimmed, "=== RUN"):
		colorWhite()
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
	case strings.HasPrefix(trimmed, "--- FAIL"):
		totalFails++
		isNextFile = true
		fileLine = lastLine

		colorRed()
	case strings.HasPrefix(trimmed, "FAIL"):
		colorRed()
	}

	if isFile {
		isFile = false

		colorYellow()
		var file []string
		var fileName string
		if verbose || !oldGo {
			file = strings.Split(fileLine, ": ")
			fileName = file[1]
		} else {
			file = strings.Split(line, ": ")
			fileName = strings.TrimSpace(file[0])
		}

		printFullFile(fileName)
		colorRed()

		line = " "
	}

	fmt.Printf("%s\n", line)
	lastLine = line

	ct.ResetColor()

	if isNextFile {
		isFile = true
	}
}

func colorRed() {
	ct.ChangeColor(ct.Red, false, ct.None, false)
}

func colorWhite() {
	ct.ChangeColor(ct.White, false, ct.None, false)
}

func colorGreen() {
	ct.ChangeColor(ct.Green, false, ct.None, false)
}

func colorCyan() {
	ct.ChangeColor(ct.Cyan, false, ct.None, false)
}

func colorYellow() {
	ct.ChangeColor(ct.Yellow, false, ct.None, false)
}

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
				testFiles[file] = dir

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

func monitorChanges() bool {

	var wg sync.WaitGroup
	stopLoop := false
	endless := true

	// quit := make(chan os.Signal, 1)
	// signal.Notify(quit, os.Interrupt)
	// wg.Add(1)
	// go func() {
	// 	println("1st")
	// 	<-quit
	// 	println("\nTest loop is stopping...")
	// 	endless = false
	// 	stopLoop = true
	// 	defer wg.Done()
	// }()

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

func printFullFile(file string) {
	file = strings.TrimSpace(file)
	fileParts := strings.Split(file, ".go")
	file = fileParts[0] + ".go"

	print(testFiles[file] + "/" + file)

	if len(fileParts) > 1 {
		print(fileParts[1])
	}
}
