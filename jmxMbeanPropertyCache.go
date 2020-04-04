package main

import "regexp"

var (
	propertyPattern *regexp.Regexp
	colonPattern    *regexp.Regexp
	equalPattern    *regexp.Regexp
)

func init() {
	propertyPattern = regexp.MustCompile(
		"([^,=:\\*\\?]+)" + // Name - non-empty, anything but comma, equals, colon, star, or question mark
			"=" + // Equals
			"(" + // Either
			"\"" + // Quoted
			"(?:" + // A possibly empty sequence of
			"[^\\\\\"]*" + // Greedily match anything but backslash or quote
			"(?:\\\\.)?" + // Greedily see if we can match an escaped sequence
			")*" +
			"\"" +
			"|" + // Or
			"[^,=:\"]*" + // Unquoted - can be empty, anything but comma, equals, colon, or quote
			")")
	colonPattern = regexp.MustCompile(":")
	equalPattern = regexp.MustCompile("=")
}

//change to []map[string]string from map[string]string
//simulate LinkedHashMap in java
func (jc *JmxCollector) getKeyPropertyList(mbeanName string) []map[string]string {
	properties, exist := jc.keyPropertiesPerBean[mbeanName]
	if !exist {
		properties = []map[string]string{}
		domainAndProperties := colonPattern.Split(mbeanName, 2)
		propertiesStr := propertyPattern.FindAllString(domainAndProperties[1], -1)
		for _, property := range propertiesStr {
			kvs := equalPattern.Split(property, 2)
			kvMap := make(map[string]string)
			kvMap[kvs[0]] = kvs[1]
			properties = append(properties, kvMap)
		}
		jc.keyPropertiesPerBean[mbeanName] = properties
	}
	return properties
}
