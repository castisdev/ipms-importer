package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/castisdev/cilog"
	"github.com/kardianos/osext"
)

const (
	component   = "ipms-importer"
	ymlFilename = "ipms-importer.yml"
	ver         = "1.0.0"
	preRelVer   = "-rc.0"
)

var usage = func() {
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] [ipms_data_file]\n", os.Args[0])
		flag.PrintDefaults()
	}

	ymlConfigFilePath := flag.String("config-file", "", "config file path")
	printSimpleVer := flag.Bool("v", false, "print version")
	printVer := flag.Bool("version", false, "print version includes pre-release version")
	flag.Parse()

	if *printSimpleVer {
		fmt.Println(component + " " + ver)
		os.Exit(0)
	}

	if *printVer {
		fmt.Println(component + " " + ver + preRelVer)
		os.Exit(0)
	}

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	if len(*ymlConfigFilePath) == 0 {
		dir, err := osext.ExecutableFolder()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		*ymlConfigFilePath = path.Join(dir, ymlFilename)
	}
	ymlCfg, err := newYmlConfig(*ymlConfigFilePath)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	lvl, err := cilog.LevelFromString(ymlCfg.LogLevel)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	cilog.Set(cilog.NewLogWriter(ymlCfg.LogDir, component, 10*1024*1024), component, ver, lvl)

	cilog.Infof("program started")

	mapping, err := getOfficeRegionMapping(ymlCfg)

	var recs []*ipmsRecord
	{
		f, err := os.Open("/home/sasgas/Documents/cdn/FILE_TB_ASSIGN.DAT")
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()

		s := bufio.NewScanner(f)
		lineCnt := 0
		invalidLineCnt := 0
		for s.Scan() {
			line := s.Text()
			lineCnt++
			ret := strings.Split(line, "|")
			officeCode := ret[5]
			if glb, ok := mapping[officeCode]; ok {
				rec, err := newRecord(glb.ServiceCode, glb.RegionID, officeCode, ret[0], ret[7])
				if err != nil {
					log.Fatal(err)
				}
				recs = append(recs, rec)
			} else {
				log.Printf("invalid office code, %s, line[%d]", officeCode, lineCnt)
				invalidLineCnt++
			}
		}
		log.Printf("success to parse file, line[%d], invalid_line[%d]", len(recs), invalidLineCnt)

		if err := s.Err(); err != nil {
			log.Fatal(err)
		}
	}

	sort.Sort(ipmsSort(recs))

	var set contSet
	var resultSet []*ipmsRecord
	for _, rec := range recs {
		if set.IsCont(rec) {
			set.Add(rec)
		} else {
			set.Sum()
			set.printLog()
			resultSet = append(resultSet, set...)
			set = set[:0]
			set.Add(rec)
		}
	}
	log.Printf("success to merge, line[%d]", len(resultSet))

	{
		b := new(bytes.Buffer)
		err := json.NewEncoder(b).Encode(resultSet)
		if err != nil {
			log.Fatal(err)
		}
		req, err := http.NewRequest("POST", "http://localhost:8085/api/ip-routing-info-cfg", b)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("%s %s", req.Method, req.URL)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			log.Fatal(resp.Status)
		}
		log.Print(resp.Status)
	}
}
