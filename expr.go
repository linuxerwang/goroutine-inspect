package main

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"reflect"
	"strconv"
)

func expr(e string) error {
	ex, err := parser.ParseExpr(e)
	if err != nil {
		return err
	}

	switch ex := ex.(type) {
	case *ast.CallExpr:
		switch fun := ex.Fun.(type) {
		case *ast.SelectorExpr:
			k := fun.X.(*ast.Ident).Name
			if v, ok := workspace[k]; ok {
				switch fun.Sel.Name {
				case "delete":
					if len(ex.Args) != 1 {
						return errors.New("keep() expects exactly one argument")
					}
					return v.Delete(ex.Args[0].(*ast.BasicLit).Value)
				case "dedup":
					if len(ex.Args) != 0 {
						return errors.New("dedup() expects no arguments")
					}
					v.Dedup()
					return nil
				case "keep":
					if len(ex.Args) != 1 {
						return errors.New("delete() expects exactly one argument")
					}
					return v.Keep(ex.Args[0].(*ast.BasicLit).Value)
				case "search":
					var err error
					offset := 0
					limit := 10
					switch len(ex.Args) {
					case 0:
						return errors.New("search() expects at least one argument")
					case 1:
					case 2:
						offset, err = strconv.Atoi(ex.Args[1].(*ast.BasicLit).Value)
						if err != nil {
							return fmt.Errorf("invalid argument %s", ex.Args[1])
						}
					case 3:
						offset, err = strconv.Atoi(ex.Args[1].(*ast.BasicLit).Value)
						if err != nil {
							return fmt.Errorf("invalid argument 'offset' %s", ex.Args[1])
						}
						limit, err = strconv.Atoi(ex.Args[2].(*ast.BasicLit).Value)
						if err != nil {
							return fmt.Errorf("invalid argument 'limit' %s", ex.Args[2])
						}
					default:
						return errors.New("search() expects at most three arguments")
					}
					v.Search(ex.Args[0].(*ast.BasicLit).Value, offset, limit)
					return nil
				case "show":
					var err error
					offset := 0
					limit := 10
					switch len(ex.Args) {
					case 0:
					case 1:
						offset, err = strconv.Atoi(ex.Args[0].(*ast.BasicLit).Value)
						if err != nil {
							return fmt.Errorf("invalid argument %s", ex.Args[0])
						}
					case 2:
						offset, err = strconv.Atoi(ex.Args[0].(*ast.BasicLit).Value)
						if err != nil {
							return fmt.Errorf("invalid argument 'offset' %s", ex.Args[0])
						}
						limit, err = strconv.Atoi(ex.Args[1].(*ast.BasicLit).Value)
						if err != nil {
							return fmt.Errorf("invalid argument 'limit' %s", ex.Args[1])
						}
					default:
						return errors.New("show() expects at least one and at most two arguments")
					}
					v.Show(offset, limit)
					return nil
				}
			}
			fmt.Println(reflect.TypeOf(ex.Args[0]))
		default:
			return fmt.Errorf("unknown instrution")
		}
	case *ast.Ident:
		if v, ok := workspace[ex.String()]; ok {
			v.Summary()
		} else {
			return fmt.Errorf("variable %s not found in workspace", e)
		}
	default:
		return fmt.Errorf("unknown instrution")
	}

	return nil
}
