package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"unicode"
)

// [] and () are special in regexes, so swtich to <>.
func angleBrackets(s string) string {
	return "<" + s[1:len(s)-1] + ">"
}

func toSnakeAndLowerCase(attrName string) string {
	if len(attrName) == 0 {
		return attrName
	}
	firstChar := attrName[0]
	prevCharIsUpperCaseOrUnderscore := unicode.IsUpper(rune(firstChar)) || firstChar == '_'
	var buf bytes.Buffer
	buf.WriteByte(byte(unicode.ToLower(rune(firstChar))))
	for _, attrChar := range attrName[1:] {
		charIsUpperCase := unicode.IsUpper(rune(attrChar))
		if !prevCharIsUpperCaseOrUnderscore && charIsUpperCase {
			buf.WriteByte('_')
		}
		buf.WriteByte(byte(unicode.ToLower(rune(attrChar))))
		prevCharIsUpperCaseOrUnderscore = charIsUpperCase || attrChar == '_'
	}
	return buf.String()
}

func map2string(m map[string]string) string {
	ks := []string{}
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	var b bytes.Buffer
	b.WriteByte('{')
	for i, k := range ks {
		if i > 0 {
			b.WriteByte(',')
			b.WriteByte(' ')
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(m[k])
	}
	b.WriteByte('}')
	return b.String()
}

func slicemap2string(slicem []map[string]string) string {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, m := range slicem {
		if i > 0 {
			b.WriteByte(',')
			b.WriteByte(' ')
		}
		//only one key and one value
		for k, v := range m {
			b.WriteString(k)
			b.WriteByte('=')
			b.WriteString(v)
		}
	}
	b.WriteByte('}')
	return b.String()
}

func slice2string(s []string) string {
	var b bytes.Buffer
	b.WriteByte('[')
	for i, v := range s {
		if i > 0 {
			b.WriteByte(',')
			b.WriteByte(' ')
		}
		b.WriteString(v)
	}
	b.WriteByte(']')
	return b.String()
}

func safeName(name string) string {
	if len(name) == 0 {
		return name
	}
	prevCharIsUnderscore := false
	var buf bytes.Buffer
	if unicode.IsDigit(rune(name[0])) {
		// prevent a numeric prefix.
		buf.WriteByte('_')
	}
	for _, c := range name {
		isUnsafeChar := !(unicode.IsDigit(c) || unicode.IsLetter(c) || c == ':' || c == '_')
		if isUnsafeChar || c == '_' {
			if prevCharIsUnderscore {
				continue
			} else {
				buf.WriteByte('_')
				prevCharIsUnderscore = true
			}
		} else {
			buf.WriteByte(byte(c))
			prevCharIsUnderscore = false
		}
	}
	return buf.String()
}

func obj2string(obj interface{}) string {
	switch t := obj.(type) {
	case int64:
		return fmt.Sprintf("%d", t)
	case float64:
		return fmt.Sprintf("%f", t)
	case bool:
		if t {
			return "true"
		}
		return "false"
	case string:
		return t
	}
	return ""
}

func obj2float64(beanValue interface{}) (float64, bool) {
	switch t := beanValue.(type) {
	case int64, float64:
		var valFloat64 float64
		var ok bool
		if valFloat64, ok = t.(float64); !ok {
			return valFloat64, false
		}
		return valFloat64, true
	case bool:
		if t {
			return 1.0, true
		}
		return 0.0, true
	}
	return 0.0, false
}

func isString(obj interface{}) bool {
	switch obj.(type) {
	case string:
		return true
	}
	return false
}

func containsKey(slicem []map[string]string, key string) bool {
	for _, m := range slicem {
		for k := range m {
			if k == "key" {
				return true
			}
		}
	}
	return false
}

func printJSON(o interface{}) {
	pod, err := json.MarshalIndent(o, "", "  ")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(string(pod))
}
