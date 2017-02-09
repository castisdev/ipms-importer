package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/mux"
)

type handler struct{}

type officeGLBNodeMapping struct {
	OfficeCode string `json:"officeCode"`
	GLBNode    string `json:"glbNodeCode"`
}

func (h *handler) getOfficeGLBNodeMapping(w http.ResponseWriter, r *http.Request) {
	f, err := os.Open("office-code-mapping.csv")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, err)
		return
	}
	defer f.Close()

	var m struct {
		List []officeGLBNodeMapping `json:"officeGLBNodeMappingList"`
	}
	s := bufio.NewScanner(f)
	s.Scan() // skip first line
	for s.Scan() {
		ret := strings.Split(s.Text(), ",")
		m.List = append(m.List, officeGLBNodeMapping{ret[1], ret[0]})
	}
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	err = enc.Encode(m)
	if err != nil {
		fmt.Fprintln(w, err)
		return
	}
}

type glbNodeRegionMapping struct {
	GLBNode     string `json:"glbNodeCode"`
	ServiceCode string `json:"serviceCode"`
	RegionID    string `json:"regionId"`
}

func (h *handler) getGLBNodeRegionMapping(w http.ResponseWriter, r *http.Request) {
	f, err := os.Open("glb-mapping.csv")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, err)
		return
	}
	defer f.Close()

	var m struct {
		List []glbNodeRegionMapping `json:"glbNodeRegionMappingList"`
	}
	s := bufio.NewScanner(f)
	s.Scan() // skip first line
	for s.Scan() {
		ret := strings.Split(s.Text(), ",")
		m.List = append(m.List, glbNodeRegionMapping{ret[0], ret[1], ret[2]})
	}
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	err = enc.Encode(m)
	if err != nil {
		fmt.Fprintln(w, err)
		return
	}
}

type officeRegionMapping struct {
	OfficeCode  string `json:"officeCode"`
	ServiceCode string `json:"serviceCode"`
	RegionID    string `json:"regionId"`
}

func (h *handler) getOfficeRegionMapping(w http.ResponseWriter, r *http.Request) {
	type serviceCodeRegionID struct {
		serviceCode, regionID string
	}
	glbMapping := map[string]serviceCodeRegionID{}
	{
		f, err := os.Open("glb-mapping.csv")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, err)
			return
		}

		s := bufio.NewScanner(f)
		s.Scan() // skip first line
		for s.Scan() {
			ret := strings.Split(s.Text(), ",")
			glbMapping[ret[0]] = serviceCodeRegionID{ret[1], ret[2]}
		}
		f.Close()
	}

	f, err := os.Open("office-code-mapping.csv")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, err)
		return
	}
	defer f.Close()

	var mapping []officeRegionMapping
	s := bufio.NewScanner(f)
	s.Scan() // skip first line
	for s.Scan() {
		ret := strings.Split(s.Text(), ",")
		glb := glbMapping[ret[0]]
		mapping = append(mapping, officeRegionMapping{ret[1], glb.serviceCode, glb.regionID})
	}
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	err = enc.Encode(mapping)
	if err != nil {
		fmt.Fprintln(w, err)
		return
	}
}

func (h *handler) postIPRoutingInfoCfg(w http.ResponseWriter, r *http.Request) {
	type ipRoutingInfo struct {
		ServiceCode    string `json:"serviceCode"`
		RegionID       string `json:"regionId"`
		NetmaskAddress string `json:"netmaskAddress"`
	}

	var infos []ipRoutingInfo
	dec := json.NewDecoder(r.Body)
	err := dec.Decode(&infos)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, err)
		return
	}

	for _, i := range infos {
		log.Printf("[%s, %s, %s]", i.ServiceCode, i.RegionID, i.NetmaskAddress)
	}

	w.WriteHeader(http.StatusCreated)
}

func main() {
	h := &handler{}
	api := mux.NewRouter()
	api.HandleFunc("/mapping/officeGLBNode", h.getOfficeGLBNodeMapping).Methods("GET")
	api.HandleFunc("/mapping/glbNodeRegion", h.getGLBNodeRegionMapping).Methods("GET")
	api.HandleFunc("/mapping/officeRegion", h.getOfficeRegionMapping).Methods("GET")
	api.HandleFunc("/import/IPRoutingInfoCfg/1", h.postIPRoutingInfoCfg).Methods("POST")
	http.ListenAndServe(":8085", api)
}