package main

import (
	"bytes"
	"crypto/md5"
	"errors"
	"fmt"
	"hash"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"os"

	"github.com/Knetic/govaluate"
	"github.com/foize/go.sgr"
)

type MetaType int

var (
	MetaState    MetaType = 0
	MetaDuration MetaType = 1

	durationPattern = regexp.MustCompile(`^\d+ minutes$`)

	functions = map[string]govaluate.ExpressionFunction{
		"contains": func(args ...interface{}) (interface{}, error) {
			if len(args) != 2 {
				return nil, fmt.Errorf("contains() accepts exactly two arguments")
			}
			idx := strings.Index(args[0].(string), args[1].(string))
			return bool(idx > -1), nil
		},
		"lower": func(args ...interface{}) (interface{}, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("lower() accepts exactly one arguments")
			}
			lowered := strings.ToLower(args[0].(string))
			return string(lowered), nil
		},
		"upper": func(args ...interface{}) (interface{}, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("upper() accepts exactly one arguments")
			}
			uppered := strings.ToUpper(args[0].(string))
			return string(uppered), nil
		},
	}
)

// Goroutine contains a goroutine info.
type Goroutine struct {
	id       int
	header   string
	trace    string
	lines    int
	duration int // In minutes.
	metas    map[MetaType]string

	lineMd5    []string
	fullMd5    string
	fullHasher hash.Hash
	duplicates []int

	frozen bool
	buf    *bytes.Buffer
}

// AddLine appends a line to the goroutine info.
func (g *Goroutine) AddLine(l string) {
	if !g.frozen {
		g.lines++
		g.buf.WriteString(l)
		g.buf.WriteString("\n")

		if strings.HasPrefix(l, "\t") {
			parts := strings.Split(l, " ")
			if len(parts) != 2 {
				fmt.Println("ignored one line for digest")
				return
			}

			fl := strings.TrimSpace(parts[0])

			h := md5.New()
			io.WriteString(h, fl)
			g.lineMd5 = append(g.lineMd5, string(h.Sum(nil)))

			io.WriteString(g.fullHasher, fl)
		}
	}
}

// Freeze freezes the goroutine info.
func (g *Goroutine) Freeze() {
	if !g.frozen {
		g.frozen = true
		g.trace = g.buf.String()
		g.buf = nil

		g.fullMd5 = string(g.fullHasher.Sum(nil))
	}
}

