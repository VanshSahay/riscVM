package vm

// DefaultMemSize is 16MB - typical for small RISC-V programs
const DefaultMemSize = 16 * 1024 * 1024

type Memory struct {
	Data []byte
}

func NewMemory(size uint32) *Memory {
	if size == 0 {
		size = DefaultMemSize
	}
	return &Memory{
		Data: make([]byte, size),
	}
}

func (m *Memory) inBounds(addr uint32, size uint32) bool {
	return addr+size <= uint32(len(m.Data))
}

func (m *Memory) LoadByte(addr uint32) uint32 {
	if !m.inBounds(addr, 1) {
		return 0
	}
	return uint32(m.Data[addr])
}

func (m *Memory) LoadHalf(addr uint32) uint32 {
	if !m.inBounds(addr, 2) {
		return 0
	}
	return uint32(m.Data[addr]) | uint32(m.Data[addr+1])<<8
}

func (m *Memory) LoadWord(addr uint32) uint32 {
	if !m.inBounds(addr, 4) {
		return 0
	}
	return uint32(m.Data[addr]) |
		uint32(m.Data[addr+1])<<8 |
		uint32(m.Data[addr+2])<<16 |
		uint32(m.Data[addr+3])<<24
}

func (m *Memory) StoreByte(addr uint32, value uint32) {
	if m.inBounds(addr, 1) {
		m.Data[addr] = byte(value)
	}
}

func (m *Memory) StoreHalf(addr uint32, value uint32) {
	if m.inBounds(addr, 2) {
		m.Data[addr] = byte(value)
		m.Data[addr+1] = byte(value >> 8)
	}
}

func (m *Memory) StoreWord(addr uint32, value uint32) {
	if m.inBounds(addr, 4) {
		m.Data[addr] = byte(value)
		m.Data[addr+1] = byte(value >> 8)
		m.Data[addr+2] = byte(value >> 16)
		m.Data[addr+3] = byte(value >> 24)
	}
}
