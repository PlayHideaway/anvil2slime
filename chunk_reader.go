package main

type MinecraftChunkRoot struct {
	Level MinecraftChunk
}

type MinecraftChunk struct {
	X int `nbt:"xPos"`
	Z int `nbt:"zPos"`

	Sections []MinecraftChunkSection

	HeightMap interface{}

	TileEntities []interface{}
	Entities     []interface{}
}

type MinecraftChunkSection struct {
	Y uint8

	BlockLight []byte
	SkyLight   []byte

	BlockStates interface{}
	Biomes      interface{}
}
