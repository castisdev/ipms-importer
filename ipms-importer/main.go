package main

import (
	"flag"
	"fmt"
	"os"
	"path"

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
	cfg, err := newYmlConfig(*ymlConfigFilePath)
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

	mapping, err := getOfficeRegionMapping(cfg)
	if err != nil {
		str := fmt.Sprintf("failed to get mapping info, %v", err)
		cilog.Errorf(str)
		fmt.Fprintln(os.Stderr, str)
		os.Exit(1)
	}

	resultSet, err := getIPMSRecords(flag.Arg(0), mapping)
	if err != nil {
		str := fmt.Sprintf("failed to get ipms records, %v", err)
		cilog.Errorf(str)
		fmt.Fprintln(os.Stderr, str)
		os.Exit(1)
	}

	err = postIPMSRecords(cfg, resultSet)
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
