package vm

type TraceStep struct {
	PC      uint32
	Instr   uint32
	Regs    [32]uint32
	MemAddr uint32 // address of memory access (if any)
	MemVal  uint32 // value read from or written to memory
	MemOp   int    // 0: none, 1: read, 2: write
}

type Trace []TraceStep
