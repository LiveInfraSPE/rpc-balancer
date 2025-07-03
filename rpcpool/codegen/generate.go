package codegen

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"golang.org/x/tools/go/packages"
)

const targetType = "Client"

type Method struct {
	Name                   string
	Params                 string
	Results                string
	ResultCount            int
	CallParams             string
	HasReturn              bool
	HasError               bool
	HasNamedReturns        bool
	ReturnValues           string
	ErrorValue             string
	NonErrorReturnValues   string
	NonErrorResultCount    int
	ZeroResults            string
	ZeroResultsNonError    string
	ReturnNames            []string
	NonErrorReturnNames    []string
	ResultTypes            []string
	ResultAssignments      []string
	ResultAssignmentsCount int
}

func GenerateEthClient(clientPkg string, source_file string, wrapperTemplate string, outputFile string) {
	sourceFile, err := findClientSource(clientPkg, source_file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to find ethclient source: %v\n", err)
		os.Exit(1)
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, sourceFile, nil, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse %s: %v\n", sourceFile, err)
		os.Exit(1)
	}

	var methods []Method
	ast.Inspect(file, func(n ast.Node) bool {
		typeSpec, ok := n.(*ast.TypeSpec)
		if !ok || typeSpec.Name.Name != targetType {
			return true
		}

		for _, decl := range file.Decls {
			funcDecl, ok := decl.(*ast.FuncDecl)
			if !ok || funcDecl.Recv == nil {
				continue
			}

			recv := funcDecl.Recv.List[0]
			starExpr, ok := recv.Type.(*ast.StarExpr)
			if !ok || starExpr.X.(*ast.Ident).Name != targetType {
				continue
			}

			name := funcDecl.Name.Name
			if !funcDecl.Name.IsExported() || name == "Close" || name == "Client" {
				continue
			}

			results, hasError := formatResults(funcDecl.Type.Results)
			m := Method{
				Name:       name,
				Params:     formatParams(funcDecl.Type.Params),
				Results:    results,
				HasError:   hasError,
				CallParams: formatCallParams(funcDecl.Type.Params),
			}

			if funcDecl.Type.Results != nil && len(funcDecl.Type.Results.List) > 0 {
				m.HasReturn = true
				returnValues := make([]string, 0)
				returnNames := make([]string, 0)
				resultTypes := make([]string, 0)
				nonErrorReturnNames := make([]string, 0)
				resultAssignments := make([]string, 0)
				hasNamedReturns := true

				for i, field := range funcDecl.Type.Results.List {
					typ := formatType(field.Type)
					var name string
					if len(field.Names) > 0 {
						name = field.Names[0].Name
					} else {
						name = fmt.Sprintf("r%d", i)
						hasNamedReturns = false
					}
					returnNames = append(returnNames, name)
					if typ == "error" {
						m.ErrorValue = name
					} else {
						resultTypes = append(resultTypes, typ)
						nonErrorReturnNames = append(nonErrorReturnNames, name)
					}
					returnValues = append(returnValues, name)
				}
				m.HasNamedReturns = hasNamedReturns
				m.ReturnValues = strings.Join(returnValues, ", ")
				m.NonErrorReturnValues = strings.Join(nonErrorReturnNames, ", ")
				m.NonErrorResultCount = len(resultTypes)
				if m.HasError {
					m.ResultCount = m.NonErrorResultCount + 1
				} else {
					m.ResultCount = m.NonErrorResultCount
				}
				m.ZeroResultsNonError = formatZeroResultsForTypes(resultTypes)
				if m.HasError {
					m.ZeroResults = m.ZeroResultsNonError + ", nil"
				} else {
					m.ZeroResults = m.ZeroResultsNonError
				}
				m.ReturnNames = returnNames
				m.NonErrorReturnNames = nonErrorReturnNames
				m.ResultTypes = resultTypes

				for i, field := range funcDecl.Type.Results.List {
					typ := formatType(field.Type)
					var name string
					if len(field.Names) > 0 {
						name = field.Names[0].Name
					} else {
						name = fmt.Sprintf("r%d", i)
						hasNamedReturns = false
					}
					if typ == "error" {
						continue
					}
					assignmentOp := "="
					if !hasNamedReturns {
						assignmentOp = ":="
					}
					if m.HasError {
						resultAssignments = append(resultAssignments, fmt.Sprintf("%s, ok %s results[%d].(%s); if !ok { return %s, fmt.Errorf(\"invalid type for %s\") }", name, assignmentOp, i, typ, m.ZeroResultsNonError, name))
					} else {
						resultAssignments = append(resultAssignments, fmt.Sprintf("%s, ok %s results[%d].(%s); if !ok { return %s }", name, assignmentOp, i, typ, m.ZeroResults))
					}
				}
				m.ResultAssignments = resultAssignments
				m.ResultAssignmentsCount = len(resultAssignments)
			} else {
				m.ZeroResults = "nil"
			}

			methods = append(methods, m)
		}
		return false
	})

	// Generate code using template
	tmpl, err := template.New("wrapper").Funcs(template.FuncMap{
		"gt": func(a, b int) bool { return a > b },
	}).Parse(wrapperTemplate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse template: %v\n", err)
		os.Exit(1)
	}

	data := struct{ Methods []Method }{Methods: methods}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		fmt.Fprintf(os.Stderr, "failed to execute template: %v\n", err)
		os.Exit(1)
	}

	// Write to output file
	if err := os.WriteFile(outputFile, buf.Bytes(), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write output: %v\n", err)
		os.Exit(1)
	}
}

