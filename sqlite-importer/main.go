package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"path"

	"github.com/castisdev/cilog"
	"github.com/castisdev/ipms-importer/ipms"
	"github.com/kardianos/osext"
	_ "github.com/mattn/go-sqlite3"
)

const (
	component   = "sqlite-importer"
	ymlFilename = "sqlite-importer.yml"
	ver         = "1.0.2"
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

	db, err := sql.Open("sqlite3", filename)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	rows, err := db.Query("select StartIP, EndIP, AMOC_OFC_CD from IPMSfile_to_AMOC_OFFICE_MAPPING")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	lineCnt := 0
	invalidLineCnt := 0
	failedOfficeCodes := map[string]int{}
	for rows.Next() {
		lineCnt++

		var s1, s2, s3 sql.NullString
		err = rows.Scan(&s1, &s2, &s3)
		if err != nil {
			log.Fatal(err)
		}

		if s1.Valid == false || s2.Valid == false || s3.Valid == false {
			cilog.Warningf("invalid row[%d]", lineCnt)
			invalidLineCnt++
			continue
		}

		officeCode := s3.String
		if glbs, ok := mapping[officeCode]; ok {
			for _, glb := range glbs {
				ips := net.ParseIP(s1.String)
				ipe := net.ParseIP(s2.String)
				if ips == nil || ipe == nil {
					cilog.Warningf("invalid row[%d], %s, %s", lineCnt, s1.String, s2.String)
					invalidLineCnt++
					continue
				}
				cidrs := ipms.Range2CIDRs(ips, ipe)
				for _, cidr := range cidrs {
					rec, err := ipms.NewRecordFromCIDR(glb.ServiceCode, glb.GLBID, "", officeCode, cidr)
					if err != nil {
						return nil, err
					}
					recs = append(recs, rec)
				}
			}
		} else {
			failedOfficeCodes[officeCode] = lineCnt
			invalidLineCnt++
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	for k, v := range failedOfficeCodes {
		cilog.Warningf("invalid office code, %s, row[%d]", k, v)
	}
	cilog.Infof("success to read sqlite db, rows[%d], invalid rows[%d], records[%d]", lineCnt, invalidLineCnt, len(recs))

	return recs, nil
}
