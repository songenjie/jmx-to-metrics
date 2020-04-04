package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"github.com/go-errors/errors"
	"gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"regexp"
	"time"
)

var logger = log.New(os.Stdout, "", log.Lshortfile)

var (
	configs []*Config
	realip string
	icount  =0
	h ,H             bool
	v ,V             bool
	listenport       string
	workdir          string
	fetchip          string
	configserverurl  string
	updateinterval   string
)

func init() {
	ip, err := getOutboundIP()
	if err != nil {
		log.Println("get local ip failed")
		return
	}
	realip = ip.String()
	log.Printf("realip: %s",realip)

	flag.BoolVar(&h, "h", false, "this help")
	flag.BoolVar(&v, "v", false, "show version and exit")

	// 注意 `signal`。默认是 -s string，有了 `signal` 之后，变为 -s signal
	flag.StringVar(&listenport,      "l", "19000", "set `listen port` ")
	flag.StringVar(&workdir,         "w", "/software/jmx-to-metrics", "set `work dir` ")
	flag.StringVar(&fetchip,         "f", "", "set `fetch ip` eg :192.168.1.1")
	flag.StringVar(&configserverurl, "c", "server", "set `configserver url`")
	flag.StringVar(&updateinterval,  "u", "5", "set `update interval Minute` to configserver")
	// 改变默认的 Usage
	flag.Usage = usage
}


func updateyaml() {
	//1 获取服务类型
	fetchtypefilename := workdir+"/fetch.txt"
	fetchyamlfilename := workdir+"/fetch.yaml"
	log.Println(fetchyamlfilename)
	log.Println(fetchtypefilename)

	log.Println("http://"+configserverurl+"/api/monitorServiceType?ip="+realip+"&promeType=jmx_parser&port="+listenport)
	body,err := request("http://"+configserverurl+"/api/monitorServiceType?ip="+realip+"&promeType=jmx_parser&port="+listenport, "GET")
	if err != nil {
		log.Println("http yaml failed form configsever!")
		return
	}
	if len(body) == 0{
		log.Println("not get fetch  type !")
		return
	}
	err =UpdateFile(fetchtypefilename,body)
	if err !=nil {
		log.Println("update fetchyaml failed !")
		return
	}

	fi, err := os.Open(fetchtypefilename)
	if err != nil {
		log.Printf("Error: %s\n", err)
		return
	}
	defer fi.Close()
	br := bufio.NewReader(fi)
	icount=0
	for {
		fetchtype, _, c := br.ReadLine()
		if c == io.EOF {
			break
		}
		icount ++
		//获取yaml 配置
		log.Println("http://"+configserverurl+"/api/config?ip="+realip+"&port="+listenport+"&promeType=jmx_parser&type="+string(fetchtype))
		body,err := request("http://"+configserverurl+"/api/config?ip="+realip+"&port="+listenport+"&promeType=jmx_parser&type="+string(fetchtype), "GET")
		if err != nil {
			log.Println("get fetchtype yaml failed form configsever!")
			body,err = request("http://"+configserverurl+"/api/config?ip="+realip+"&port="+listenport+"&promeType=jmx_parser&type="+string(fetchtype), "GET")
			if err != nil{
			       continue
			}
		}

		if icount ==1  {
			err =UpdateFile(fetchyamlfilename,body)
			if err !=nil{
				log.Println("update yaml file failed !")
				continue 
			}
		}else{	
			err =appendyamlfile(fetchyamlfilename,body)
			if err != nil {
				log.Println("append yaml file failed !")
				continue
			}
		
		}

	}

	icount=0
	log.Println("WriteFile file success!")
	err =loadConfig(fetchyamlfilename)

	if err != nil {
		log.Println("load  yaml file failed !")
		return
	}
	return
}


func appendyamlfile(filename string,body []byte) error {
	// If the file doesn't exist, create it, or append to the file
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	if _, err := f.Write(body); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return nil
}
func  UpdateFile(filename string,body []byte) ( error){
	reader  :=bytes.NewReader(body)
	scanner :=bufio.NewScanner(reader)
	f, err := os.OpenFile(filename,os.O_RDWR|os.O_CREATE, 0644)
	if err != nil{
		return err
	}
	os.Truncate(filename, 0)
	for scanner.Scan() {
		_,err :=f.WriteString(scanner.Text()+"\n")
		if err != nil{
			return err
		}
	}

	f.Close()
	return err
}


func loadConfig(filename string ) error {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(content, &configs)
	if err != nil {
		return  err
	}
	for _, config := range configs{
		config.WhitelistObjectNamesReg = make([]*regexp.Regexp, len(config.WhitelistObjectNames))
		config.BlacklistObjectNamesReg = make([]*regexp.Regexp, len(config.BlacklistObjectNames))
		for i, whitelistStr := range config.WhitelistObjectNames {
			config.WhitelistObjectNamesReg[i] = regexp.MustCompile(whitelistStr)
		}
		for i, blacklistStr := range config.BlacklistObjectNames {
			config.BlacklistObjectNamesReg[i] = regexp.MustCompile(blacklistStr)
		}
		for _, rule := range config.Rules {
			rule.PatternReg = regexp.MustCompile(rule.Pattern)
			var tmp float64
			if rule.ValueFactor == tmp {
				rule.ValueFactor = 1.0
			}
			for name, value := range rule.Labels {
				rule.LabelNames = append(rule.LabelNames, name)
				rule.LabelValues = append(rule.LabelValues, value)
			}
		}
	}
	return nil
}


func request(url, method string) ( []byte, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{
		Timeout: time.Duration(10 * time.Second),
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		msg := fmt.Sprintf("response status code is %d", resp.StatusCode)
		return nil, errors.New(msg)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
		return nil,  err
	}
	return body ,err
}


func  getOutboundIP() (net.IP, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP, nil
}


