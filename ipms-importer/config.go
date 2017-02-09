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
	OfficeGLBNodeAPI    string `yaml:"office-glb-node-api"`
	GLBNodeRegionAPI    string `yaml:"glb-node-region-api"`
	IPRoutingInfoCfgAPI string `yaml:"ip-routing-info-cfg-api"`
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
		return nil, errors.New("office-glb-node-api not exist")
	}
	if cfg.GLBNodeRegionAPI == "" {
		return nil, errors.New("glb-node-region-api not exist")
	}
	if cfg.IPRoutingInfoCfgAPI == "" {
		return nil, errors.New("ip-routing-info-cfg-api not exist")
	}

	return &cfg, nil
}
