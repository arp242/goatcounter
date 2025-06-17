// Package maxminddb provides a reader for the MaxMind DB file format.
package maxminddb

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"os"
	"reflect"
	"runtime"
)

const dataSectionSeparatorSize = 16

var metadataStartMarker = []byte("\xAB\xCD\xEFMaxMind.com")

// Reader holds the data corresponding to the MaxMind DB file. Its only public
// field is Metadata, which contains the metadata from the MaxMind DB file.
//
// All of the methods on Reader are thread-safe. The struct may be safely
// shared across goroutines.
type Reader struct {
	nodeReader        nodeReader
	buffer            []byte
	decoder           decoder
	Metadata          Metadata
	Path              string
	ipv4Start         uint
	ipv4StartBitDepth int
	nodeOffsetMult    uint
	hasMappedFile     bool
}

// Metadata holds the metadata decoded from the MaxMind DB file. In particular
// it has the format version, the build time as Unix epoch time, the database
// type and description, the IP version supported, and a slice of the natural
// languages included.
type Metadata struct {
	Description              map[string]string `maxminddb:"description"`
	DatabaseType             string            `maxminddb:"database_type"`
	Languages                []string          `maxminddb:"languages"`
	BinaryFormatMajorVersion uint              `maxminddb:"binary_format_major_version"`
	BinaryFormatMinorVersion uint              `maxminddb:"binary_format_minor_version"`
	BuildEpoch               uint              `maxminddb:"build_epoch"`
	IPVersion                uint              `maxminddb:"ip_version"`
	NodeCount                uint              `maxminddb:"node_count"`
	RecordSize               uint              `maxminddb:"record_size"`
}

// Open takes a string path to a MaxMind DB file and returns a Reader
// structure or an error. The database file is opened using a memory map
// on supported platforms. On platforms without memory map support, such
// as WebAssembly or Google App Engine, or if the memory map attempt fails
// due to lack of support from the filesystem, the database is loaded into memory.
// Use the Close method on the Reader object to return the resources to the system.
func Open(file string) (*Reader, error) {
	mapFile, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer mapFile.Close()

	stats, err := mapFile.Stat()
	if err != nil {
		return nil, err
	}

	size64 := stats.Size()
	// mmapping an empty file returns -EINVAL on Unix platforms,
	// and ERROR_FILE_INVALID on Windows.
	if size64 == 0 {
		return nil, errors.New("file is empty")
	}

	size := int(size64)
	// Check for overflow.
	if int64(size) != size64 {
		return nil, errors.New("file too large")
	}

	data, err := mmap(int(mapFile.Fd()), size)
	if err != nil {
		if errors.Is(err, errors.ErrUnsupported) {
			data, err = openFallback(mapFile, size)
			if err != nil {
				return nil, err
			}
			return FromBytes(data)
		}
		return nil, err
	}

	reader, err := FromBytes(data)
	if err != nil {
		_ = munmap(data)
		return nil, err
	}
	reader.Path = file

	reader.hasMappedFile = true
	runtime.SetFinalizer(reader, (*Reader).Close)
	return reader, nil
}

func openFallback(f *os.File, size int) (data []byte, err error) {
	data = make([]byte, size)
	_, err = io.ReadFull(f, data)
	return data, err
}

// Close returns the resources used by the database to the system.
func (r *Reader) Close() error {
	var err error
	if r.hasMappedFile {
		runtime.SetFinalizer(r, nil)
		r.hasMappedFile = false
		err = munmap(r.buffer)
	}
	r.buffer = nil
	return err
}

