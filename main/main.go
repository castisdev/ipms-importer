package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
)

type IPMSSort []*IPMSRecord

func (s IPMSSort) Len() int {
	return len(s)
}
func (s IPMSSort) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s IPMSSort) Less(i, j int) bool {
	if s[i].rcode != s[j].rcode {
		return s[i].rcode < s[j].rcode
	}
	return s[i].ipStartInt < s[j].ipStartInt
}

type IPMSSort2 []*IPMSRecord

func (s IPMSSort2) Len() int {
	return len(s)
}
func (s IPMSSort2) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s IPMSSort2) Less(i, j int) bool {
	return len(s[i].child) > len(s[j].child)
}

func int2ip(nn uint32) net.IP {
	ip := make(net.IP, 4)
	binary.BigEndian.PutUint32(ip, nn)
	return ip
}

func PrintStartEndIP(cidr string) {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		fmt.Println(err)
		return
	}
	startIP := ip.Mask(ipnet.Mask)
	prefix, _ := ipnet.Mask.Size()
	endIP := EndIP(startIP, prefix)
	//fmt.Println(ipnet, startIP, endIP)
	fmt.Println(startIP, endIP)
}

type IPMSRecord struct {
	ipStart    net.IP
	ipStartInt int
	prefix     int
	ncode      string
	rcode      string
	cidr       string
	ipnet      *net.IPNet
	child      []*IPMSRecord
}

func NewRecord(ncode, rcode, ipStart, prefix string) (*IPMSRecord, error) {
	rec := &IPMSRecord{
		ncode: ncode,
		rcode: rcode,
	}

	var err error
	rec.prefix, err = strconv.Atoi(prefix)
	if err != nil {
		return nil, err
	}

	cidr := fmt.Sprintf("%s/%d", ipStart, rec.prefix)
	var ip net.IP
	ip, rec.ipnet, err = net.ParseCIDR(cidr)
	rec.ipStart = ip.Mask(rec.ipnet.Mask)
	rec.ipStartInt = int(binary.BigEndian.Uint32(rec.ipStart))
	if err != nil {
		return nil, err
	}
	return rec, nil
}

func (r *IPMSRecord) NextStartIP() net.IP {
	startInt := binary.BigEndian.Uint32(r.ipStart)
	endInt := startInt + (1 << uint(32-r.prefix))
	return int2ip(endInt)
}

func (r *IPMSRecord) CanStart() bool {
	ip, ipnet, err := net.ParseCIDR(fmt.Sprintf("%v/%d", r.ipStart, r.prefix-1))
	if err != nil {
		return false
	}
	return ip.Mask(ipnet.Mask).String() == r.ipStart.String()
}

type ContSet []*IPMSRecord

func (set ContSet) Fprintln(w io.Writer) {
	for _, rec := range set {
		fmt.Fprintf(w, "%s, %v", rec.rcode, rec.ipnet)
		if len(rec.child) != 0 {
			fmt.Fprintf(w, ", [%v, %v, %d]", rec.child[0].ipnet, rec.child[len(rec.child)-1].ipnet, len(rec.child))
		}
		fmt.Fprintln(w)
	}
}

func NewParent(first, second *IPMSRecord) (*IPMSRecord, error) {
	rec := &IPMSRecord{
		ncode:  first.ncode,
		rcode:  first.rcode,
		prefix: first.prefix - 1,
	}
	if len(first.child) != 0 {
		rec.child = append(rec.child, first.child...)
	} else {
		rec.child = append(rec.child, first)
	}
	if len(second.child) != 0 {
		rec.child = append(rec.child, second.child...)
	} else {
		rec.child = append(rec.child, second)
	}

	cidr := fmt.Sprintf("%v/%d", first.ipStart, rec.prefix)
	var ip net.IP
	var err error
	ip, rec.ipnet, err = net.ParseCIDR(cidr)
	rec.ipStart = ip.Mask(rec.ipnet.Mask)
	rec.ipStartInt = int(binary.BigEndian.Uint32(rec.ipStart))
	if err != nil {
		return nil, err
	}
	return rec, nil
}

