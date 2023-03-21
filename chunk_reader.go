package main

type MinecraftChunkRoot struct {
	Level MinecraftChunk
}

type MinecraftChunk struct {
	X int `nbt:"xPos"`
	Z int `nbt:"zPos"`

	Sections []MinecraftChunkSection `nbt:"sections"`

	HeightMap interface{} `nbt:"Heightmaps"`

	TileEntities []interface{} `nbt:"block_entities"`
	Entities     []interface{} `nbt:"Entities"`
}

type MinecraftChunkSection struct {
	Y uint8 `nbt:"Y"`

	BlockLight []byte `nbt:"BlockLight"`
	SkyLight   []byte `nbt:"SkyLight"`

	BlockStates interface{} `nbt:"block_states"`
	Biomes      interface{} `nbt:"biomes"`
}