// Print outputs the goroutine details to w.
func (g Goroutine) Print(w io.Writer) error {
	if _, err := fmt.Fprint(w, g.header); err != nil {
		return err
	}
	if len(g.duplicates) > 0 {
		if _, err := fmt.Fprintf(w, " %d times: [[", len(g.duplicates)); err != nil {
			return err
		}
		for i, id := range g.duplicates {
			if i > 0 {
				if _, err := fmt.Fprint(w, ", "); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprint(w, id); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprint(w, "]"); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, g.trace); err != nil {
		return err
	}
	return nil
}

// PrintWithColor outputs the goroutine details to stdout with color.
func (g Goroutine) PrintWithColor() {
	sgr.Printf("[fg-blue]%s[reset]", g.header)
	if len(g.duplicates) > 0 {
		sgr.Printf(" [fg-red]%d[reset] times: [[", len(g.duplicates))
		for i, id := range g.duplicates {
			if i > 0 {
				sgr.Printf(", ")
			}
			sgr.Printf("[fg-green]%d[reset]", id)
		}
		sgr.Print("]")
	}
	sgr.Println()
	sgr.Println(g.trace)
}

// NewGoroutine creates and returns a new Goroutine.
func NewGoroutine(metaline string) (*Goroutine, error) {
	idx := strings.Index(metaline, "[")
	parts := strings.Split(metaline[idx+1:len(metaline)-2], ",")
	metas := map[MetaType]string{
		MetaState: strings.TrimSpace(parts[0]),
	}

	duration := 0
	if len(parts) > 1 {
		value := strings.TrimSpace(parts[1])
		metas[MetaDuration] = value
		if durationPattern.MatchString(value) {
			if d, err := strconv.Atoi(value[:len(value)-8]); err == nil {
				duration = d
			}
		}
	}

	idstr := strings.TrimSpace(metaline[9:idx])
	id, err := strconv.Atoi(idstr)
	if err != nil {
		return nil, err
	}

	return &Goroutine{
		id:         id,
		lines:      1,
		header:     metaline,
		buf:        &bytes.Buffer{},
		duration:   duration,
		metas:      metas,
		fullHasher: md5.New(),
		duplicates: []int{},
	}, nil
}

// GoroutineDump defines a goroutine dump.
type GoroutineDump struct {
	goroutines []*Goroutine
}

// Add appends a goroutine info to the list.
func (gd *GoroutineDump) Add(g *Goroutine) {
	gd.goroutines = append(gd.goroutines, g)
}

// Copy duplicates and returns the GoroutineDump.
func (gd GoroutineDump) Copy(cond string) *GoroutineDump {
	dump := GoroutineDump{
		goroutines: []*Goroutine{},
	}
	if cond == "" {
		// Copy all.
		for _, d := range gd.goroutines {
			dump.goroutines = append(dump.goroutines, d)
		}
	} else {
		goroutines, err := gd.withCondition(cond, func(i int, g *Goroutine, passed bool) *Goroutine {
			if passed {
				return g
			}
			return nil
		})
		if err != nil {
			fmt.Println(err)
			return nil
		}
		dump.goroutines = goroutines
	}
	return &dump
}

// Dedup finds goroutines with duplicated stack traces and keeps only one copy
// of them.
func (gd *GoroutineDump) Dedup() {
	m := map[string][]int{}
	for _, g := range gd.goroutines {
		if _, ok := m[g.fullMd5]; ok {
			m[g.fullMd5] = append(m[g.fullMd5], g.id)
		} else {
			m[g.fullMd5] = []int{g.id}
		}
	}

	kept := make([]*Goroutine, 0, len(gd.goroutines))

outter:
	for digest, ids := range m {
		for _, g := range gd.goroutines {
			if g.fullMd5 == digest {
				g.duplicates = ids
				kept = append(kept, g)
				continue outter
			}
		}
	}

	if len(gd.goroutines) != len(kept) {
		fmt.Printf("Dedupped %d, kept %d\n", len(gd.goroutines), len(kept))
		gd.goroutines = kept
	}
}

// Delete deletes by the condition.
func (gd *GoroutineDump) Delete(cond string) error {
	goroutines, err := gd.withCondition(cond, func(i int, g *Goroutine, passed bool) *Goroutine {
		if !passed {
			return g
		}
		return nil
	})
	if err != nil {
		return err
	}
	gd.goroutines = goroutines
	return nil
}

// Diff shows the difference between two dumps.
func (gd *GoroutineDump) Diff(another *GoroutineDump) (*GoroutineDump, *GoroutineDump, *GoroutineDump) {
	lonly := map[int]*Goroutine{}
	ronly := map[int]*Goroutine{}
	common := map[int]*Goroutine{}

	for _, v := range gd.goroutines {
		lonly[v.id] = v
	}
	for _, v := range another.goroutines {
		if _, ok := lonly[v.id]; ok {
			delete(lonly, v.id)
			common[v.id] = v
		} else {
			ronly[v.id] = v
		}
	}
	return NewGoroutineDumpFromMap(lonly), NewGoroutineDumpFromMap(common), NewGoroutineDumpFromMap(ronly)
}

// Keep keeps by the condition.
func (gd *GoroutineDump) Keep(cond string) error {
	goroutines, err := gd.withCondition(cond, func(i int, g *Goroutine, passed bool) *Goroutine {
		if passed {
			return g
		}
		return nil
	})
	if err != nil {
		return err
	}
	gd.goroutines = goroutines
	return nil
}

// Save saves the goroutine dump to the given file.
func (gd GoroutineDump) Save(fn string) error {
	f, err := os.Create(fn)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, g := range gd.goroutines {
		if err := g.Print(f); err != nil {
			return err
		}
	}
	return nil
}

// Search displays the goroutines with the offset and limit.
func (gd GoroutineDump) Search(cond string, offset, limit int) {
	sgr.Printf("[fg-green]Search with offset %d and limit %d.[reset]\n\n", offset, limit)

	count := 0
	_, err := gd.withCondition(cond, func(i int, g *Goroutine, passed bool) *Goroutine {
		if passed {
			if count >= offset && count < offset+limit {
				g.PrintWithColor()
			}
			count++
		}
		return nil
	})
	if err != nil {
		fmt.Println(err)
	}
}

// Show displays the goroutines with the offset and limit.
func (gd GoroutineDump) Show(offset, limit int) {
	for i := offset; i < offset+limit && i < len(gd.goroutines); i++ {
		gd.goroutines[offset+i].PrintWithColor()
	}
}

// Sort sorts the goroutine entries.
func (gd *GoroutineDump) Sort() {
	fmt.Printf("# of goroutines: %d\n", len(gd.goroutines))
}

// Summary prints the summary of the goroutine dump.
func (gd GoroutineDump) Summary() {
	fmt.Printf("# of goroutines: %d\n", len(gd.goroutines))
	stats := map[string]int{}
	if len(gd.goroutines) > 0 {
		for _, g := range gd.goroutines {
			stats[g.metas[MetaState]]++
		}
		fmt.Println()
	}
	if len(stats) > 0 {
		states := make([]string, 0, 10)
		for k := range stats {
			states = append(states, k)
		}
		sort.Sort(sort.StringSlice(states))

		for _, k := range states {
			fmt.Printf("%15s: %d\n", k, stats[k])
		}
		fmt.Println()
	}
}

// NewGoroutineDump creates and returns a new GoroutineDump.
func NewGoroutineDump() *GoroutineDump {
	return &GoroutineDump{
		goroutines: []*Goroutine{},
	}
}

// NewGoroutineDumpFromMap creates and returns a new GoroutineDump from a map.
func NewGoroutineDumpFromMap(gs map[int]*Goroutine) *GoroutineDump {
	gd := &GoroutineDump{
		goroutines: []*Goroutine{},
	}
	for _, v := range gs {
		gd.goroutines = append(gd.goroutines, v)
	}
	return gd
}

func (gd *GoroutineDump) withCondition(cond string, callback func(int, *Goroutine, bool) *Goroutine) ([]*Goroutine, error) {
	cond = strings.Trim(cond, "\"")
	expression, err := govaluate.NewEvaluableExpressionWithFunctions(cond, functions)
	if err != nil {
		return nil, err
	}

	goroutines := make([]*Goroutine, 0, len(gd.goroutines))
	for i, g := range gd.goroutines {
		params := map[string]interface{}{
			"id":       g.id,
			"duration": g.duration,
			"lines":    g.lines,
			"state":    g.metas[MetaState],
			"trace":    g.trace,
		}
		res, err := expression.Evaluate(params)
		if err != nil {
			return nil, err
		}
		if val, ok := res.(bool); ok {
			if gor := callback(i, g, val); gor != nil {
				goroutines = append(goroutines, gor)
			}
		} else {
			return nil, errors.New("argument expression should return a boolean")
		}
	}
	fmt.Printf("Deleted %d goroutines, kept %d.\n", len(gd.goroutines)-len(goroutines), len(goroutines))
	return goroutines, nil
}
