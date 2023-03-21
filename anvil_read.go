package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/zlib"
)

const anvilMaxOffsets = 1024
const anvilSectorSize = 4096

var ErrNoChunk = errors.New("anvil: chunk not found")
var ErrInvalidChunkLength = errors.New("anvil: invalid chunk length")
var ErrInvalidCompression = errors.New("anvil: invalid compression format")

type AnvilCompressionLevel byte

const (
	AnvilCompressionLevelGzip    AnvilCompressionLevel = 1
	AnvilCompressionLevelDeflate AnvilCompressionLevel = 2
)

// Struct AnvilReader allows you to read an Anvil region file and extract its components. The reader is not safe for
// concurrent access; usage should be protected by a mutex if concurrent access is desired.
type AnvilReader struct {
	source      io.ReadSeeker
	sectorTable []int32
	Name        string
}

// Creates an AnvilReader. The ownership of the source is transferred to this reader.
func NewAnvilReader(source io.ReadSeeker) (reader *AnvilReader, err error) {
	reader = &AnvilReader{
		source:      source,
		sectorTable: make([]int32, anvilMaxOffsets),
	}

	if file, ok := source.(*os.File); ok {
		reader.Name = file.Name()
	}
	err = reader.readSectorTable()
	return
}

func (world *AnvilReader) readSectorTable() (err error) {
	_, err = world.source.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}

	rawSectorData := make([]byte, anvilSectorSize)
	_, err = io.ReadFull(world.source, rawSectorData)
	if err != nil {
		return err
	}

	rawSectorIn := bytes.NewReader(rawSectorData)
	err = binary.Read(rawSectorIn, binary.BigEndian, world.sectorTable)
	return
}

// ReadChunk reads an Anvil chunk at the specified X and Z coordinates. Note that these coordinates are relative to the
// region file and are not chunk coordinates. If successful, the provided reader may be provided to an NBT deserialization
// routine.
func (world *AnvilReader) ReadChunk(x, z int) (chunk io.Reader, err error) {
	location := world.sectorTable[x+z*32]

	start := location >> 8
	if start == 0 {
		err = ErrNoChunk
		return
	}

	if _, err = world.source.Seek(int64(start*anvilSectorSize), io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek: %s", err.Error())
	}

	// Payload Header

	payloadHeader := make([]byte, 5)
	if _, err = io.ReadFull(world.source, payloadHeader); err != nil {
		return nil, fmt.Errorf("could not read payload header: %s", err.Error())
	}

	var payloadInfo struct {
		Length      int32
		Compression AnvilCompressionLevel
	}

	payloadHeaderReader := bytes.NewReader(payloadHeader)
	if err = binary.Read(payloadHeaderReader, binary.BigEndian, &payloadInfo); err != nil {
		return nil, fmt.Errorf("could not parse payload header: %s", err.Error())
	}

	// Payload

	payloadData := make([]byte, payloadInfo.Length-1)
	if _, err = io.ReadFull(world.source, payloadData); err != nil {
		return nil, fmt.Errorf("could not read payload data: %s", err.Error())
	}

	payloadReader := bytes.NewReader(payloadData)
	chunkStream := io.LimitReader(payloadReader, int64(payloadInfo.Length-1))
	switch payloadInfo.Compression {
	case AnvilCompressionLevelGzip:
		return gzip.NewReader(chunkStream)
	case AnvilCompressionLevelDeflate:
		return zlib.NewReader(chunkStream)
	default:
		return nil, ErrInvalidCompression
	}
}

func (world *AnvilReader) ChunkExists(x, z int) bool {
	return world.sectorTable[x+z*32] != 0
}

func (world *AnvilReader) Close() error {
	if closer, ok := world.source.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}
