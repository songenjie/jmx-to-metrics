package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"os"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	dto "github.com/prometheus/client_model/go"
)

func main() {
	flag.Parse()
	if h {
		flag.Usage()
		return
	}
	_, err := os.Stat(workdir)
	if err != nil {
		log.Println("work dir not Exists")
		return
	}
	if fetchip =="" ||fetchip == "127.0.0.1" || fetchip == "0.0.0.0" {
		fetchip = realip
	}else {
		realip  =fetchip
	}
	log.Println("http://"+configserverurl+"/api/registerPort?ip="+realip+"&promeType=jmx_parser&port="+listenport)
	body,err := request("http://"+configserverurl+"/api/registerPort?ip="+realip+"&promeType=jmx_parser&port="+listenport, "GET")
	if err != nil {
			log.Println("register port faild!")
			return
	}
	if string(body) != "registerPort success!\n" {
		log.Println("register port faild !")
		return
	}

    //获取抓取间隔参数
	parametime, err :=strconv.ParseInt(updateinterval, 10, 64)
	if err != nil{
		log.Println(err)
		return
	}
	ticker:=time.NewTicker(time.Minute * time.Duration(parametime) )

	updateyaml()
	go func(){
		for{
			select{
			case _ = <-ticker.C:
				updateyaml()
			}
		}
	}()

	var addr = flag.String("listen-address", ":"+listenport, "The address to listen on for HTTP requests.")
	flag.Parse()
	// Inject the metric families returned by ms.GetMetricFamilies into the default Gatherer:
	http.Handle("/metrics", promhttp.HandlerFor(prometheus.Gatherers{
		//prometheus.DefaultGatherer,
		prometheus.GathererFunc(func() ([]*dto.MetricFamily, error) {
			return GetMetricFamilies(), nil
		}),
	},
		promhttp.HandlerOpts{
			ErrorHandling: promhttp.ContinueOnError,
		},
	))

	//提供http reload 加载yaml功能
	http.HandleFunc("/reload/", yamlreload)
	log.Fatal(http.ListenAndServe(*addr, nil))
}

func usage() {
	fmt.Fprintf(os.Stderr, `jmx_parser  version: jmx_parser/0.0.3
Usage: jmx_parser [-hvHV] [- l] [-w] [-f] [-c] [-u]

Options:
`)
	flag.PrintDefaults()
}


func yamlreload(w http.ResponseWriter, r *http.Request) {
	defer    r.Body.Close()
	updateyaml()
	fmt.Fprint(w, "success")
}
func (jc *JmxCollector) processBeanValue(domain string, beanProperties []map[string]string, attrKeys []string, attrName string, value interface{},config Config,wg *sync.WaitGroup) {
	switch t := value.(type) {
	case nil:
	case int64, float64, string, bool:
		jc.recordBean(domain, beanProperties, attrKeys, attrName, value,config)
	case map[string]interface{}:
		newAttrKeys := make([]string, len(attrKeys)+1)
		copy(newAttrKeys, attrKeys)
		newAttrKeys[len(attrKeys)] = attrName
		//printJSON(newAttrKeys)
		for key, val := range t {
			wg.Add(1)
			go jc.processBeanValue(domain, beanProperties, newAttrKeys, key, val,config,wg)
		}
	case []interface{}:
		for _, val := range t {
			switch tt := val.(type) {
			case map[string]interface{}:
				l2s := []map[string]string{}
				for _, propertiesmap := range beanProperties {
					m := make(map[string]string)
					for k, v := range propertiesmap {
						m[k] = v
					}
					l2s = append(l2s, m)
				}
				keyValue := tt["key"]
				switch kval := keyValue.(type) {
				case string:
					// Nested tabulardata will repeat the 'key' label, so
					// append a suffix to distinguish each.
					if containsKey(l2s, "key") {
						m := make(map[string]string)
						m["key_"] = kval
						l2s = append(l2s, m)
					} else {
						m := make(map[string]string)
						m["key"] = kval
						l2s = append(l2s, m)
					}
				}
				wg.Add(1)
				go jc.processBeanValue(domain, l2s, attrKeys, attrName, tt["value"],config,wg)
			default:
				//not a correct tabulardata formats
			}
		}
	default:
	}
	wg.Done()
}

