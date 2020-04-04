package main

import (
	"regexp"

	dto "github.com/prometheus/client_model/go"
)

//Rule .
type Rule struct {
	Pattern           string            `yaml:"pattern"`
	Name              string            `yaml:"name"`
	Help              string            `yaml:"help"`
	AttrNameSnakeCase bool              `yaml:"attrNameSnakeCase"`
	Type              string            `yaml:"type"`
	Labels            map[string]string `yaml:"labels"`
	ValueFactor       float64           `yaml:"valueFactor"`
	Value             string            `yaml:"value"`
	LabelNames        []string
	LabelValues       []string
	PatternReg        *regexp.Regexp
}



//Config .
type Config struct {
	StartDelaySeconds         int64    `yaml:"startDelaySeconds"`
	LowercaseOutputName       bool     `yaml:"lowercaseOutputName"`
	LowercaseOutputLabelNames bool     `yaml:"lowercaseOutputLabelNames"`
	Rules                     []*Rule  `yaml:"rules"`
	WhitelistObjectNames      []string `yaml:"whitelistObjectNames"`
	BlacklistObjectNames      []string `yaml:"blacklistObjectNames"`
	Endpointport               string  `yaml:"endpointport"`
	WhitelistObjectNamesReg   []*regexp.Regexp
	BlacklistObjectNamesReg   []*regexp.Regexp
}

//JmxCollector .
type JmxCollector struct {
	metricFamilies       map[string]*dto.MetricFamily
	keyPropertiesPerBean map[string][]map[string]string
}
