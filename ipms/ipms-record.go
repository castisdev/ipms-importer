package ipms

import (
	"encoding/binary"
	"fmt"
	"net"
	"strconv"

	"github.com/castisdev/cilog"
)

type ipmsSort []*IpmsRecord

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
	if s[i].GLBID != s[j].GLBID {
		return s[i].GLBID < s[j].GLBID
	}
	if s[i].NetCode != s[j].NetCode {
		return s[i].NetCode < s[j].NetCode
	}
	return s[i].IPStartInt < s[j].IPStartInt
}

type ipmsSort2 []*IpmsRecord

func (s ipmsSort2) Len() int {
	return len(s)
}
func (s ipmsSort2) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s ipmsSort2) Less(i, j int) bool {
	return s[i].IPStartInt < s[j].IPStartInt
}

func int2ip(nn uint32) net.IP {
	ip := make(net.IP, 4)
	binary.BigEndian.PutUint32(ip, nn)
	return ip
}

// IpmsRecord :
type IpmsRecord struct {
	IPStart     net.IP
	IPStartInt  int
	Prefix      int
	ServiceCode string `json:"serviceCode"`
	GLBID       string `json:"glbId"`
	NetCode     string `json:"netCode"`
	OfficeCode  string
	CIDR        string `json:"netMaskAddress"`
	IPNet       *net.IPNet
}

func simpleMaskLength(mask net.IPMask) int {
	var n int
	for i, v := range mask {
		if v == 0xff {
			n += 8
			continue
		}
		// found non-ff byte
		// count 1 bits
		for v&0x80 != 0 {
			n++
			v <<= 1
		}
		// rest must be 0 bits
		if v != 0 {
			return -1
		}
		for i++; i < len(mask); i++ {
			if mask[i] != 0 {
				return -1
			}
		}
		break
	}
	return n
}

// NewRecordFromCIDR :
func NewRecordFromCIDR(serviceCode, glbID, netCode, officeCode string, cidr *net.IPNet) (*IpmsRecord, error) {
	rec := &IpmsRecord{
		ServiceCode: serviceCode,
		NetCode:     netCode,
		GLBID:       glbID,
		OfficeCode:  officeCode,
	}

	rec.Prefix = simpleMaskLength(cidr.Mask)

	rec.CIDR = cidr.String()
	rec.IPNet = cidr
	rec.IPStart = cidr.IP
	rec.IPStartInt = int(binary.BigEndian.Uint32(rec.IPStart))

	return rec, nil
}

// NewRecord :
func NewRecord(serviceCode, glbID, netCode, officeCode, ipStart, prefix string) (*IpmsRecord, error) {
	rec := &IpmsRecord{
		ServiceCode: serviceCode,
		NetCode:     netCode,
		GLBID:       glbID,
		OfficeCode:  officeCode,
	}

	var err error
	rec.Prefix, err = strconv.Atoi(prefix)
	if err != nil {
		return nil, err
	}

	rec.CIDR = fmt.Sprintf("%s/%d", ipStart, rec.Prefix)
	var ip net.IP
	ip, rec.IPNet, err = net.ParseCIDR(rec.CIDR)
	rec.IPStart = ip.Mask(rec.IPNet.Mask)
	rec.IPStartInt = int(binary.BigEndian.Uint32(rec.IPStart))
	if err != nil {
		return nil, err
	}
	return rec, nil
}

func (r *IpmsRecord) nextStartIP() net.IP {
	startInt := binary.BigEndian.Uint32(r.IPStart)
	endInt := startInt + (1 << uint(32-r.Prefix))
	return int2ip(endInt)
}

func (r *IpmsRecord) canStart() bool {
	ip, ipnet, err := net.ParseCIDR(fmt.Sprintf("%v/%d", r.IPStart, r.Prefix-1))
	if err != nil {
		return false
	}
	return ip.Mask(ipnet.Mask).String() == r.IPStart.String()
}

type contSet []*IpmsRecord

func (set contSet) printLog() {
	for _, rec := range set {
		cilog.Infof("success to parse ipms data, serviceCode[%s], glbId[%s], netCode[%s], netMask[%v]", rec.ServiceCode, rec.GLBID, rec.NetCode, rec.IPNet)
	}
}

func newParent(first, second *IpmsRecord) (*IpmsRecord, error) {
	rec := &IpmsRecord{
		ServiceCode: first.ServiceCode,
		NetCode:     first.NetCode,
		GLBID:       first.GLBID,
		OfficeCode:  first.OfficeCode,
		Prefix:      first.Prefix - 1,
	}

	rec.CIDR = fmt.Sprintf("%v/%d", first.IPStart, rec.Prefix)
	var ip net.IP
	var err error
	ip, rec.IPNet, err = net.ParseCIDR(rec.CIDR)
	rec.IPStart = ip.Mask(rec.IPNet.Mask)
	rec.IPStartInt = int(binary.BigEndian.Uint32(rec.IPStart))
	if err != nil {
		return nil, err
	}
	cilog.Debugf("[%s, %s, %s] merge [%v, %v] to [%v]", rec.ServiceCode, rec.GLBID, rec.NetCode, first.IPNet, second.IPNet, rec.IPNet)
	return rec, nil
}

func (set *contSet) Sum() {
	if len(*set) <= 1 {
		return
	}
	cilog.Debugf("merge start")
	var set2 contSet
	for i := 0; i < len(*set); i++ {
		rec := (*set)[i]
		startInt := rec.IPStartInt >> uint(32-rec.Prefix)
		if startInt%2 != 0 || i == len(*set)-1 || rec.Prefix != (*set)[i+1].Prefix {
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
	cilog.Debugf("merge done")

	if len(*set) == len(set2) {
		return
	}

	*set = set2
	set.Sum()
}

func (set *contSet) Add(r *IpmsRecord) {
	*set = append(*set, r)
}

func (set contSet) IsCont(r *IpmsRecord) bool {
	if set == nil || len(set) == 0 {
		return true
	}
	if set[0].ServiceCode != r.ServiceCode {
		return false
	}
	if set[0].GLBID != r.GLBID {
		return false
	}
	if set[0].NetCode != r.NetCode {
		return false
	}
	return set[len(set)-1].nextStartIP().String() == r.IPStart.String()
}
