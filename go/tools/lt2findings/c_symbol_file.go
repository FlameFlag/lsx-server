package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_c "github.com/tree-sitter/tree-sitter-c/bindings/go"
)

func scanCSymbolsInFile(path string, linked map[string]string) ([]cSymbol, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	parser := sitter.NewParser()
	defer parser.Close()
	if err := parser.SetLanguage(sitter.NewLanguage(tree_sitter_c.Language())); err != nil {
		return nil, fmt.Errorf("set C parser language: %w", err)
	}
	tree := parser.Parse(data, nil)
	if tree == nil {
		return nil, errors.New("parse C source: no syntax tree returned")
	}
	defer tree.Close()

	scanner := cSymbolScanner{
		path:    filepath.ToSlash(path),
		linked:  linked,
		symbols: []cSymbol{},
		seen:    map[string]bool{},
	}
	scanner.scanTranslationUnit(tree.RootNode(), data)
	return scanner.symbols, nil
}

type cSymbolScanner struct {
	path    string
	linked  map[string]string
	symbols []cSymbol
	seen    map[string]bool
}

func (s *cSymbolScanner) scanTranslationUnit(root *sitter.Node, data []byte) {
	if root == nil {
		return
	}
	for i := uint(0); i < root.NamedChildCount(); i++ {
		child := root.NamedChild(i)
		s.scanTopLevel(child, data)
	}
}

func (s *cSymbolScanner) scanTopLevel(node *sitter.Node, data []byte) {
	if node == nil {
		return
	}
	switch node.Kind() {
	case "function_definition":
		name := functionNameFromDeclarator(node.ChildByFieldName("declarator"), data)
		s.add("function", name, lineNo(node))
	case "declaration":
		s.scanGlobalDeclaration(node, data)
	case "type_definition":
		s.scanTypeDefinition(node, data)
	case "struct_specifier", "union_specifier", "enum_specifier":
		s.scanNamedTagType(node, data)
		if node.Kind() == "enum_specifier" {
			s.scanEnumValues(node, data)
		}
	case "preproc_def", "preproc_function_def":
		name := textOf(node.ChildByFieldName("name"), data)
		if name == "" {
			name = firstIdentifier(node, data)
		}
		s.add("macro", name, lineNo(node))
	case "preproc_if", "preproc_ifdef", "preproc_ifndef", "preproc_else":
		for i := uint(0); i < node.NamedChildCount(); i++ {
			s.scanTopLevel(node.NamedChild(i), data)
		}
	}
}

func (s *cSymbolScanner) scanGlobalDeclaration(node *sitter.Node, data []byte) {
	if hasChildKind(node, "ERROR") || hasStorageClass(node, "extern", data) {
		return
	}
	forEachDescendant(node, func(child *sitter.Node) bool {
		if child.Kind() != "init_declarator" {
			return true
		}
		name := identifierNameFromDeclarator(child.ChildByFieldName("declarator"), data)
		s.add("global", name, lineNo(child))
		return true
	})
}

func (s *cSymbolScanner) scanTypeDefinition(node *sitter.Node, data []byte) {
	name := identifierNameFromDeclarator(node.ChildByFieldName("declarator"), data)
	if name == "" {
		name = lastTypeIdentifier(node, data)
	}
	s.add("type", name, lineNo(node))
	s.scanEnumValues(node, data)
}

func (s *cSymbolScanner) scanNamedTagType(node *sitter.Node, data []byte) {
	name := textOf(node.ChildByFieldName("name"), data)
	s.add("type", name, lineNo(node))
}

func (s *cSymbolScanner) scanEnumValues(node *sitter.Node, data []byte) {
	forEachDescendant(node, func(child *sitter.Node) bool {
		if child.Kind() != "enumerator" {
			return true
		}
		name := textOf(child.ChildByFieldName("name"), data)
		if name == "" {
			name = firstIdentifier(child, data)
		}
		s.add("enum", name, lineNo(child))
		return true
	})
}

func (s *cSymbolScanner) add(kind, name string, line int) {
	if name == "" || isControlKeyword(name) {
		return
	}
	key := kind + "\x00" + name
	if s.seen[key] {
		return
	}
	s.seen[key] = true
	s.symbols = append(s.symbols, cSymbol{
		Kind:      kind,
		Name:      name,
		Source:    s.path,
		Line:      line,
		FindingID: s.linked[name],
	})
}

func functionNameFromDeclarator(node *sitter.Node, data []byte) string {
	if node == nil {
		return ""
	}
	if node.Kind() == "function_declarator" {
		return identifierNameFromDeclarator(node.ChildByFieldName("declarator"), data)
	}
	for i := uint(0); i < node.NamedChildCount(); i++ {
		if name := functionNameFromDeclarator(node.NamedChild(i), data); name != "" {
			return name
		}
	}
	return ""
}

func identifierNameFromDeclarator(node *sitter.Node, data []byte) string {
	if node == nil {
		return ""
	}
	switch node.Kind() {
	case "identifier", "type_identifier":
		return textOf(node, data)
	case "function_declarator":
		return identifierNameFromDeclarator(node.ChildByFieldName("declarator"), data)
	case "parenthesized_declarator", "pointer_declarator", "array_declarator",
		"init_declarator":
		if declarator := node.ChildByFieldName("declarator"); declarator != nil {
			return identifierNameFromDeclarator(declarator, data)
		}
	}
	for i := uint(0); i < node.NamedChildCount(); i++ {
		if name := identifierNameFromDeclarator(node.NamedChild(i), data); name != "" {
			return name
		}
	}
	return ""
}

func lastTypeIdentifier(node *sitter.Node, data []byte) string {
	name := ""
	forEachDescendant(node, func(child *sitter.Node) bool {
		switch child.Kind() {
		case "type_identifier", "identifier":
			name = textOf(child, data)
		}
		return true
	})
	return name
}

func firstIdentifier(node *sitter.Node, data []byte) string {
	if node == nil {
		return ""
	}
	switch node.Kind() {
	case "identifier", "type_identifier":
		return textOf(node, data)
	}
	for i := uint(0); i < node.NamedChildCount(); i++ {
		if name := firstIdentifier(node.NamedChild(i), data); name != "" {
			return name
		}
	}
	return ""
}

func hasStorageClass(node *sitter.Node, value string, data []byte) bool {
	found := false
	forEachDescendant(node, func(child *sitter.Node) bool {
		if child.Kind() == "storage_class_specifier" && textOf(child, data) == value {
			found = true
			return false
		}
		return true
	})
	return found
}

func hasChildKind(node *sitter.Node, kind string) bool {
	found := false
	forEachDescendant(node, func(child *sitter.Node) bool {
		if child.Kind() == kind {
			found = true
			return false
		}
		return true
	})
	return found
}

func forEachDescendant(node *sitter.Node, visit func(*sitter.Node) bool) bool {
	if node == nil {
		return true
	}
	if !visit(node) {
		return false
	}
	for i := uint(0); i < node.NamedChildCount(); i++ {
		if !forEachDescendant(node.NamedChild(i), visit) {
			return false
		}
	}
	return true
}

func textOf(node *sitter.Node, data []byte) string {
	if node == nil {
		return ""
	}
	start := node.StartByte()
	end := node.EndByte()
	if start > end || end > uint(len(data)) {
		return ""
	}
	return string(data[start:end])
}

func lineNo(node *sitter.Node) int {
	if node == nil {
		return 0
	}
	return int(node.StartPosition().Row) + 1
}
