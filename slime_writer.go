package main

import (
	"bytes"
	"encoding/binary"
	"io"
	"sort"

	"github.com/astei/anvil2slime/nbt"
	"github.com/klauspost/compress/zstd"
)

const slimeHeader = 0xB10B
const slimeLatestVersion = 1

func slimeChunkKey(coord ChunkCoord) int64 {
	return (int64(coord.Z) * 0x7fffffff) + int64(coord.X)
}

func (world *AnvilWorld) WriteAsSlime(writer io.Writer) error {
	zstdWriter, err := zstd.NewWriter(io.Discard)
	if err != nil {
		return err
	}
	slimeWriter := &slimeWriter{writer: writer, world: world, zstdWriter: zstdWriter}
	return slimeWriter.writeWorld()
}

type slimeWriter struct {
	writer     io.Writer
	world      *AnvilWorld
	zstdWriter *zstd.Encoder
}

func (w *slimeWriter) writeWorld() (err error) {
	if err = w.writeHeader(); err != nil {
		return
	}
	if err = w.writeChunks(); err != nil {
		return
	}
	if err = w.writeTileEntities(); err != nil {
		return
	}
	if err = w.writeEntities(); err != nil {
		return
	}
	if err = w.writeExtra(); err != nil {
		return
	}
	return
}

func (w *slimeWriter) writeHeader() (err error) {
	var header struct {
		Magic   uint16
		Version uint8
	}

	header.Magic = slimeHeader
	header.Version = slimeLatestVersion

	if err = binary.Write(w.writer, binary.BigEndian, header); err != nil {
		return
	}

	return
}

func (w *slimeWriter) writeChunks() (err error) {
	slimeSorted := w.world.getChunkKeys()
	sort.Slice(slimeSorted, func(one, two int) bool {
		k1 := slimeChunkKey(slimeSorted[one])
		k2 := slimeChunkKey(slimeSorted[two])
		return k1 < k2
	})

	var out bytes.Buffer

	if err = binary.Write(&out, binary.BigEndian, uint32(len(slimeSorted))); err != nil {
		return
	}

	for _, coord := range slimeSorted {
		chunk := w.world.chunks[coord]
		if err = w.writeChunkHeader(chunk, &out); err != nil {
			return
		}

		if err = binary.Write(&out, binary.BigEndian, uint32(len(chunk.Sections))); err != nil {
			return
		}

		for _, section := range chunk.Sections {
			if err = w.writeChunkSection(section, &out); err != nil {
				return
			}
		}
	}

	return w.writeZstdCompressed(&out)
}

func (w *slimeWriter) writeChunkHeader(chunk MinecraftChunk, out io.Writer) (err error) {
	if err = binary.Write(out, binary.BigEndian, uint32(chunk.X)); err != nil {
		return
	}

	if err = binary.Write(out, binary.BigEndian, uint32(chunk.Z)); err != nil {
		return
	}

	w.writeNbt(chunk.HeightMap, "Heightmaps", out)
	return
}

func (w *slimeWriter) writeChunkSection(section MinecraftChunkSection, out io.Writer) (err error) {
	if section.BlockLight != nil {
		if err = binary.Write(out, binary.BigEndian, bool(true)); err != nil {
			return
		}

		if _, err = out.Write(section.BlockLight); err != nil {
			return
		}
	} else {
		if err = binary.Write(out, binary.BigEndian, bool(false)); err != nil {
			return
		}
	}

	if section.SkyLight != nil {
		if err = binary.Write(out, binary.BigEndian, bool(true)); err != nil {
			return
		}

		if _, err = out.Write(section.SkyLight); err != nil {
			return
		}
	} else {
		if err = binary.Write(out, binary.BigEndian, bool(false)); err != nil {
			return
		}
	}

	if err = w.writeNbt(section.BlockStates, "block_states", out); err != nil {
		return
	}

	if err = w.writeNbt(section.Biomes, "biomes", out); err != nil {
		return
	}

	return
}

func (w *slimeWriter) writeZstdCompressed(buf *bytes.Buffer) (err error) {
	uncompressedSize := buf.Len()

	var compressedOutput bytes.Buffer
	w.zstdWriter.Reset(&compressedOutput)
	if _, err = buf.WriteTo(w.zstdWriter); err != nil {
		return
	}
	if err = w.zstdWriter.Close(); err != nil {
		return
	}
	w.zstdWriter.Reset(io.Discard)

	if err = binary.Write(w.writer, binary.BigEndian, uint32(compressedOutput.Len())); err != nil {
		return
	}
	if err = binary.Write(w.writer, binary.BigEndian, uint32(uncompressedSize)); err != nil {
		return
	}
	_, err = compressedOutput.WriteTo(w.writer)
	return
}

func (w *slimeWriter) writeTileEntities() (err error) {
	var tileEntities []interface{}
	for _, chunk := range w.world.chunks {
		tileEntities = append(tileEntities, chunk.TileEntities...)
	}

	var compound struct {
		Tiles []interface{} `nbt:"tiles"`
	}

	compound.Tiles = tileEntities
	return w.writeCompressedNbt(compound, "tiles")
}

func (w *slimeWriter) writeEntities() (err error) {
	var entities []interface{}
	for _, chunk := range w.world.chunks {
		entities = append(entities, chunk.Entities...)
	}

	var compound struct {
		Entities []interface{} `nbt:"entities"`
	}

	compound.Entities = entities
	return w.writeCompressedNbt(compound, "entities")
}

func (w *slimeWriter) writeCompressedNbt(compound interface{}, tagName string) (err error) {
	var buf bytes.Buffer
	if err = nbt.NewEncoder(&buf).Encode(compound, tagName); err != nil {
		return
	}
	return w.writeZstdCompressed(&buf)
}

func (w *slimeWriter) writeNbt(compound interface{}, tagName string, out io.Writer) (err error) {
	var buf bytes.Buffer
	if err = nbt.NewEncoder(&buf).Encode(compound, tagName); err != nil {
		return
	}
	if err = binary.Write(out, binary.BigEndian, uint32(buf.Len())); err != nil {
		return
	}
	_, err = buf.WriteTo(out)
	return
}

func (w *slimeWriter) writeExtra() (err error) {
	// Write empty NBT tag compound
	var empty map[string]interface{}
	return w.writeCompressedNbt(empty, "extra")
}

func (world *AnvilWorld) getChunkKeys() []ChunkCoord {
	var keys []ChunkCoord
	for coord := range world.chunks {
		keys = append(keys, coord)
	}
	return keys
}
