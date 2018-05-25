package main

import (
	"bufio"
	"encoding/csv"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/castisdev/cilog"
	"github.com/castisdev/ipms-importer/ipms"
)

const (
	component = "ipms-to-report-collector"
	ver       = "1.0.0"
	preRelVer = "-rc.0"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]... INPUT_FILE\n", os.Args[0])
		flag.PrintDefaults()
	}

	logDirPath := flag.String("log-dir", "./log", "log dir path")
	outputDirPath := flag.String("output-dir", "./output", "output dir path")
	api := flag.String("api-url", "http://localhost:8780/import/reportCollector", "api url")
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

	cilog.Set(cilog.NewLogWriter(*logDirPath, component, 10*1024*1024), component, ver, cilog.DEBUG)

	cilog.Infof("program started")

	ipmsSet, err := getIPMSRecords2(flag.Arg(0))
	if err != nil {
		str := fmt.Sprintf("failed to get ipms records, %v", err)
		cilog.Errorf(str)
		fmt.Fprintln(os.Stderr, str)
		os.Exit(1)
	}

	resultSet := ipms.MergeIPMSRecords2(ipmsSet)

	_, fn := filepath.Split(flag.Arg(0))
	csvFilepath := filepath.Join(*outputDirPath, fn)

	err = os.MkdirAll(*outputDirPath, 0777)
	if err != nil {
		str := fmt.Sprintf("failed to mkdir, %v", err)
		cilog.Errorf(str)
		fmt.Fprintln(os.Stderr, str)
		os.Exit(1)
	}

	o, err := os.Create(csvFilepath)
	if err != nil {
		str := fmt.Sprintf("failed to write csv records, %v", err)
		cilog.Errorf(str)
		fmt.Fprintln(os.Stderr, str)
		os.Exit(1)
	}
	defer o.Close()

	w := csv.NewWriter(o)
	if err := w.Write([]string{"NetMaskAddress", "Beallorg", "IPMS_OFC_NAME"}); err != nil {
		cilog.Errorf("%v", err)
		os.Exit(1)
	}

	for _, r := range resultSet {
		csv := []string{r.NetMaskAddress, r.AreaName, r.OfficeName}
		if err := w.Write(csv); err != nil {
			cilog.Errorf("%v", err)
			continue
		}
	}
	w.Flush()

	err = ipms.PostReportCollectorRecords(*api, resultSet)
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

type checkItem struct {
	beallorg   string
	officeName string
	cidr       string
}

func getIPMSRecords2(filename string) ([]*ipms.IpmsRecord, error) {
	var recs []*ipms.IpmsRecord

	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	lineCnt := 0
	invalidLineCnt := 0
	checker := make(map[checkItem]struct{})
	for s.Scan() {
		line := s.Text()
		lineCnt++
		ret := strings.Split(line, "|")
		if len(ret) != 8 {
			cilog.Warningf("invalid line[%d], %s", lineCnt, line)
			invalidLineCnt++
			continue
		}
		officeName := ret[4]
		beallorg := ret[2]

		ips := net.ParseIP(ret[0])
		ipe := net.ParseIP(ret[1])
		if ips == nil || ipe == nil {
			cilog.Warningf("invalid row[%d], %s, %s", lineCnt, ret[0], ret[1])
			invalidLineCnt++
			continue
		}

		cidrs := ipms.Range2CIDRs(ips, ipe)
		for _, cidr := range cidrs {
			rec, err := ipms.NewRecordFromCIDR(beallorg, officeName, "", officeName, cidr)
			if err != nil {
				return nil, err
			}
			ci := checkItem{beallorg, officeName, rec.CIDR}
			if _, ok := checker[ci]; ok {
				cilog.Warningf("duplicate record, line[%d], %s", lineCnt, line)
				invalidLineCnt++
				continue
			}
			checker[ci] = struct{}{}
			recs = append(recs, rec)
		}
	}

	if err := s.Err(); err != nil {
		return nil, err
	}

	cilog.Infof("success to parse file, lines[%d], invalid lines[%d]", len(recs), invalidLineCnt)

	return recs, nil
}
