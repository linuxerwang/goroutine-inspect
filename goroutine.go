package main

import (
	"bytes"
	"fmt"
	"strings"

	"errors"

	"github.com/Knetic/govaluate"
)

type MetaType int

var (
	MetaState    MetaType = 0
	MetaDuration MetaType = 1
)

// Goroutine contains a goroutine info.
type Goroutine struct {
	id    int
	trace string
	lines int
	metas map[MetaType]string

	freezed bool
	buf     *bytes.Buffer
}

// AddLine appends a line to the goroutine info.
func (g *Goroutine) AddLine(l string) {
	if !g.freezed {
		g.lines++
		g.buf.WriteString(l)
		g.buf.WriteString("\n")
	}
}

// Freeze freezes the goroutine info.
func (g *Goroutine) Freeze() {
	if !g.freezed {
		g.freezed = true
		g.trace = g.buf.String()
		g.buf = nil
	}
}

// NewGoroutine creates and returns a new Goroutine.
func NewGoroutine(id int, metas map[MetaType]string) *Goroutine {
	return &Goroutine{
		id:    id,
		lines: 1,
		buf:   &bytes.Buffer{},
		metas: metas,
	}
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

// Delete deletes by the condition.
func (gd *GoroutineDump) Delete(cond string) error {
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

// Keep keeps by the condition.
func (gd *GoroutineDump) Keep(cond string) error {
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

// Search displays the goroutines with the offset and limit.
func (gd GoroutineDump) Search(cond string, offset, limit int) {
	_, err := gd.withCondition(cond, func(i int, g *Goroutine, passed bool) *Goroutine {
		if passed {
			fmt.Println(g.trace)
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
		fmt.Println(gd.goroutines[offset+i].trace)
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
		for k, v := range stats {
			fmt.Printf("%15s: %d\n", k, v)
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

func (gd *GoroutineDump) withCondition(cond string, callback func(int, *Goroutine, bool) *Goroutine) ([]*Goroutine, error) {
	cond = strings.Trim(cond, "\"")
	expression, err := govaluate.NewEvaluableExpression(cond)
	if err != nil {
		return nil, err
	}

	goroutines := make([]*Goroutine, 0, len(gd.goroutines))
	for i, g := range gd.goroutines {
		params := map[string]interface{}{
			"id":       g.id,
			"duration": g.metas[MetaDuration],
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