// FromBytes takes a byte slice corresponding to a MaxMind DB file and returns
// a Reader structure or an error.
func FromBytes(buffer []byte) (*Reader, error) {
	metadataStart := bytes.LastIndex(buffer, metadataStartMarker)

	if metadataStart == -1 {
		return nil, newInvalidDatabaseError("error opening database: invalid MaxMind DB file")
	}

	metadataStart += len(metadataStartMarker)
	metadataDecoder := decoder{buffer: buffer[metadataStart:]}

	var metadata Metadata

	rvMetadata := reflect.ValueOf(&metadata)
	_, err := metadataDecoder.decode(0, rvMetadata, 0)
	if err != nil {
		return nil, err
	}

	searchTreeSize := metadata.NodeCount * (metadata.RecordSize / 4)
	dataSectionStart := searchTreeSize + dataSectionSeparatorSize
	dataSectionEnd := uint(metadataStart - len(metadataStartMarker))
	if dataSectionStart > dataSectionEnd {
		return nil, newInvalidDatabaseError("the MaxMind DB contains invalid metadata")
	}
	d := decoder{
		buffer: buffer[searchTreeSize+dataSectionSeparatorSize : metadataStart-len(metadataStartMarker)],
	}

	nodeBuffer := buffer[:searchTreeSize]
	var nodeReader nodeReader
	switch metadata.RecordSize {
	case 24:
		nodeReader = nodeReader24{buffer: nodeBuffer}
	case 28:
		nodeReader = nodeReader28{buffer: nodeBuffer}
	case 32:
		nodeReader = nodeReader32{buffer: nodeBuffer}
	default:
		return nil, newInvalidDatabaseError("unknown record size: %d", metadata.RecordSize)
	}

	reader := &Reader{
		buffer:         buffer,
		nodeReader:     nodeReader,
		decoder:        d,
		Metadata:       metadata,
		ipv4Start:      0,
		nodeOffsetMult: metadata.RecordSize / 4,
	}

	reader.setIPv4Start()

	return reader, err
}

func (r *Reader) setIPv4Start() {
	if r.Metadata.IPVersion != 6 {
		r.ipv4StartBitDepth = 96
		return
	}

	nodeCount := r.Metadata.NodeCount

	node := uint(0)
	i := 0
	for ; i < 96 && node < nodeCount; i++ {
		node = r.nodeReader.readLeft(node * r.nodeOffsetMult)
	}
	r.ipv4Start = node
	r.ipv4StartBitDepth = i
}

// Lookup retrieves the database record for ip and returns Result, which can
// be used to decode the data..
func (r *Reader) Lookup(ip netip.Addr) Result {
	if r.buffer == nil {
		return Result{err: errors.New("cannot call Lookup on a closed database")}
	}
	pointer, prefixLen, err := r.lookupPointer(ip)
	if err != nil {
		return Result{
			ip:        ip,
			prefixLen: uint8(prefixLen),
			err:       err,
		}
	}
	if pointer == 0 {
		return Result{
			ip:        ip,
			prefixLen: uint8(prefixLen),
			offset:    notFound,
		}
	}
	offset, err := r.resolveDataPointer(pointer)
	return Result{
		decoder:   r.decoder,
		ip:        ip,
		offset:    uint(offset),
		prefixLen: uint8(prefixLen),
		err:       err,
	}
}

// LookupOffset returns the Result for the specified offset. Note that
// netip.Prefix returned by Networks will be invalid when using LookupOffset.
func (r *Reader) LookupOffset(offset uintptr) Result {
	if r.buffer == nil {
		return Result{err: errors.New("cannot call Decode on a closed database")}
	}

	return Result{decoder: r.decoder, offset: uint(offset)}
}

var zeroIP = netip.MustParseAddr("::")

func (r *Reader) lookupPointer(ip netip.Addr) (uint, int, error) {
	if r.Metadata.IPVersion == 4 && ip.Is6() {
		return 0, 0, fmt.Errorf(
			"error looking up '%s': you attempted to look up an IPv6 address in an IPv4-only database",
			ip.String(),
		)
	}

	node, prefixLength := r.traverseTree(ip, 0, 128)

	nodeCount := r.Metadata.NodeCount
	if node == nodeCount {
		// Record is empty
		return 0, prefixLength, nil
	} else if node > nodeCount {
		return node, prefixLength, nil
	}

	return 0, prefixLength, newInvalidDatabaseError("invalid node in search tree")
}

func (r *Reader) traverseTree(ip netip.Addr, node uint, stopBit int) (uint, int) {
	i := 0
	if ip.Is4() {
		i = r.ipv4StartBitDepth
		node = r.ipv4Start
	}
	nodeCount := r.Metadata.NodeCount

	ip16 := ip.As16()

	for ; i < stopBit && node < nodeCount; i++ {
		bit := uint(1) & (uint(ip16[i>>3]) >> (7 - (i % 8)))

		offset := node * r.nodeOffsetMult
		if bit == 0 {
			node = r.nodeReader.readLeft(offset)
		} else {
			node = r.nodeReader.readRight(offset)
		}
	}

	return node, i
}

func (r *Reader) resolveDataPointer(pointer uint) (uintptr, error) {
	resolved := uintptr(pointer - r.Metadata.NodeCount - dataSectionSeparatorSize)

	if resolved >= uintptr(len(r.buffer)) {
		return 0, newInvalidDatabaseError("the MaxMind DB file's search tree is corrupt")
	}
	return resolved, nil
}
