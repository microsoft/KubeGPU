package utils

import (
	"fmt"
	"net"
	"reflect"
	"sort"
)

func LocalIPsWithoutLoopback() ([]net.IP, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("could not list network interfaces: %v", err)
	}
	var ips []net.IP
	for _, i := range interfaces {
		addresses, err := i.Addrs()
		if err != nil {
			return nil, fmt.Errorf("could not list the addresses for network interface %v: %v\n", i, err)
		}
		for _, address := range addresses {
			switch v := address.(type) {
			case *net.IPNet:
				if !v.IP.IsLoopback() {
					ips = append(ips, v.IP)
				}
			}
		}
	}
	return ips, nil
}

// sorted string keys
func SortedStringKeys(x interface{}) []string {
	t := reflect.TypeOf(x)
	keys := []string{}
	if t.Kind() == reflect.Map {
		mv := reflect.ValueOf(x)
		keysV := mv.MapKeys()
		for _, val := range keysV {
			keys = append(keys, val.String())
		}
		sort.Strings(keys)
		return keys
	}
	panic("Not a map")
}
