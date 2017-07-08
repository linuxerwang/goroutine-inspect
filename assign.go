package main

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"regexp"
	"strings"
)

var (
	identifierPattern = regexp.MustCompile("[_a-zA-Z][_a-zA-Z0-9]*")
)

func assign(cmd string) error {
	if idx := strings.Index(cmd, "="); idx > 0 {
		k := strings.TrimSpace(cmd[:idx])
		if k == "" {
			return errors.New("incomplete assignment")
		}
		if !identifierPattern.MatchString(k) {
			return fmt.Errorf("invalid variable name %s", k)
		}

		v := strings.TrimSpace(cmd[idx+1:])
		if v == "" {
			return errors.New("incomplete assignment")
		}

		ex, err := parser.ParseExpr(v)
		if err != nil {
			return err
		}

		switch ex := ex.(type) {
		case *ast.CallExpr:
			switch fun := ex.Fun.(type) {
			case *ast.SelectorExpr:
				s := fun.X.(*ast.Ident).Name
				if val, ok := workspace[s]; ok {
					switch fun.Sel.Name {
					case "copy":
						if len(ex.Args) > 1 {
							return errors.New("copy expects zero or one argument")
						}
						if len(ex.Args) == 0 {
							workspace[k] = val.Copy("")
						} else {
							workspace[k] = val.Copy(ex.Args[0].(*ast.BasicLit).Value)
						}
					default:
						return fmt.Errorf("%s.%s() is not allowed for assigning to a variable", k, fun.Sel.Name)
					}
				}
			case *ast.Ident:
				if fun.Name == "load" {
					if len(ex.Args) != 1 {
						return errors.New("load() expects exactly one argument")
					}
					dump, err := load(ex.Args[0].(*ast.BasicLit).Value)
					if err != nil {
						return err
					}
					workspace[k] = dump
					dump.Summary()
				} else {
					return fmt.Errorf("unknown instrution %s", fun.Name)
				}
			default:
				return fmt.Errorf("unknown instrution")
			}
		case *ast.Ident:
			if v, ok := workspace[ex.String()]; ok {
				workspace[k] = v.Copy("")
			} else {
				return fmt.Errorf("variable %s not found in workspace", ex.String())
			}
		default:
			return errors.New("unknown instrution")
		}
	}
	return nil
}