func (jc *JmxCollector) recordBean(domain string, beanProperties []map[string]string, attrKeys []string, attrName string, beanValue interface{},config Config) {
	beanName := domain + angleBrackets(slicemap2string(beanProperties)) + angleBrackets(slice2string(attrKeys))
	attrNameSnakeCase := toSnakeAndLowerCase(attrName)
	for _, rule := range config.Rules {
		var matchName string
		var metricName string
		var value float64
		labelsMap := make(map[string]string)
		if rule.AttrNameSnakeCase {
			matchName = beanName + toSnakeAndLowerCase(attrName)
		} else {
			matchName = beanName + attrName
		}
		if rule.PatternReg != nil {
			beforePattern := matchName + ": " + obj2string(beanValue)
			find := rule.PatternReg.MatchString(beforePattern)
			if !find {
				continue
			}
			if len(rule.Value) > 0 {
				var err error
				val := rule.PatternReg.ReplaceAllString(beforePattern, rule.Value)
				beanValue, err = strconv.ParseFloat(val, 64)
				if err != nil {
					return
				}
			}
			_, ok := obj2float64(beanValue)
			if !ok {
				return
			}
			// validation in the constructor.
			if len(rule.Name) > 0 {
				metricName = safeName(rule.PatternReg.ReplaceAllString(beforePattern, rule.Name))
				if len(metricName) == 0 {
					return
				}
				if config.LowercaseOutputName {
					metricName = strings.ToLower(metricName)
				}
				labelsMap["__name__"] = metricName
			}
			// Set the labels.
			if len(rule.LabelNames) > 0 {
				for i, unsafeLabelName := range rule.LabelNames {
					labelValReplacement := rule.LabelValues[i]
					labelName := safeName(rule.PatternReg.ReplaceAllString(beforePattern, unsafeLabelName))
					labelValue := rule.PatternReg.ReplaceAllString(beforePattern, labelValReplacement)
					if config.LowercaseOutputLabelNames {
						labelName = strings.ToLower(labelName)
					}
					if len(labelName) > 0 && len(labelValue) > 0 {
						labelsMap[labelName] = labelValue
					}
				}
			}
		}
		bVal, ok := obj2float64(beanValue)
		if !ok {
			return
		}
		value = bVal * rule.ValueFactor
		// If there's no name provided, use default export format.
		if len(rule.Name) == 0 {
			if rule.AttrNameSnakeCase {
				jc.defaultExport(domain, beanProperties, attrKeys, attrNameSnakeCase, value,config)
				return
			}
			jc.defaultExport(domain, beanProperties, attrKeys, attrName, value,config)
			return
		}
		//printJSON(labelsMap)
		labelPars := []*dto.LabelPair{}
		for k, v := range labelsMap {
			if k == "__name__" {
				continue
			}
			name := k
			value := v
			labelPars = append(labelPars, &dto.LabelPair{
				Name:  &name,
				Value: &value,
			})
		}
		jc.addMetric(labelsMap["__name__"], &dto.Metric{
			Label: labelPars,
			Untyped: &dto.Untyped{
				Value: &value,
			},
		})
		return
	}
}

