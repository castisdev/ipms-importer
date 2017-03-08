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

type officeNodeMapping struct {
	OfficeCode string `json:"officeCode"`
	NodeCode   string `json:"nodeCode"`
}

type nodeGLBIDMapping struct {
	NodeCode    string `json:"nodeCode"`
	ServiceCode string `json:"serviceCode"`
	GLBID       string `json:"glbId"`
}

type officeGLBIDMapping struct {
	OfficeCode  string `json:"officeCode"`
	ServiceCode string `json:"serviceCode"`
	GLBID       string `json:"glbId"`
}

func getOfficeGLBIDMapping(cfg *ymlConfig) (map[string][]officeGLBIDMapping, error) {
	var officeNodes struct {
		List []officeNodeMapping `json:"officeNodeMappingList"`
	}
	{
		req, err := http.NewRequest("GET", cfg.OfficeNodeAPI, nil)
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
		err = dec.Decode(&officeNodes)
		if err != nil {
			return nil, err
		}
		cilog.Infof("success to get office-code-node-code-mapping, row[%d]", len(officeNodes.List))
	}

	nodeGLBIDMap := map[string][]nodeGLBIDMapping{}
	{
		req, err := http.NewRequest("GET", cfg.NodeGLBIDAPI, nil)
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

		var nodeGLBIDs struct {
			List []nodeGLBIDMapping `json:"nodeGLBIdMappingList"`
		}
		err = dec.Decode(&nodeGLBIDs)
		if err != nil {
			return nil, err
		}
		cilog.Infof("success to get node-code-glb-id-mapping, row[%d]", len(nodeGLBIDs.List))

		for _, m := range nodeGLBIDs.List {
			nodeGLBIDMap[m.NodeCode] = append(nodeGLBIDMap[m.NodeCode], m)
		}
	}

	failedNodes := map[string]struct{}{}
	mapping := map[string][]officeGLBIDMapping{}
	for _, m := range officeNodes.List {
		if regions, ok := nodeGLBIDMap[m.NodeCode]; ok {
			for _, r := range regions {
				mapping[m.OfficeCode] = append(mapping[m.OfficeCode], officeGLBIDMapping{
					OfficeCode:  m.OfficeCode,
					ServiceCode: r.ServiceCode,
					GLBID:       r.GLBID,
				})
			}
		} else {
			failedNodes[m.NodeCode] = struct{}{}
		}
	}

	for k := range failedNodes {
		cilog.Warningf("failed to find glbId[%s]", k)
	}

	return mapping, nil
}

type netMaskInfo struct {
	NetMaskAddress string `json:"netMaskAddress"`
	NetCode        string `json:"netCode"`
}

type glbInfo struct {
	GLBID              string         `json:"glbId"`
	NetMaskAddressList []*netMaskInfo `json:"netMaskAddressList"`
}

type serviceCodeInfo struct {
	ServiceCode      string     `json:"serviceCode"`
	GLBIDNetMaskList []*glbInfo `json:"glbIdNetMaskList"`
}

func getIPMSRecords(filename string, mapping map[string][]officeGLBIDMapping) ([]*serviceCodeInfo, error) {
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
			if len(ret) < 8 {
				cilog.Warningf("invalid line[%d], %s", lineCnt, line)
				invalidLineCnt++
				continue
			}
			officeCode := ret[5]
			if glbs, ok := mapping[officeCode]; ok {
				for _, glb := range glbs {
					rec, err := newRecord(glb.ServiceCode, glb.GLBID, ret[1], officeCode, ret[0], ret[7])
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

	// last set
	set.Sum()
	set.printLog()
	resultSet = append(resultSet, set...)

	var serviceCodeInfos []*serviceCodeInfo
	var scInfo *serviceCodeInfo
	var rInfo *glbInfo

	var prevServiceCode, prevGLBID string
	for _, rec2 := range resultSet {
		if prevServiceCode != rec2.ServiceCode {
			scInfo = &serviceCodeInfo{}
			scInfo.ServiceCode = rec2.ServiceCode
			prevServiceCode = rec2.ServiceCode
			serviceCodeInfos = append(serviceCodeInfos, scInfo)
		}
		if prevGLBID != rec2.GLBID {
			rInfo = &glbInfo{}
			rInfo.GLBID = rec2.GLBID
			prevGLBID = rec2.GLBID
			scInfo.GLBIDNetMaskList = append(scInfo.GLBIDNetMaskList, rInfo)
		}
		rInfo.NetMaskAddressList = append(rInfo.NetMaskAddressList, &netMaskInfo{rec2.CIDR, rec2.NetCode})
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