func (set *ContSet) Sum() {
	if len(*set) <= 1 {
		return
	}
	var set2 ContSet
	for i := 0; i < len(*set); i++ {
		rec := (*set)[i]
		startInt := rec.ipStartInt >> uint(32-rec.prefix)
		if startInt%2 != 0 || i == len(*set)-1 || rec.prefix != (*set)[i+1].prefix {
			set2 = append(set2, rec)
			continue
		}
		newRec, err := NewParent(rec, (*set)[i+1])
		if err != nil {
			set2 = append(set2, rec)
			continue
		}
		set2 = append(set2, newRec)
		i++
	}

	if len(*set) == len(set2) {
		return
	} else {
		*set = set2
		set.Sum()
	}
}

func (set *ContSet) Add(r *IPMSRecord) {
	*set = append(*set, r)
}

func (set ContSet) IsCont(r *IPMSRecord) bool {
	if set == nil || len(set) == 0 {
		return true
	}
	if set[0].rcode != r.rcode {
		return false
	}
	if set[0].ncode != r.ncode {
		return false
	}
	return set[len(set)-1].NextStartIP().String() == r.ipStart.String()
}

func main() {
	//PrintStartEndIP("211.55.254.128/27")
	//PrintStartEndIP("211.55.254.160/28")
	//PrintStartEndIP("211.55.254.176/28")
	//PrintStartEndIP("211.55.254.192/27")
	//PrintStartEndIP("211.55.254.224/27")
	//fmt.Println()
	//PrintStartEndIP("211.55.254.128/25")
	//return

	//PrintStartEndIP("211.218.232.0/22")
	//PrintStartEndIP("211.218.236.0/23")
	//PrintStartEndIP("14.63.175.0/24")
	//PrintStartEndIP("14.63.128.0/19")

	f, err := os.Open("/home/sasgas/Documents/cdn/FILE_TB_ASSIGN.DAT")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer f.Close()

	rcodes := make(map[string]int)
	var recs []*IPMSRecord
	s := bufio.NewScanner(f)
	for s.Scan() {
		ret := strings.Split(s.Text(), "|")
		rec, err := NewRecord(ret[1], ret[5], ret[0], ret[7])
		//rec, err := NewRecord("", "", ret[0], ret[7])
		rcodes[ret[5]]++
		if err != nil {
			fmt.Println(err)
			continue
		}
		recs = append(recs, rec)
	}

	if err := s.Err(); err != nil {
		fmt.Println(err)
		return
	}

	sort.Sort(IPMSSort(recs))

	o, err := os.Create("ipms.txt")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer o.Close()

	var set, set2 ContSet
	for _, rec := range recs {
		//fmt.Printf("%s, %v\n", rec.rcode, rec.ipnet)
		if set.IsCont(rec) {
			set.Add(rec)
		} else {
			set.Sum()
			for _, r := range set {
				set2.Add(r)
			}
			set.Fprintln(o)
			set = set[:0]
			set.Add(rec)
		}
	}

	for k, v := range rcodes {
		fmt.Println(k, v)
	}
	fmt.Println(len(rcodes))

	sort.Sort(IPMSSort2(set2))
	set3 := set2[:10]
	set3.Fprintln(os.Stdout)

	or, err := os.Create("ipRoutingInfo.cfg")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer or.Close()

	sort.Sort(IPMSSort(set2))
	regions := []string{"A000", "B001", "C002", "D003"}
	rcodeRegionMap := make(map[string]string)
	for _, rec := range set2 {
		val, ok := rcodeRegionMap[rec.rcode]
		if ok == false {
			rcodeRegionMap[rec.rcode] = regions[rand.Intn(len(regions))]
			val = rcodeRegionMap[rec.rcode]
		}
		fmt.Fprintf(or, "%s\t%s\t%s\n", "OTM", rec.ipnet, val)
	}
	rcodeRegionMap = make(map[string]string)
	for _, rec := range set2 {
		val, ok := rcodeRegionMap[rec.rcode]
		if ok == false {
			rcodeRegionMap[rec.rcode] = regions[rand.Intn(len(regions))]
			val = rcodeRegionMap[rec.rcode]
		}
		fmt.Fprintf(or, "%s\t%s\t%s\n", "SKYLIFE", rec.ipnet, val)
	}
}

func EndIP(startIP net.IP, prefix int) net.IP {
	startInt := binary.BigEndian.Uint32(startIP)
	endInt := startInt
	for i := 0; i < (32 - prefix); i++ {
		endInt += (1 << uint(i))
	}
	return int2ip(endInt)
}
