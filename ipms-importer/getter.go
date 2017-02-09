package main

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/castisdev/cilog"
)

type officeGLBNodeMapping struct {
	OfficeCode string `json:"officeCode"`
	GLBNode    string `json:"glbNodeCode"`
}

type glbNodeRegionMapping struct {
	GLBNode     string `json:"glbNodeCode"`
	ServiceCode string `json:"serviceCode"`
	RegionID    string `json:"regionId"`
}

type officeRegionMapping struct {
	OfficeCode  string `json:"officeCode"`
	ServiceCode string `json:"serviceCode"`
	RegionID    string `json:"regionId"`
}

func getOfficeRegionMapping(cfg *ymlConfig) (map[string]officeRegionMapping, error) {
	var officeGlbs []officeGLBNodeMapping
	{
		req, err := http.NewRequest("GET", cfg.OfficeGLBNodeAPI, nil)
		if err != nil {
			return nil, err
		}
		cilog.Infof("%s %s", req.Method, req.URL)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, errors.New(resp.Status)
		}
		cilog.Infof(resp.Status)

		dec := json.NewDecoder(resp.Body)
		err = dec.Decode(&officeGlbs)
		if err != nil {
			return nil, err
		}
		cilog.Infof("success to get office-code-glb-node-code-mapping, row[%d]", len(officeGlbs))
	}

	glbRegionMap := map[string]glbNodeRegionMapping{}
	{
		req, err := http.NewRequest("GET", cfg.GLBNodeRegionAPI, nil)
		if err != nil {
			return nil, err
		}
		cilog.Infof("%s %s", req.Method, req.URL)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, errors.New(resp.Status)
		}
		cilog.Infof(resp.Status)

		dec := json.NewDecoder(resp.Body)
		var glbRegions []glbNodeRegionMapping
		err = dec.Decode(&glbRegions)
		if err != nil {
			return nil, err
		}
		cilog.Infof("success to get glb-node-code-region-id-mapping, row[%d]", len(officeGlbs))

		for _, m := range glbRegions {
			glbRegionMap[m.GLBNode] = m
		}
	}

	mapping := map[string]officeRegionMapping{}
	for _, m := range officeGlbs {
		if v, ok := glbRegionMap[m.GLBNode]; ok {
			mapping[m.OfficeCode] = officeRegionMapping{
				OfficeCode:  m.OfficeCode,
				ServiceCode: v.ServiceCode,
				RegionID:    v.RegionID,
			}
		} else {
			cilog.Warningf("failed to find glbNodeCode[%s]", m.GLBNode)
		}
	}

	return mapping, nil
}
