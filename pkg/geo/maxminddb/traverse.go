package maxminddb

import (
	"errors"
	"fmt"
	"reflect"

	// comment to prevent gofumpt from randomly moving iter.
	"iter"
	"net/netip"
)

// Internal structure used to keep track of nodes we still need to visit.
type netNode struct {
	ip      netip.Addr
	bit     uint
	pointer uint
}

type networkOptions struct {
	includeAliasedNetworks bool
	includeEmptyNetworks   bool
}

var (
	allIPv4 = netip.MustParsePrefix("0.0.0.0/0")
	allIPv6 = netip.MustParsePrefix("::/0")
)

// NetworksOption are options for Networks and NetworksWithin.
type NetworksOption func(*networkOptions)

// IncludeAliasedNetworks is an option for Networks and NetworksWithin
// that makes them iterate over aliases of the IPv4 subtree in an IPv6
// database, e.g., ::ffff:0:0/96, 2001::/32, and 2002::/16.
func IncludeAliasedNetworks(networks *networkOptions) {
	networks.includeAliasedNetworks = true
}

// IncludeNetworksWithoutData is an option for Networks and NetworksWithin
// that makes them include networks without any data in the iteration.
func IncludeNetworksWithoutData(networks *networkOptions) {
	networks.includeEmptyNetworks = true
}

// Networks returns an iterator that can be used to traverse the networks in
// the database.
//
// Please note that a MaxMind DB may map IPv4 networks into several locations
// in an IPv6 database. This iterator will only iterate over these once by
// default. To iterate over all the IPv4 network locations, use the
// [IncludeAliasedNetworks] option.
//
// Networks without data are excluded by default. To include them, use
// [IncludeNetworksWithoutData].
func (r *Reader) Networks(options ...NetworksOption) iter.Seq[Result] {
	if r.Metadata.IPVersion == 6 {
		return r.NetworksWithin(allIPv6, options...)
	}
	return r.NetworksWithin(allIPv4, options...)
}

// NetworksWithin returns an iterator that can be used to traverse the networks
// in the database which are contained in a given prefix.
//
// Please note that a MaxMind DB may map IPv4 networks into several locations
// in an IPv6 database. This iterator will only iterate over these once by
// default. To iterate over all the IPv4 network locations, use the
// [IncludeAliasedNetworks] option.
//
// If the provided prefix is contained within a network in the database, the
// iterator will iterate over exactly one network, the containing network.
//
// Networks without data are excluded by default. To include them, use
// [IncludeNetworksWithoutData].
func (r *Reader) NetworksWithin(prefix netip.Prefix, options ...NetworksOption) iter.Seq[Result] {
	return func(yield func(Result) bool) {
		if r.Metadata.IPVersion == 4 && prefix.Addr().Is6() {
			yield(Result{
				err: fmt.Errorf(
					"error getting networks with '%s': you attempted to use an IPv6 network in an IPv4-only database",
					prefix,
				),
			})
			return
		}

		n := &networkOptions{}
		for _, option := range options {
			option(n)
		}

		ip := prefix.Addr()
		netIP := ip
		stopBit := prefix.Bits()
		if ip.Is4() {
			netIP = v4ToV16(ip)
			stopBit += 96
		}

		pointer, bit := r.traverseTree(ip, 0, stopBit)

		prefix, err := netIP.Prefix(bit)
		if err != nil {
			yield(Result{
				ip:        ip,
				prefixLen: uint8(bit),
				err:       fmt.Errorf("prefixing %s with %d", netIP, bit),
			})
			return
		}

		nodes := make([]netNode, 0, 64)
		nodes = append(nodes,
			netNode{
				ip:      prefix.Addr(),
				bit:     uint(bit),
				pointer: pointer,
			},
		)

		for len(nodes) > 0 {
			node := nodes[len(nodes)-1]
			nodes = nodes[:len(nodes)-1]

			for {
				if node.pointer == r.Metadata.NodeCount {
					if n.includeEmptyNetworks {
						ok := yield(Result{
							ip:        mappedIP(node.ip),
							offset:    notFound,
							prefixLen: uint8(node.bit),
						})
						if !ok {
							return
						}
					}
					break
				}
				// This skips IPv4 aliases without hardcoding the networks that the writer
				// currently aliases.
				if !n.includeAliasedNetworks && r.ipv4Start != 0 &&
					node.pointer == r.ipv4Start && !isInIPv4Subtree(node.ip) {
					break
				}

				if node.pointer > r.Metadata.NodeCount {
					offset, err := r.resolveDataPointer(node.pointer)
					ok := yield(Result{
						decoder:   r.decoder,
						ip:        mappedIP(node.ip),
						offset:    uint(offset),
						prefixLen: uint8(node.bit),
						err:       err,
					})
					if !ok {
						return
					}
					break
				}
				ipRight := node.ip.As16()
				if len(ipRight) <= int(node.bit>>3) {
					displayAddr := node.ip
					if isInIPv4Subtree(node.ip) {
						displayAddr = v6ToV4(displayAddr)
					}

					res := Result{
						ip:        displayAddr,
						prefixLen: uint8(node.bit),
					}
					res.err = newInvalidDatabaseError(
						"invalid search tree at %s", res.Prefix())

					yield(res)

					return
				}
				ipRight[node.bit>>3] |= 1 << (7 - (node.bit % 8))

				offset := node.pointer * r.nodeOffsetMult
				rightPointer := r.nodeReader.readRight(offset)

				node.bit++
				nodes = append(nodes, netNode{
					pointer: rightPointer,
					ip:      netip.AddrFrom16(ipRight),
					bit:     node.bit,
				})

				node.pointer = r.nodeReader.readLeft(offset)
			}
		}
	}
}

var ipv4SubtreeBoundary = netip.MustParseAddr("::255.255.255.255").Next()

func mappedIP(ip netip.Addr) netip.Addr {
	if isInIPv4Subtree(ip) {
		return v6ToV4(ip)
	}
	return ip
}

// isInIPv4Subtree returns true if the IP is in the database's IPv4 subtree.
func isInIPv4Subtree(ip netip.Addr) bool {
	return ip.Is4() || ip.Less(ipv4SubtreeBoundary)
}

// We store IPv4 addresses at ::/96 for unclear reasons.
func v4ToV16(ip netip.Addr) netip.Addr {
	b4 := ip.As4()
	var b16 [16]byte
	copy(b16[12:], b4[:])
	return netip.AddrFrom16(b16)
}

// Converts an IPv4 address embedded in IPv6 to IPv4.
func v6ToV4(ip netip.Addr) netip.Addr {
	b := ip.As16()
	v, _ := netip.AddrFromSlice(b[12:])
	return v
}

// Data represents a set of data entries that we are iterating over.
type Data struct {
	reader *Reader
	offset uint
	err    error
}

// Data returns an iterator over all data sections in the database.
func (r *Reader) Data() *Data {
	return &Data{reader: r}
}

// Next returns true if there are more sections to be processed, and false if
// there are no more sections or if there is an error.
func (d *Data) Next() bool {
	if d.err != nil {
		return false
	}
	return d.offset < uint(len(d.reader.decoder.buffer))
}

// Err returns an error, if any, that was encountered during iteration.
func (d *Data) Err() error {
	return d.err
}

// Data returns the current data section or an error if there is a problem
// decoding the data.
//
// v should be a pointer.
func (d *Data) Data(v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return errors.New("result param must be a pointer")
	}

	o, err := d.reader.decoder.decode(d.offset, rv, 0)
	if err != nil {
		return err
	}

	d.offset = o
	d.err = err
	return nil
}
