package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/mux"
)

type handler struct{}

type officeGLBNodeMapping struct {
	OfficeCode string `json:"officeCode"`
	GLBNode    string `json:"glbNode"`
}

func (h *handler) getOfficeGLBNodeMapping(w http.ResponseWriter, r *http.Request) {
	f, err := os.Open("office-code-mapping.csv")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, err)
		return
	}
	defer f.Close()

	var mapping []officeGLBNodeMapping
	s := bufio.NewScanner(f)
	s.Scan() // skip first line
	for s.Scan() {
		ret := strings.Split(s.Text(), ",")
		mapping = append(mapping, officeGLBNodeMapping{ret[1], ret[0]})
	}
	w.Header().Set("Content-Type", "application/json")
	// w.WriteHeader(http.StatusOK)
	enc := json.NewEncoder(w)
	err = enc.Encode(mapping)
	if err != nil {
		fmt.Fprintln(w, err)
		return
	}
}

type glbNodeRegionMapping struct {
	GLBNode     string `json:"glbNode"`
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

	var mapping []glbNodeRegionMapping
	s := bufio.NewScanner(f)
	s.Scan() // skip first line
	for s.Scan() {
		ret := strings.Split(s.Text(), ",")
		mapping = append(mapping, glbNodeRegionMapping{ret[0], ret[1], ret[2]})
	}
	w.Header().Set("Content-Type", "application/json")
	// w.WriteHeader(http.StatusOK)
	enc := json.NewEncoder(w)
	err = enc.Encode(mapping)
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
	// w.WriteHeader(http.StatusOK)
	enc := json.NewEncoder(w)
	err = enc.Encode(mapping)
	if err != nil {
		fmt.Fprintln(w, err)
		return
	}
}

func main() {
	h := &handler{}
	api := mux.NewRouter()
	api.HandleFunc("/api/office-glb-mappings", h.getOfficeGLBNodeMapping).Methods("GET")
	api.HandleFunc("/api/glb-region-mappings", h.getGLBNodeRegionMapping).Methods("GET")
	api.HandleFunc("/api/office-region-mappings", h.getOfficeRegionMapping).Methods("GET")
	http.ListenAndServe(":8085", api)
}