func findClientSource(clientPkg string, code_file string) (string, error) {
	cfg := &packages.Config{
		Mode: packages.NeedFiles,
	}
	pkgs, err := packages.Load(cfg, clientPkg)
	if err != nil {
		return "", fmt.Errorf("failed to load package %s: %w", clientPkg, err)
	}

	if len(pkgs) != 1 {
		return "", fmt.Errorf("expected 1 package for %s, got %d", clientPkg, len(pkgs))
	}

	pkg := pkgs[0]
	for _, file := range pkg.GoFiles {
		if filepath.Base(file) == code_file {
			return file, nil
		}
	}

	return "", fmt.Errorf("%s not found in package %s", code_file, clientPkg)
}

func formatParams(fields *ast.FieldList) string {
	if fields == nil {
		return ""
	}
	var parts []string
	for i, field := range fields.List {
		names := make([]string, len(field.Names))
		for j, name := range field.Names {
			names[j] = name.Name
		}
		if len(names) == 0 {
			names = []string{fmt.Sprintf("arg%d", i)}
		}
		typ := formatType(field.Type)
		parts = append(parts, fmt.Sprintf("%s %s", strings.Join(names, ","), typ))
	}
	return strings.Join(parts, ", ")
}

func formatResults(fields *ast.FieldList) (string, bool) {
	hasError := false
	if fields == nil {
		return "", hasError
	}
	var parts []string
	for _, field := range fields.List {
		typ := formatType(field.Type)
		if typ == "error" {
			hasError = true
		}
		if len(field.Names) > 0 {
			parts = append(parts, fmt.Sprintf("%s %s", field.Names[0].Name, typ))
		} else {
			parts = append(parts, typ)
		}
	}
	return strings.Join(parts, ", "), hasError
}

func formatCallParams(fields *ast.FieldList) string {
	if fields == nil {
		return ""
	}
	var parts []string
	for i, field := range fields.List {
		if len(field.Names) > 0 {
			for _, name := range field.Names {
				switch field.Type.(type) {
				case *ast.Ellipsis:
					parts = append(parts, fmt.Sprintf("%s...", name.Name))
				default:
					parts = append(parts, name.Name)
				}
			}
		} else {
			parts = append(parts, fmt.Sprintf("arg%d", i))
		}
	}
	return strings.Join(parts, ", ")
}

func formatZeroResultsForTypes(types []string) string {
	var parts []string
	for _, typ := range types {
		switch {
		case typ == "string":
			parts = append(parts, `""`)
		case typ == "int", typ == "uint64", typ == "int64", typ == "uint32", typ == "int32", typ == "uint16", typ == "int16", typ == "uint8", typ == "int8", typ == "uint":
			parts = append(parts, "0")
		case typ == "bool":
			parts = append(parts, "false")
		case strings.HasPrefix(typ, "*"):
			parts = append(parts, "nil")
		case strings.HasPrefix(typ, "ethereum"):
			parts = append(parts, "nil")
		default:
			parts = append(parts, fmt.Sprintf("%s{}", typ))
		}
	}
	return strings.Join(parts, ", ")
}

func formatType(expr ast.Expr) string {
	var typ string
	switch t := expr.(type) {
	case *ast.Ident:
		typ = t.Name
	case *ast.StarExpr:
		typ = "*" + formatType(t.X)
	case *ast.ArrayType:
		typ = "[]" + formatType(t.Elt)
	case *ast.SelectorExpr:
		typ = formatType(t.X) + "." + t.Sel.Name
	case *ast.ChanType:
		var dir string
		switch t.Dir {
		case ast.SEND:
			dir = "chan<- "
		case ast.RECV:
			dir = "<-chan "
		default:
			dir = "chan "
		}
		typ = dir + formatType(t.Value)
	case *ast.InterfaceType:
		typ = "interface{}"
	case *ast.MapType:
		typ = fmt.Sprintf("map[%s]%s", formatType(t.Key), formatType(t.Value))
	case *ast.FuncType:
		params := formatParams(t.Params)
		results, _ := formatResults(t.Results)
		if results == "" {
			typ = fmt.Sprintf("func(%s)", params)
		}
		typ = fmt.Sprintf("func(%s) %s", params, results)
	case *ast.Ellipsis:
		typ = "..."
		if t.Elt != nil {
			typ = "..." + formatType(t.Elt)
		}

	default:
		typ = fmt.Sprintf("%v", expr)
	}

	switch typ {
	case "[]BatchElem":
		typ = "[]rpc.BatchElem"
	case "*ClientSubscription":
		typ = "*rpc.ClientSubscription"
	}

	return typ
}
