package kish

import (
	"net"
)

type IPSet struct {
	Nets []net.IPNet
}

func (is *IPSet) String() string {
	s := ""
	for i, val := range is.Nets {
		s += val.String()
		if i+1 < len(is.Nets) {
			s += ","
		}
	}
	return s
}

func (is *IPSet) Add(s string) error {
	_, pnet, err := net.ParseCIDR(s)
	if err != nil {
		return err
	}
	is.Nets = append(is.Nets, *pnet)
	return nil
}

func (is *IPSet) Contains(ip net.IP) bool {
	for _, n := range is.Nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

func (is *IPSet) ContainsIPString(s string) bool {
	pip := net.ParseIP(s)
	if pip == nil {
		return false
	}
	return is.Contains(pip)
}

func (is *IPSet) ContainsHostPort(s string) (bool, string, string) {
	host, port, err := net.SplitHostPort(s)
	if err != nil {
		return false, "", ""
	}
	pip := net.ParseIP(host)
	if pip == nil {
		return false, host, port
	}
	return is.Contains(pip), host, port
}
