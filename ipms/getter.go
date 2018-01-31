package ipms

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sort"

	"github.com/castisdev/cilog"
)

// OfficeNodeMapping :
type OfficeNodeMapping struct {
	OfficeCode string `json:"officeCode"`
	NodeCode   string `json:"nodeCode"`
}

// NodeGLBIDMapping :
type NodeGLBIDMapping struct {
	NodeCode    string `json:"nodeCode"`
	ServiceCode string `json:"serviceCode"`
	GLBID       string `json:"glbId"`
}

// OfficeGLBIDMapping :
type OfficeGLBIDMapping struct {
	OfficeCode  string `json:"officeCode"`
	ServiceCode string `json:"serviceCode"`
	GLBID       string `json:"glbId"`
}

// GetOfficeGLBIDMapping :
func GetOfficeGLBIDMapping(cfg *YmlConfig) (map[string][]OfficeGLBIDMapping, error) {
	var officeNodes struct {
		List []OfficeNodeMapping `json:"officeNodeMappingList"`
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

	nodeGLBIDMap := map[string][]NodeGLBIDMapping{}
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
			List []NodeGLBIDMapping `json:"nodeGLBIdMappingList"`
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
	mapping := map[string][]OfficeGLBIDMapping{}
	for _, m := range officeNodes.List {
		if regions, ok := nodeGLBIDMap[m.NodeCode]; ok {
			for _, r := range regions {
				mapping[m.OfficeCode] = append(mapping[m.OfficeCode], OfficeGLBIDMapping{
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

// NetMaskInfo :
type NetMaskInfo struct {
	NetMaskAddress string `json:"netMaskAddress"`
	NetCode        string `json:"netCode"`
}

// GLBInfo :
type GLBInfo struct {
	GLBID              string         `json:"glbId"`
	NetMaskAddressList []*NetMaskInfo `json:"netMaskAddressList"`
}

// ServiceCodeInfo :
type ServiceCodeInfo struct {
	ServiceCode      string     `json:"serviceCode"`
	GLBIDNetMaskList []*GLBInfo `json:"glbIdNetMaskList"`
}

// MergeIPMSRecords :
func MergeIPMSRecords(recs []*IpmsRecord) ([]*ServiceCodeInfo, error) {
	sort.Sort(ipmsSort(recs))

	var set contSet
	var resultSet []*IpmsRecord
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

	var serviceCodeInfos []*ServiceCodeInfo
	var scInfo *ServiceCodeInfo
	var rInfo *GLBInfo

	var prevServiceCode, prevGLBID string
	for _, rec2 := range resultSet {
		if prevServiceCode != rec2.ServiceCode {
			scInfo = &ServiceCodeInfo{}
			scInfo.ServiceCode = rec2.ServiceCode
			prevServiceCode = rec2.ServiceCode
			serviceCodeInfos = append(serviceCodeInfos, scInfo)
		}
		if prevGLBID != rec2.GLBID {
			rInfo = &GLBInfo{}
			rInfo.GLBID = rec2.GLBID
			prevGLBID = rec2.GLBID
			scInfo.GLBIDNetMaskList = append(scInfo.GLBIDNetMaskList, rInfo)
		}
		rInfo.NetMaskAddressList = append(rInfo.NetMaskAddressList, &NetMaskInfo{rec2.CIDR, rec2.NetCode})
	}
	cilog.Infof("success to merge, lines[%d]", len(resultSet))
	return serviceCodeInfos, nil
}

// PostIPMSRecords :
func PostIPMSRecords(cfg *YmlConfig, infos []*ServiceCodeInfo) error {
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
