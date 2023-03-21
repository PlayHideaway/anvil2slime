package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/astei/anvil2slime/nbt"
)

type ChunkCoord struct {
	X int
	Z int
}

type AnvilWorld struct {
	chunks map[ChunkCoord]MinecraftChunk
}

func OpenAnvilWorld(root string) (world *AnvilWorld, err error) {
	rootDirectory, err := os.Open(root)
	if err != nil {
		return
	}

	files, err := rootDirectory.Readdir(0)
	if err != nil {
		return
	}

	var regionReaders []*AnvilReader
	for _, possibleRegionFile := range files {
		fmt.Println("discovered ", possibleRegionFile.Name())
		if strings.HasSuffix(possibleRegionFile.Name(), ".mca") {
			file, err := os.Open(filepath.Join(root, possibleRegionFile.Name()))
			if err != nil {
				return nil, err
			}
			reader, err := NewAnvilReader(file)
			if err != nil {
				return nil, err
			}
			regionReaders = append(regionReaders, reader)
		}
	}

	var wg sync.WaitGroup
	wg.Add(len(regionReaders))
	resultChan := make(chan *map[ChunkCoord]MinecraftChunk, len(regionReaders))
	for _, reader := range regionReaders {
		go func(reader *AnvilReader, res chan *map[ChunkCoord]MinecraftChunk, wg *sync.WaitGroup) {
			defer wg.Done()
			result, err := tryToReadRegion(reader)
			if err != nil {
				fmt.Println("Unable to read chunks: " + err.Error())
				return
			}
			res <- result
		}(reader, resultChan, &wg)
	}

	wg.Wait()
	close(resultChan)

	allChunks := make(map[ChunkCoord]MinecraftChunk)
	for m := range resultChan {
		for k, v := range *m {
			allChunks[k] = v
		}
	}
	fmt.Printf("Discovered %d chunks in the world\n", len(allChunks))
	return &AnvilWorld{chunks: allChunks}, nil
}

func tryToReadRegion(reader *AnvilReader) (*map[ChunkCoord]MinecraftChunk, error) {
	byXZ := make(map[ChunkCoord]MinecraftChunk)
	for x := 0; x < 32; x++ {
		for z := 0; z < 32; z++ {
			if reader.ChunkExists(x, z) {
				chunkReader, err := reader.ReadChunk(x, z)
				if err != nil {
					return nil, fmt.Errorf("could not read chunk %d,%d in %s: %s", x, z, reader.Name, err.Error())
				}

				var anvilChunkRoot MinecraftChunk
				if _, err = nbt.NewDecoder(chunkReader).Decode(&anvilChunkRoot); err != nil {
					return nil, fmt.Errorf("could not deserialize chunk %d,%d in %s: %s", x, z, reader.Name, err.Error())
				}

				var cleanedSections []MinecraftChunkSection
				for _, section := range anvilChunkRoot.Sections {
					if section.BlockStates != nil {
						cleanedSections = append(cleanedSections, section)

						if section.BlockLight != nil && len(section.BlockLight) != 2048 {
							return nil, fmt.Errorf("could not deserialize chunk %d,%d in %s: invalid block light size", x, z, reader.Name)
						}

						if section.SkyLight != nil && len(section.SkyLight) != 2048 {
							return nil, fmt.Errorf("could not deserialize chunk %d,%d in %s: invalid sky light size", x, z, reader.Name)
						}
					}
				}

				anvilChunkRoot.Sections = cleanedSections
				if len(anvilChunkRoot.Sections) == 0 {
					continue
				}

				coords := ChunkCoord{X: anvilChunkRoot.X, Z: anvilChunkRoot.Z}
				byXZ[coords] = anvilChunkRoot
			}
		}
	}
	return &byXZ, nil
}
