package main

import (
	"bufio"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var (
	startLinePattern = regexp.MustCompile(`^goroutine\s+(\d+)\s+\[(.*)\]:$`)
)

func load(fn string) (*GoroutineDump, error) {
	fn = strings.Trim(fn, "\"")
	f, err := os.Open(fn)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	dump := NewGoroutineDump()
	var goroutine *Goroutine

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if startLinePattern.MatchString(line) {
			idx := strings.Index(line, "[")
			parts := strings.Split(line[idx+1:len(line)-2], ",")
			metas := map[MetaType]string{
				MetaState: strings.TrimSpace(parts[0]),
			}
			if len(parts) > 1 {
				metas[MetaDuration] = strings.TrimSpace(parts[1])
			}
			idstr := strings.TrimSpace(line[9:idx])
			id, err := strconv.Atoi(idstr)
			if err != nil {
				return nil, err
			}
			goroutine = NewGoroutine(id, metas)
			dump.Add(goroutine)
			goroutine.AddLine(line)
		} else if line == "" {
			// End of a goroutine section.
			if goroutine != nil {
				goroutine.Freeze()
			}
			goroutine = nil
		} else if goroutine != nil {
			goroutine.AddLine(line)
		}
	}

	if goroutine != nil {
		goroutine.Freeze()
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return dump, nil
}
