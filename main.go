package main

import (
	"fmt"
	"io"
	"log"
	"regexp"
	"strings"

	"os"

	"sort"

	"github.com/peterh/liner"
)

var (
	assignPattern = regexp.MustCompile(`^\s*[_a-zA-Z][_a-zA-Z0-9]*\s*=\s*.*$`)
	cdPattern     = regexp.MustCompile(`^\s*cd\s*.*$`)

	commands = map[string]string{
		"?":     "Show this help",
		"cd":    "Change current working directory",
		"clear": "Clear the workspace",
		"help":  "Show this help",
		"ls":    "Show files in current directory",
		"pwd":   "Show current working directory",
		"whos":  "Show all varaibles in workspace",
	}
	cmds []string

	workspace = map[string]*GoroutineDump{}
)

func init() {
	cmds = make([]string, 0, len(commands))
	for k := range commands {
		cmds = append(cmds, k)
	}
	sort.Sort(sort.StringSlice(cmds))
}

func main() {
	line := createLiner()
	defer line.Close()
	defer saveLiner(line)

	for {
		if cmd, err := line.Prompt(">> "); err == nil {
			cmd = strings.TrimSpace(cmd)
			if cmd == "" {
				continue
			}
			line.AppendHistory(cmd)

			switch cmd {
			case "?", "help":
				printHelp()
			case "clear":
				workspace = map[string]*GoroutineDump{}
				fmt.Println("Workspace cleared.")
			case "ls":
				wd, err := os.Getwd()
				if err != nil {
					fmt.Println(err)
					continue
				}
				printDir(wd)
			case "pwd":
				wd, err := os.Getwd()
				if err != nil {
					fmt.Println(err)
					continue
				}
				fmt.Println(wd)
			case "whos":
				if len(workspace) == 0 {
					fmt.Println("No variables defined.")
					continue
				}
				for k := range workspace {
					fmt.Printf("%s\t", k)
				}
				fmt.Println()
			default:
				if cdPattern.MatchString(cmd) {
					// Change directory.
					idx := strings.Index(cmd, "cd")
					dir := strings.TrimSpace(cmd[idx+2:])
					if dir == "" {
						fmt.Println("Expect command \"cd <dir>\"")
						continue
					}
					if err := os.Chdir(dir); err != nil {
						fmt.Println(err)
					}
					continue
				}

				// Assignment.
				if assignPattern.MatchString(cmd) {
					if err := assign(cmd); err != nil {
						fmt.Printf("Error, %s.\n", err.Error())
					}
					continue
				}

				if err := expr(cmd); err != nil {
					fmt.Printf("Error, %s.\n", err.Error())
				}
			}
		} else if err == liner.ErrPromptAborted || err == io.EOF {
			fmt.Println()
			break
		} else {
			log.Print("Error reading line: ", err)
		}
	}
}

func printDir(wd string) {
	f, err := os.Open(wd)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer f.Close()

	fis, err := f.Readdir(-1)
	if err != nil {
		fmt.Println(err)
		return
	}

	for _, fi := range fis {
		fmt.Println(fi.Name())
	}
}

func printHelp() {
	fmt.Println("Commands:")
	for _, k := range cmds {
		fmt.Printf("  %12s: %s\n", k, commands[k])
	}
	fmt.Println()
	fmt.Println("Statements:")
	fmt.Println("\t<var>")
	fmt.Println("\t<var> = load(\"<file-name>\")")
	fmt.Println("\t<var> = <another-var>")
	fmt.Println("\t<var> = <another-var>.copy()")
	fmt.Println("\t<var> = <another-var>.copy(\"<condition>\")")
	fmt.Println("\t<var>.delete(\"<condition>\")")
	fmt.Println("\t<var>.keep(\"<condition>\")")
	fmt.Println("\t<var>.search(\"<condition>\")")
	fmt.Println("\t<var>.search(\"<condition>\", offset)")
	fmt.Println("\t<var>.search(\"<condition>\", offset, limit)")
	fmt.Println("\t<var>.show()")
	fmt.Println("\t<var>.show(offset)")
	fmt.Println("\t<var>.show(offset, limit)")
	fmt.Println()
}
