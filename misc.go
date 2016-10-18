package candyjs

import (
	"reflect"
	"strings"
)

func isExported(name string) bool {
	return nameToJavaScript(name) != name
}

func nameToJavaScript(name string) string {
	var toLower, keep string
	for _, c := range name {
		if c >= 'A' && c <= 'Z' && len(keep) == 0 {
			toLower += string(c)
		} else {
			keep += string(c)
		}
	}

	lc := len(toLower)
	if lc > 1 && lc != len(name) {
		keep = toLower[lc-1:] + keep
		toLower = toLower[:lc-1]

	}

	return strings.ToLower(toLower) + keep
}

func nameToGo(name string) []string {
	if name[0] >= 'A' && name[0] <= 'Z' {
		return nil
	}

	var toUpper, keep string
	for _, c := range name {
		if c >= 'a' && c <= 'z' && len(keep) == 0 {
			toUpper += string(c)
		} else {
			keep += string(c)
		}
	}

	return []string{
		strings.Title(name),
		strings.ToUpper(toUpper) + keep,
	}
}

func nameToFieldName(t reflect.Type, name string) string {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	} else if t.Kind() != reflect.Struct {
		return ""
	}

	// find a field matching "json" field tag
	if hasJsonTags(t) {
		return nameToFieldNameByJsonTag(t, name)
	}

	// do a fuzzy search using "tcpPort" => ["TcpPort", "TCPPort"]
	for _, goName := range nameToGo(name) {
		if f, ok := t.FieldByName(goName); ok {
			return f.Name
		}
	}
	return ""
}

func hasJsonTags(t reflect.Type) bool {
	for i := 0; i < t.NumField(); i++ {
		if t.Field(i).Tag.Get("json") != "" {
			return true
		}
	}
	return false
}

func nameToFieldNameByJsonTag(t reflect.Type, name string) string {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !isExported(f.Name) {
			// ignore
		} else if f.Anonymous {
			if n := nameToFieldName(f.Type, name); n != "" { // <-- recursive
				return n
			}
		} else {
			n := parseJsonTag(f.Tag.Get("json"))
			if (n != "" && n == name) || (n == "" && name == f.Name) {
				return f.Name
			}
		}
	}
	return ""
}

func fieldToName(f reflect.StructField) string {
	if !isExported(f.Name) {
		return ""
	}
	name := parseJsonTag(f.Tag.Get("json"))
	if name == "" {
		name = nameToJavaScript(f.Name)
	}
	return name
}

// visitFields visits all fields of a type including fields of nested types (recursively).
func visitFields(t reflect.Type, fn func(f reflect.StructField) bool) bool {
	switch t.Kind() {
	case reflect.Ptr:
		t = t.Elem()
	case reflect.Struct:
		// do nothing
	default:
		return false
	}

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if isExported(f.Name) {
			if f.Anonymous {
				if visitFields(f.Type, fn) {
					return true
				}
			} else if fn(f) {
				return true
			}
		}
	}
	return false
}

// parseJsonTag parses the name part out of a json tag
func parseJsonTag(tag string) string {
	if tag == "-" {
		return ""
	}
	if idx := strings.Index(tag, ","); idx != -1 {
		return tag[:idx]
	}
	return tag
}
