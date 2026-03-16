package vm

const DefaultMemSize = 16 * 1024 * 1024

type Memory struct {
	Data     []byte
	OnAccess func(addr uint32, value uint32, op int) // op: 1=read, 2=write
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
	v := uint32(m.Data[addr])
	if m.OnAccess != nil {
		m.OnAccess(addr, v, 1)
	}
	return v
}

func (m *Memory) LoadHalf(addr uint32) uint32 {
	if !m.inBounds(addr, 2) {
		return 0
	}
	v := uint32(m.Data[addr]) | uint32(m.Data[addr+1])<<8
	if m.OnAccess != nil {
		m.OnAccess(addr, v, 1)
	}
	return v
}

func (m *Memory) LoadWord(addr uint32) uint32 {
	if !m.inBounds(addr, 4) {
		return 0
	}
	v := uint32(m.Data[addr]) |
		uint32(m.Data[addr+1])<<8 |
		uint32(m.Data[addr+2])<<16 |
		uint32(m.Data[addr+3])<<24
	if m.OnAccess != nil {
		m.OnAccess(addr, v, 1)
	}
	return v
}

func (m *Memory) StoreByte(addr uint32, value uint32) {
	if m.inBounds(addr, 1) {
		m.Data[addr] = byte(value)
		if m.OnAccess != nil {
			m.OnAccess(addr, value&0xFF, 2)
		}
	}
}

func (m *Memory) StoreHalf(addr uint32, value uint32) {
	if m.inBounds(addr, 2) {
		m.Data[addr] = byte(value)
		m.Data[addr+1] = byte(value >> 8)
		if m.OnAccess != nil {
			m.OnAccess(addr, value&0xFFFF, 2)
		}
	}
}

func (m *Memory) StoreWord(addr uint32, value uint32) {
	if m.inBounds(addr, 4) {
		m.Data[addr] = byte(value)
		m.Data[addr+1] = byte(value >> 8)
		m.Data[addr+2] = byte(value >> 16)
		m.Data[addr+3] = byte(value >> 24)
		if m.OnAccess != nil {
			m.OnAccess(addr, value, 2)
		}
	}
}