func (jc *JmxCollector) defaultExport(domain string, beanProperties []map[string]string, attrKeys []string, attrName string, beanValue float64,config Config) {
	var buf bytes.Buffer
	buf.WriteString(domain)
	if len(beanProperties) > 0 {
		buf.WriteByte('_')
		fisrtmap := beanProperties[0]
		//this map: only one key & one value
		for _, v := range fisrtmap {
			buf.WriteString(v)
		}
	}
	for _, k := range attrKeys {
		buf.WriteByte('_')
		buf.WriteString(k)
	}
	buf.WriteByte('_')
	buf.WriteString(attrName)
	fullname := safeName(buf.String())
	if config.LowercaseOutputName {
		fullname = strings.ToLower(fullname)
	}
	labelsMap := make(map[string]string)
	labelsMap["__name__"] = fullname
	for _, propertiesmap := range beanProperties {
		for k, v := range propertiesmap {
			safek := safeName(k)
			if config.LowercaseOutputLabelNames {
				safek = strings.ToLower(safek)
			}
			labelsMap[safek] = v
		}
	}
	//printJSON(labelsMap)
	labelPars := []*dto.LabelPair{}
	for k, v := range labelsMap {
		if k == "__name__" {
			continue
		}
		name := k
		value := v
		labelPars = append(labelPars, &dto.LabelPair{
			Name:  &name,
			Value: &value,
		})
	}
	jc.addMetric(labelsMap["__name__"], &dto.Metric{
		Label: labelPars,
		Untyped: &dto.Untyped{
			Value: &beanValue,
		},
	})
}


func (jc *JmxCollector) getKeyPropertyListString(mbean interface{}) string {
	switch t := mbean.(type) {
	case map[string]interface{}:
		properties, ok := t["name"]
		if ok {
			return jc.getKeyPropertyListString(properties)
		}
		return ""
	case string:
		return t
	}
	return ""
}

func (jc *JmxCollector) getMbeans(iface interface{}) []interface{} {
	switch t := iface.(type) {
	case map[string]interface{}:
		beans, exist := t["beans"]
		if exist {
			return jc.getMbeans(beans)
		}
	case []interface{}:
		return t
	}
	return nil
}

func (jc *JmxCollector) getAttributes(mbean interface{}) map[string]interface{} {
	switch t := mbean.(type) {
	case map[string]interface{}:
		return t
	}
	return nil
}

func unmarshal(str string) (interface{}, error) {
	var iface interface{}
	decoder := json.NewDecoder(strings.NewReader(str))
	if err := decoder.Decode(&iface); err != nil {
		return nil, err
	}
	return iface, nil
}

func (jc *JmxCollector) addMetric(name string, metric *dto.Metric) {
	if len(name) == 0 {
		return
	}
	mfs, exist := jc.metricFamilies[name]
	if !exist {
		mfs = &dto.MetricFamily{
			Name:   &name,
			Type:   dto.MetricType_UNTYPED.Enum(),
			Metric: []*dto.Metric{},
		}
	}
	mfs.Metric = append(mfs.Metric, metric)
	jc.metricFamilies[name] = mfs
}

// GetMetricFamilies implements the MetricStore interface.
func GetMetricFamilies() []*dto.MetricFamily {
	jc := JmxCollector{
		metricFamilies:       map[string]*dto.MetricFamily{},
		keyPropertiesPerBean: map[string][]map[string]string{},
	}
	result := []*dto.MetricFamily{}
	err := jc.Scrape()
	if err != nil {
		log.Println(err)
		return result
	}
	for _, mfs := range jc.metricFamilies {
		result = append(result, mfs)
	}
	//printJSON(result)
	return result
}




