package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/castisdev/cilog"
	"github.com/castisdev/ipms-importer/ipms"
	"github.com/kardianos/osext"
)

const (
	component   = "ipms-importer"
	ymlFilename = "ipms-importer.yml"
	ver         = "1.0.1"
	preRelVer   = "-rc.0"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]... INPUT_FILE\n", os.Args[0])
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
		fmt.Fprintf(os.Stderr, "there is no INPUT_FILE\n\n")
		flag.Usage()
		os.Exit(1)
	}

	if _, err := os.Stat(flag.Arg(0)); os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if len(*ymlConfigFilePath) == 0 {
		dir, err := osext.ExecutableFolder()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		*ymlConfigFilePath = path.Join(dir, ymlFilename)
	}
	cfg, err := ipms.NewYmlConfig(*ymlConfigFilePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	lvl, err := cilog.LevelFromString(cfg.LogLevel)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	cilog.Set(cilog.NewLogWriter(cfg.LogDir, component, 10*1024*1024), component, ver, lvl)

	cilog.Infof("program started")

	mapping, err := ipms.GetOfficeGLBIDMapping(cfg)
	if err != nil {
		str := fmt.Sprintf("failed to get mapping info, %v", err)
		cilog.Errorf(str)
		fmt.Fprintln(os.Stderr, str)
		os.Exit(1)
	}

	ipmsSet, err := getIPMSRecords(flag.Arg(0), mapping)
	if err != nil {
		str := fmt.Sprintf("failed to get ipms records, %v", err)
		cilog.Errorf(str)
		fmt.Fprintln(os.Stderr, str)
		os.Exit(1)
	}

	resultSet, err := ipms.MergeIPMSRecords(ipmsSet)
	if err != nil {
		str := fmt.Sprintf("failed to merge ipms records, %v", err)
		cilog.Errorf(str)
		fmt.Fprintln(os.Stderr, str)
		os.Exit(1)
	}

	err = ipms.PostIPMSRecords(cfg, resultSet)
	if err != nil {
		str := fmt.Sprintf("failed to post ipms records, %v", err)
		cilog.Errorf(str)
		fmt.Fprintln(os.Stderr, str)
		os.Exit(1)
	}

	str := fmt.Sprintf("success to import, %s", flag.Arg(0))
	cilog.Infof(str)
	fmt.Println(str)
	cilog.Infof("program ended")
}

func getIPMSRecords(filename string, mapping map[string][]ipms.OfficeGLBIDMapping) ([]*ipms.IpmsRecord, error) {
	var recs []*ipms.IpmsRecord

	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	lineCnt := 0
	invalidLineCnt := 0
	failedOfficeCodes := map[string]int{}
	for s.Scan() {
		line := s.Text()
		lineCnt++
		ret := strings.Split(line, "|")
		if len(ret) < 8 {
			cilog.Warningf("invalid line[%d], %s", lineCnt, line)
			invalidLineCnt++
			continue
		}
		officeCode := ret[5]
		if glbs, ok := mapping[officeCode]; ok {
			for _, glb := range glbs {
				rec, err := ipms.NewRecord(glb.ServiceCode, glb.GLBID, ret[1], officeCode, ret[0], ret[7])
				if err != nil {
					return nil, err
				}
				recs = append(recs, rec)
			}
		} else {
			failedOfficeCodes[officeCode] = lineCnt
			invalidLineCnt++
		}
	}

	if err := s.Err(); err != nil {
		return nil, err
	}

	for k, v := range failedOfficeCodes {
		cilog.Warningf("invalid office code, %s, line[%d]", k, v)
	}
	cilog.Infof("success to parse file, lines[%d], invalid lines[%d]", len(recs), invalidLineCnt)

	return recs, nil
}
