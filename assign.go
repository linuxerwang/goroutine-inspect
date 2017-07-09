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
	identifierPattern  = regexp.MustCompile("[_a-zA-Z][_a-zA-Z0-9]*")
	identifiersPattern = regexp.MustCompile("[_a-zA-Z][_a-zA-Z0-9]*(\\s*,\\s*[_a-zA-Z][_a-zA-Z0-9]*)*\\s*")
)

func assign(cmd string) error {
	if idx := strings.Index(cmd, "="); idx > 0 {
		k := strings.TrimSpace(cmd[:idx])
		if k == "" {
			return errors.New("incomplete assignment")
		}
		if !identifiersPattern.MatchString(k) {
			if !identifierPattern.MatchString(k) {
				return fmt.Errorf("invalid variable name %s", k)
			}
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
					case "diff":
						if len(ex.Args) != 1 {
							return errors.New("diff() expects exactly one argument")
						}
						args := strings.Split(k, ",")
						if len(args) == 0 || len(args) > 3 {
							return errors.New("diff() expects at least one and at most 3 result receiver")
						}
						varName := strings.TrimSpace(ex.Args[0].(*ast.Ident).Name)
						if val, ok := workspace[varName]; ok {
							if v, ok := workspace[s]; ok {
								lonly, common, ronly := v.Diff(val)
								if len(args) >= 1 {
									workspace[strings.TrimSpace(args[0])] = lonly
								}
								if len(args) >= 2 {
									workspace[strings.TrimSpace(args[1])] = common
								}
								if len(args) == 3 {
									workspace[strings.TrimSpace(args[2])] = ronly
								}
							} else {
								return fmt.Errorf("variable %s not found in workspace", s)
							}
						} else {
							return fmt.Errorf("variable %s not found in workspace", varName)
						}
						return nil
					default:
						return fmt.Errorf("%s.%s() is not allowed for assigning to a variable", k, fun.Sel.Name)
					}
				} else {
					return fmt.Errorf("variable %s not found in workspace", s)
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
