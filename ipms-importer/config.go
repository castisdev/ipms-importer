package main

import (
	"errors"
	"fmt"
	"io/ioutil"

	yaml "gopkg.in/yaml.v1"
)

type ymlConfig struct {
	LogDir              string `yaml:"log-directory"`
	LogLevel            string `yaml:"log-level"`
	OfficeGLBNodeAPI    string `yaml:"mapping-office-glb-node-api"`
	GLBNodeRegionAPI    string `yaml:"mapping-glb-node-region-api"`
	IPRoutingInfoCfgAPI string `yaml:"import-ipms-api"`
}

func newYmlConfig(ymlConfigFilePath string) (*ymlConfig, error) {
	originData, err := ioutil.ReadFile(ymlConfigFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config, %v", err)
	}
	cfg := ymlConfig{}
	err = yaml.Unmarshal([]byte(originData), &cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal, %v", err)
	}
	if cfg.LogDir == "" {
		cfg.LogDir = "log"
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}
	if cfg.OfficeGLBNodeAPI == "" {
		return nil, errors.New("mapping-office-glb-node-api not exist")
	}
	if cfg.GLBNodeRegionAPI == "" {
		return nil, errors.New("mapping-glb-node-region-api not exist")
	}
	if cfg.IPRoutingInfoCfgAPI == "" {
		return nil, errors.New("import-ipms-api not exist")
	}

	return &cfg, nil
}
