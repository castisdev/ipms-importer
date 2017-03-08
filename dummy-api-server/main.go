package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/mux"
)

type handler struct{}

type officeNodeMapping struct {
	OfficeCode string `json:"officeCode"`
	NodeCode   string `json:"nodeCode"`
}

func (h *handler) getOfficeNodeMapping(w http.ResponseWriter, r *http.Request) {
	f, err := os.Open("office-code-mapping.csv")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, err)
		return
	}
	defer f.Close()

	var m struct {
		List []officeNodeMapping `json:"officeNodeMappingList"`
	}
	s := bufio.NewScanner(f)
	s.Scan() // skip first line
	for s.Scan() {
		ret := strings.Split(s.Text(), ",")
		m.List = append(m.List, officeNodeMapping{ret[1], ret[0]})
	}
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	err = enc.Encode(m)
	if err != nil {
		fmt.Fprintln(w, err)
		return
	}
}

type nodeGLBIDMapping struct {
	NodeCode    string `json:"nodeCode"`
	ServiceCode string `json:"serviceCode"`
	GLBID       string `json:"glbId"`
}

func (h *handler) getNodeGLBIDMapping(w http.ResponseWriter, r *http.Request) {
	f, err := os.Open("glb-mapping.csv")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, err)
		return
	}
	defer f.Close()

	var m struct {
		List []nodeGLBIDMapping `json:"nodeGLBIdMappingList"`
	}
	s := bufio.NewScanner(f)
	s.Scan() // skip first line
	for s.Scan() {
		ret := strings.Split(s.Text(), ",")
		m.List = append(m.List, nodeGLBIDMapping{ret[0], ret[1], ret[2]})
	}
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	err = enc.Encode(m)
	if err != nil {
		fmt.Fprintln(w, err)
		return
	}
}

func (h *handler) postIPRoutingInfoCfg(w http.ResponseWriter, r *http.Request) {
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

	var infos []serviceCodeInfo
	dec := json.NewDecoder(r.Body)
	err := dec.Decode(&infos)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, err)
		return
	}

	cnt := 0
	for _, s := range infos {
		serviceCode := s.ServiceCode
		for _, r := range s.GLBIDNetMaskList {
			regionID := r.GLBID
			for _, n := range r.NetMaskAddressList {
				log.Printf("[%s, %s, %s, %s]", serviceCode, regionID, n.NetCode, n.NetMaskAddress)
				cnt++
			}
		}
	}
	log.Printf("total %d lines", cnt)

	w.WriteHeader(http.StatusCreated)

	f, err := os.Create("ipms.json")
	if err != nil {
		log.Fatal(err)
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	err = enc.Encode(infos)
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	addr := flag.String("addr", ":8780", "listen address")
	flag.Parse()

	h := &handler{}
	api := mux.NewRouter()
	api.HandleFunc("/mapping/officeNode", h.getOfficeNodeMapping).Methods("GET")
	api.HandleFunc("/mapping/nodeGLBId", h.getNodeGLBIDMapping).Methods("GET")
	api.HandleFunc("/import/ipms", h.postIPRoutingInfoCfg).Methods("POST")
	http.ListenAndServe(*addr, api)
}