//Scrape .
func (jc *JmxCollector) Scrape() error {
	var js interface{}
	var btrue float64
	for _, config := range configs {
		data ,err := fetchJson(config)
		if err !=nil{
			btrue=0
			jc.addMetric("jmx_parser_"+config.Endpointport, &dto.Metric{
				Label: nil,
				Untyped: &dto.Untyped{
					Value: &btrue,
				},
			})
			continue
			//这里添加抓取失败的metrics  labels port
		}
		btrue=1
		jc.addMetric("jmx_parser_"+config.Endpointport, &dto.Metric{
			Label: nil,
			Untyped: &dto.Untyped{
				Value: &btrue,
			},
		})
		//这里添加port 抓取成功的信息
		if js, err = unmarshal(string(data)); err != nil {
			continue
		}
		mbeans := jc.getMbeans(js)
		var wg  sync.WaitGroup
		for _, mbean := range mbeans {
			fqdn := jc.getKeyPropertyListString(mbean)
			attrs := jc.getAttributes(mbean)
			for attrName, attrVal := range attrs {
				wg.Add(1)
				jc.processBeanValue(colonPattern.Split(fqdn, 2)[0], jc.getKeyPropertyList(fqdn), []string{}, attrName, attrVal, *config,&wg)
			}
		}
		wg.Wait()
	}
	return nil
}
/*
//Scrape .
func (jc *JmxCollector) Scrape() error {
	var js interface{}
	for _, config := range configs {
		data ,err := fetchJson(config)
		if err !=nil{
			continue
			//这里添加抓取失败的metrics  labels port
		}
		//这里添加port 抓取成功的信息

		if js, err = unmarshal(string(data)); err != nil {
			continue
		}
		mbeans := jc.getMbeans(js)
		for _, mbean := range mbeans {
			fqdn := jc.getKeyPropertyListString(mbean)
			attrs := jc.getAttributes(mbean)
			for attrName, attrVal := range attrs {
				//这里添加抓取对象是否争取的信息
				jc.processBeanValue(colonPattern.Split(fqdn, 2)[0], jc.getKeyPropertyList(fqdn), []string{}, attrName, attrVal, *config)
			}
			//这里提供metrics 为 yaml 与抓取的json 匹配失败
		}
	}
	return nil
}*/

func fetchJson(config* Config) ([]byte, error) {
	c := &http.Client{
		Timeout: 20   * time.Second,
	}
	log.Println("fetchjson  http://"+fetchip+":"+config.Endpointport+"/jmx")
	resp, err := c.Get("http://"+fetchip+":"+config.Endpointport+"/jmx")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch json from endpoint;endpoint:<%s>,warning :<%s>", fetchip, err)
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return nil, fmt.Errorf("failed to read response body;err:<%s>", err)
	}

	return data, nil
}


func ifaceToString(iface interface{}) (string, error) {
	switch t := iface.(type) {
	case map[string]interface{}:
		strs := make([]string, len(t))
		i := 0
		for key, val := range t {
			str, err := ifaceToString(val)
			if err != nil {
				return "", err
			}
			strs[i] = fmt.Sprintf("%s: %s", key, str)
			i++
		}
		return "{" + strings.Join(strs, ", ") + "}", nil
	case []interface{}:
		strs := make([]string, len(t))
		i := 0
		for _, val := range t {
			str, err := ifaceToString(val)
			if err != nil {
				return "", err
			}
			strs[i] = str
			i++
		}
		return "[" + strings.Join(strs, ", ") + "]", nil
	case int64:
		return fmt.Sprintf("%d", t), nil
	case float64:
		return fmt.Sprintf("%v", t), nil
	case json.Number:
		return fmt.Sprintf("%s", t), nil
	case []string:
		strs := make([]string, len(t))
		i := 0
		for _, val := range t {
			str, err := ifaceToString(val)
			if err != nil {
				return "", err
			}
			strs[i] = str
			i++
		}
		return "[" + strings.Join(strs, ", ") + "]", nil
	case map[string]string:
		strs := make([]string, len(t))
		i := 0
		for key, val := range t {
			str, err := ifaceToString(val)
			if err != nil {
				return "", err
			}
			strs[i] = fmt.Sprintf("%s: %s", key, str)
			i++
		}
		return "{" + strings.Join(strs, ", ") + "}", nil
	case string:
		return fmt.Sprintf(`"%s"`, t), nil
	case bool:
		return fmt.Sprintf("%v", t), nil
	case nil:
		return fmt.Sprintf("%v", t), nil
	}
	return "", fmt.Errorf("unsupported value %#v (%T)", iface, iface)
}

func print(iface interface{}) {
	if str, err := ifaceToString(iface); err != nil {
		logger.Fatalf("error converting js (%s)", err)
	} else {
		logger.Println(str)
	}
}

