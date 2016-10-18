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

func nameToFieldName(v reflect.Value, name string) string {
	hasJsonTags := false
	fieldName := ""
	// find a field matching "json" field tag
	for i := 0; i < v.NumField(); i++ {
		f := v.Type().Field(i)
		if !isExported(f.Name) {
			continue
		}
		jsonName := parseJsonTag(f.Tag.Get("json"))
		if jsonName == name {
			return f.Name
		} else if jsonName != "" {
			hasJsonTags = true
		} else if f.Name == name {
			fieldName = name
		}
	}
	if !hasJsonTags {
		// do a fuzzy search using "tcpPort" => ["TcpPort", "TCPPort"]
		for _, goName := range nameToGo(name) {
			f := v.FieldByName(goName)
			if f.IsValid() {
				return goName
			}
		}
		return ""
	}
	return fieldName
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
