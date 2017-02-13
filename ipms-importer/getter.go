package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sort"
	"strings"

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

func getOfficeRegionMapping(cfg *ymlConfig) (map[string][]officeRegionMapping, error) {
	var officeGlbs struct {
		List []officeGLBNodeMapping `json:"officeGLBNodeMappingList"`
	}
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
			b, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				cilog.Warningf("%v", err)
			}
			return nil, fmt.Errorf("%s, %s", resp.Status, string(b))
		}
		cilog.Infof(resp.Status)

		dec := json.NewDecoder(resp.Body)
		err = dec.Decode(&officeGlbs)
		if err != nil {
			return nil, err
		}
		cilog.Infof("success to get office-code-glb-node-code-mapping, row[%d]", len(officeGlbs.List))
	}

	glbRegionMap := map[string][]glbNodeRegionMapping{}
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
			b, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				cilog.Warningf("%v", err)
			}
			return nil, fmt.Errorf("%s, %s", resp.Status, string(b))
		}
		cilog.Infof(resp.Status)

		dec := json.NewDecoder(resp.Body)

		var glbRegions struct {
			List []glbNodeRegionMapping `json:"glbNodeRegionMappingList"`
		}
		err = dec.Decode(&glbRegions)
		if err != nil {
			return nil, err
		}
		cilog.Infof("success to get glb-node-code-region-id-mapping, row[%d]", len(glbRegions.List))

		for _, m := range glbRegions.List {
			glbRegionMap[m.GLBNode] = append(glbRegionMap[m.GLBNode], m)
		}
	}

	failedNodes := map[string]struct{}{}
	mapping := map[string][]officeRegionMapping{}
	for _, m := range officeGlbs.List {
		if regions, ok := glbRegionMap[m.GLBNode]; ok {
			for _, r := range regions {
				mapping[m.OfficeCode] = append(mapping[m.OfficeCode], officeRegionMapping{
					OfficeCode:  m.OfficeCode,
					ServiceCode: r.ServiceCode,
					RegionID:    r.RegionID,
				})
			}
		} else {
			failedNodes[m.GLBNode] = struct{}{}
		}
	}

	for k := range failedNodes {
		cilog.Warningf("failed to find glbNodeCode[%s]", k)
	}

	return mapping, nil
}

type regionInfo struct {
	RegionID           string   `json:"regionId"`
	NetMaskAddressList []string `json:"netMaskAddressList"`
}

type serviceCodeInfo struct {
	ServiceCode       string        `json:"serviceCode"`
	RegionNetMaskList []*regionInfo `json:"regionNetMaskList"`
}

func getIPMSRecords(filename string, mapping map[string][]officeRegionMapping) ([]*serviceCodeInfo, error) {
	var recs []*ipmsRecord
	{
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
			officeCode := ret[5]
			if glbs, ok := mapping[officeCode]; ok {
				for _, glb := range glbs {
					rec, err := newRecord(glb.ServiceCode, glb.RegionID, officeCode, ret[0], ret[7])
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

	var serviceCodeInfos []*serviceCodeInfo
	var scInfo *serviceCodeInfo
	var rInfo *regionInfo

	var prevServiceCode, prevRegionID string
	for _, rec2 := range resultSet {
		if prevServiceCode != rec2.ServiceCode {
			scInfo = &serviceCodeInfo{}
			scInfo.ServiceCode = rec2.ServiceCode
			prevServiceCode = rec2.ServiceCode
			serviceCodeInfos = append(serviceCodeInfos, scInfo)
		}
		if prevRegionID != rec2.RegionID {
			rInfo = &regionInfo{}
			rInfo.RegionID = rec2.RegionID
			prevRegionID = rec2.RegionID
			scInfo.RegionNetMaskList = append(scInfo.RegionNetMaskList, rInfo)
		}
		rInfo.NetMaskAddressList = append(rInfo.NetMaskAddressList, rec2.CIDR)
	}
	cilog.Infof("success to merge, lines[%d]", len(resultSet))
	return serviceCodeInfos, nil
}

func postIPMSRecords(cfg *ymlConfig, infos []*serviceCodeInfo) error {
	b := new(bytes.Buffer)
	err := json.NewEncoder(b).Encode(infos)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", cfg.IPRoutingInfoCfgAPI, b)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	cilog.Infof("%s %s", req.Method, req.URL)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			cilog.Warningf("%v", err)
		}
		return fmt.Errorf("%s, %s", resp.Status, string(b))
	}
	cilog.Infof(resp.Status)
	return nil
}
