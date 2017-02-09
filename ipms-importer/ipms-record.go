package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"strconv"

	"github.com/castisdev/cilog"
)

type ipmsSort []*ipmsRecord

func (s ipmsSort) Len() int {
	return len(s)
}
func (s ipmsSort) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s ipmsSort) Less(i, j int) bool {
	if s[i].ServiceCode != s[j].ServiceCode {
		return s[i].ServiceCode < s[j].ServiceCode
	}
	if s[i].RegionID != s[j].RegionID {
		return s[i].RegionID < s[j].RegionID
	}
	return s[i].ipStartInt < s[j].ipStartInt
}

func int2ip(nn uint32) net.IP {
	ip := make(net.IP, 4)
	binary.BigEndian.PutUint32(ip, nn)
	return ip
}

type ipmsRecord struct {
	ipStart     net.IP
	ipStartInt  int
	prefix      int
	ServiceCode string `json:"serviceCode"`
	RegionID    string `json:"regionId"`
	officeCode  string
	CIDR        string `json:"netMaskAddress"`
	ipnet       *net.IPNet
}

func newRecord(serviceCode, regionID, officeCode, ipStart, prefix string) (*ipmsRecord, error) {
	rec := &ipmsRecord{
		ServiceCode: serviceCode,
		RegionID:    regionID,
		officeCode:  officeCode,
	}

	var err error
	rec.prefix, err = strconv.Atoi(prefix)
	if err != nil {
		return nil, err
	}

	rec.CIDR = fmt.Sprintf("%s/%d", ipStart, rec.prefix)
	var ip net.IP
	ip, rec.ipnet, err = net.ParseCIDR(rec.CIDR)
	rec.ipStart = ip.Mask(rec.ipnet.Mask)
	rec.ipStartInt = int(binary.BigEndian.Uint32(rec.ipStart))
	if err != nil {
		return nil, err
	}
	return rec, nil
}

func (r *ipmsRecord) NextStartIP() net.IP {
	startInt := binary.BigEndian.Uint32(r.ipStart)
	endInt := startInt + (1 << uint(32-r.prefix))
	return int2ip(endInt)
}

func (r *ipmsRecord) CanStart() bool {
	ip, ipnet, err := net.ParseCIDR(fmt.Sprintf("%v/%d", r.ipStart, r.prefix-1))
	if err != nil {
		return false
	}
	return ip.Mask(ipnet.Mask).String() == r.ipStart.String()
}

type contSet []*ipmsRecord

func (set contSet) printLog() {
	for _, rec := range set {
		cilog.Debugf("%s, %s, %s, %v", rec.ServiceCode, rec.RegionID, rec.officeCode, rec.ipnet)
	}
}

func newParent(first, second *ipmsRecord) (*ipmsRecord, error) {
	rec := &ipmsRecord{
		ServiceCode: first.ServiceCode,
		RegionID:    first.RegionID,
		officeCode:  first.officeCode,
		prefix:      first.prefix - 1,
	}

	rec.CIDR = fmt.Sprintf("%v/%d", first.ipStart, rec.prefix)
	var ip net.IP
	var err error
	ip, rec.ipnet, err = net.ParseCIDR(rec.CIDR)
	rec.ipStart = ip.Mask(rec.ipnet.Mask)
	rec.ipStartInt = int(binary.BigEndian.Uint32(rec.ipStart))
	if err != nil {
		return nil, err
	}
	cilog.Debugf("[%s, %s] merge [%v, %v] to [%v]", rec.ServiceCode, rec.RegionID, first.ipnet, second.ipnet, rec.ipnet)
	return rec, nil
}

func (set *contSet) Sum() {
	if len(*set) <= 1 {
		return
	}
	var set2 contSet
	for i := 0; i < len(*set); i++ {
		rec := (*set)[i]
		startInt := rec.ipStartInt >> uint(32-rec.prefix)
		if startInt%2 != 0 || i == len(*set)-1 || rec.prefix != (*set)[i+1].prefix {
			set2 = append(set2, rec)
			continue
		}
		newRec, err := newParent(rec, (*set)[i+1])
		if err != nil {
			set2 = append(set2, rec)
			continue
		}
		set2 = append(set2, newRec)
		i++
	}

	if len(*set) == len(set2) {
		return
	}

	*set = set2
	set.Sum()
}

func (set *contSet) Add(r *ipmsRecord) {
	*set = append(*set, r)
}

func (set contSet) IsCont(r *ipmsRecord) bool {
	if set == nil || len(set) == 0 {
		return true
	}
	if set[0].ServiceCode != r.ServiceCode {
		return false
	}
	if set[0].RegionID != r.RegionID {
		return false
	}
	return set[len(set)-1].NextStartIP().String() == r.ipStart.String()
}
