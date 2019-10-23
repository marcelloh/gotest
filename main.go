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
	"strings"
	"sync"
	"syscall"

	ct "github.com/daviddengcn/go-colortext"
)

var (
	isFile    bool
	testFiles = map[string]string{}
)

func main() {
	findTestFiles()

	exitCode := gotest(os.Args[1:])
	colorWhite()
	os.Exit(exitCode)
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
		fallthrough
	case strings.HasPrefix(trimmed, "ok"):
		fallthrough
	case strings.HasPrefix(trimmed, "PASS"):
		colorGreen()

	// failure
	case strings.HasPrefix(trimmed, "--- FAIL"):
		isNextFile = true
		colorRed()
	case strings.HasPrefix(trimmed, "FAIL"):
		colorRed()
	}

	if isFile {
		isFile = false
		colorYellow()

		printFullFile(trimmed)
		file := strings.Split(trimmed, ": ")
		colorRed()
		line = " " + file[1]
	}

	fmt.Printf("%s\n", line)

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

/*
walker gets all files in the filterDir and directories below
You can:
    filter files like "*.go"
    give a max. directory depth
        -1 = unrestricted deep
        0 = only filterDir
        1 =  filterDir and 1 deeper
        etc.
Example (unrestricted directories deep):
files, err := Walker(rootDir, "*.go", -1)

TODO: Move this to library
*/
func walker(filterDir, filter string, depth int) error {
	orgSlashes := strings.Count(filterDir, "/") + 1 + depth
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
				testFiles[file] = path
			}
		}

		return nil
	})

	return err
}

func findTestFiles() {
	dir, err := filepath.Abs(filepath.Dir("."))
	if err != nil {
		log.Fatal(err)
	}

	_ = walker(dir, "*_test.go", 10)
}

func printFullFile(file string) {
	fileParts := strings.Split(file, ".go")
	file = fileParts[0] + ".go"
	print(testFiles[file])
}
