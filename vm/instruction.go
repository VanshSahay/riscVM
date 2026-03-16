package vm

type Instruction interface {
	Execute(cpu *CPU)
	ModifiesPC() bool // true for branches, jal, jalr - they set PC; Step won't add 4
}

type Unknown struct{}

func (Unknown) Execute(cpu *CPU)   {}
func (Unknown) ModifiesPC() bool   { return false }

// R-type arithmetic
type Add struct {
	Rd  uint8
	Rs1 uint8
	Rs2 uint8
}

func (i Add) Execute(cpu *CPU)  { cpu.Regs[i.Rd] = cpu.Regs[i.Rs1] + cpu.Regs[i.Rs2] }
func (i Add) ModifiesPC() bool  { return false }

type Sub struct {
	Rd  uint8
	Rs1 uint8
	Rs2 uint8
}

func (i Sub) Execute(cpu *CPU)  { cpu.Regs[i.Rd] = cpu.Regs[i.Rs1] - cpu.Regs[i.Rs2] }
func (i Sub) ModifiesPC() bool  { return false }

type Sll struct {
	Rd  uint8
	Rs1 uint8
	Rs2 uint8
}

func (i Sll) Execute(cpu *CPU) {
	shamt := cpu.Regs[i.Rs2] & 0x1F
	cpu.Regs[i.Rd] = cpu.Regs[i.Rs1] << shamt
}
func (i Sll) ModifiesPC() bool { return false }

type Slt struct {
	Rd  uint8
	Rs1 uint8
	Rs2 uint8
}

func (i Slt) Execute(cpu *CPU) {
	if int32(cpu.Regs[i.Rs1]) < int32(cpu.Regs[i.Rs2]) {
		cpu.Regs[i.Rd] = 1
	} else {
		cpu.Regs[i.Rd] = 0
	}
}
func (i Slt) ModifiesPC() bool { return false }

type Sltu struct {
	Rd  uint8
	Rs1 uint8
	Rs2 uint8
}

func (i Sltu) Execute(cpu *CPU) {
	if cpu.Regs[i.Rs1] < cpu.Regs[i.Rs2] {
		cpu.Regs[i.Rd] = 1
	} else {
		cpu.Regs[i.Rd] = 0
	}
}
func (i Sltu) ModifiesPC() bool { return false }

type Xor struct {
	Rd  uint8
	Rs1 uint8
	Rs2 uint8
}

func (i Xor) Execute(cpu *CPU) {
	cpu.Regs[i.Rd] = cpu.Regs[i.Rs1] ^ cpu.Regs[i.Rs2]
}
func (i Xor) ModifiesPC() bool { return false }

type Srl struct {
	Rd  uint8
	Rs1 uint8
	Rs2 uint8
}

func (i Srl) Execute(cpu *CPU) {
	shamt := cpu.Regs[i.Rs2] & 0x1F
	cpu.Regs[i.Rd] = cpu.Regs[i.Rs1] >> shamt
}
func (i Srl) ModifiesPC() bool { return false }

type Sra struct {
	Rd  uint8
	Rs1 uint8
	Rs2 uint8
}

func (i Sra) Execute(cpu *CPU) {
	shamt := cpu.Regs[i.Rs2] & 0x1F
	cpu.Regs[i.Rd] = uint32(int32(cpu.Regs[i.Rs1]) >> shamt)
}
func (i Sra) ModifiesPC() bool { return false }

type Or struct {
	Rd  uint8
	Rs1 uint8
	Rs2 uint8
}

func (i Or) Execute(cpu *CPU) {
	cpu.Regs[i.Rd] = cpu.Regs[i.Rs1] | cpu.Regs[i.Rs2]
}
func (i Or) ModifiesPC() bool { return false }

type And struct {
	Rd  uint8
	Rs1 uint8
	Rs2 uint8
}

func (i And) Execute(cpu *CPU) {
	cpu.Regs[i.Rd] = cpu.Regs[i.Rs1] & cpu.Regs[i.Rs2]
}
func (i And) ModifiesPC() bool { return false }

// I-type arithmetic (op 0x13)
type Addi struct {
	Rd  uint8
	Rs1 uint8
	Imm int32
}

func (i Addi) Execute(cpu *CPU) {
	cpu.Regs[i.Rd] = uint32(int32(cpu.Regs[i.Rs1]) + i.Imm)
}
func (Addi) ModifiesPC() bool { return false }

type Slti struct {
	Rd  uint8
	Rs1 uint8
	Imm int32
}

func (i Slti) Execute(cpu *CPU) {
	if int32(cpu.Regs[i.Rs1]) < i.Imm {
		cpu.Regs[i.Rd] = 1
	} else {
		cpu.Regs[i.Rd] = 0
	}
}
func (Slti) ModifiesPC() bool { return false }

type Sltiu struct {
	Rd  uint8
	Rs1 uint8
	Imm uint32
}

func (i Sltiu) Execute(cpu *CPU) {
	if cpu.Regs[i.Rs1] < i.Imm {
		cpu.Regs[i.Rd] = 1
	} else {
		cpu.Regs[i.Rd] = 0
	}
}
func (Sltiu) ModifiesPC() bool { return false }

type Xori struct {
	Rd  uint8
	Rs1 uint8
	Imm int32
}

func (i Xori) Execute(cpu *CPU) {
	cpu.Regs[i.Rd] = cpu.Regs[i.Rs1] ^ uint32(i.Imm)
}
func (Xori) ModifiesPC() bool { return false }

type Ori struct {
	Rd  uint8
	Rs1 uint8
	Imm int32
}

func (i Ori) Execute(cpu *CPU) {
	cpu.Regs[i.Rd] = cpu.Regs[i.Rs1] | uint32(i.Imm)
}
func (Ori) ModifiesPC() bool { return false }

type Andi struct {
	Rd  uint8
	Rs1 uint8
	Imm int32
}

func (i Andi) Execute(cpu *CPU) {
	cpu.Regs[i.Rd] = cpu.Regs[i.Rs1] & uint32(i.Imm)
}
func (Andi) ModifiesPC() bool { return false }

type Slli struct {
	Rd    uint8
	Rs1   uint8
	Shamt uint8
}

func (i Slli) Execute(cpu *CPU) {
	cpu.Regs[i.Rd] = cpu.Regs[i.Rs1] << (i.Shamt & 0x1F)
}
func (Slli) ModifiesPC() bool { return false }

type Srli struct {
	Rd    uint8
	Rs1   uint8
	Shamt uint8
}

func (i Srli) Execute(cpu *CPU) {
	cpu.Regs[i.Rd] = cpu.Regs[i.Rs1] >> (i.Shamt & 0x1F)
}
func (Srli) ModifiesPC() bool { return false }

type Srai struct {
	Rd    uint8
	Rs1   uint8
	Shamt uint8
}

func (i Srai) Execute(cpu *CPU) {
	cpu.Regs[i.Rd] = uint32(int32(cpu.Regs[i.Rs1]) >> (i.Shamt & 0x1F))
}
func (Srai) ModifiesPC() bool { return false }

// U-type
type Lui struct {
	Rd  uint8
	Imm uint32
}

func (i Lui) Execute(cpu *CPU) {
	cpu.Regs[i.Rd] = i.Imm
}
func (Lui) ModifiesPC() bool { return false }

type Auipc struct {
	Rd  uint8
	Imm uint32
}

func (i Auipc) Execute(cpu *CPU) {
	cpu.Regs[i.Rd] = cpu.PC + i.Imm
}
func (Auipc) ModifiesPC() bool { return false }

// Loads (op 0x03)
type Lb struct {
	Rd  uint8
	Rs1 uint8
	Imm int32
}

func (i Lb) Execute(cpu *CPU) {
	addr := uint32(int32(cpu.Regs[i.Rs1]) + i.Imm)
	v := cpu.Mem.LoadByte(addr)
	cpu.Regs[i.Rd] = uint32(int32(int8(v)))
}
func (Lb) ModifiesPC() bool { return false }

type Lh struct {
	Rd  uint8
	Rs1 uint8
	Imm int32
}

func (i Lh) Execute(cpu *CPU) {
	addr := uint32(int32(cpu.Regs[i.Rs1]) + i.Imm)
	v := cpu.Mem.LoadHalf(addr)
	cpu.Regs[i.Rd] = uint32(int32(int16(v)))
}
func (Lh) ModifiesPC() bool { return false }

type Lw struct {
	Rd  uint8
	Rs1 uint8
	Imm int32
}

func (i Lw) Execute(cpu *CPU) {
	addr := uint32(int32(cpu.Regs[i.Rs1]) + i.Imm)
	cpu.Regs[i.Rd] = cpu.Mem.LoadWord(addr)
}
func (Lw) ModifiesPC() bool { return false }

type Lbu struct {
	Rd  uint8
	Rs1 uint8
	Imm int32
}

func (i Lbu) Execute(cpu *CPU) {
	addr := uint32(int32(cpu.Regs[i.Rs1]) + i.Imm)
	cpu.Regs[i.Rd] = cpu.Mem.LoadByte(addr)
}
func (Lbu) ModifiesPC() bool { return false }

type Lhu struct {
	Rd  uint8
	Rs1 uint8
	Imm int32
}

func (i Lhu) Execute(cpu *CPU) {
	addr := uint32(int32(cpu.Regs[i.Rs1]) + i.Imm)
	cpu.Regs[i.Rd] = cpu.Mem.LoadHalf(addr)
}
func (Lhu) ModifiesPC() bool { return false }

// Stores (op 0x23)
type Sb struct {
	Rs1 uint8
	Rs2 uint8
	Imm int32
}

func (i Sb) Execute(cpu *CPU) {
	addr := uint32(int32(cpu.Regs[i.Rs1]) + i.Imm)
	cpu.Mem.StoreByte(addr, cpu.Regs[i.Rs2])
}
func (Sb) ModifiesPC() bool { return false }

type Sh struct {
	Rs1 uint8
	Rs2 uint8
	Imm int32
}

func (i Sh) Execute(cpu *CPU) {
	addr := uint32(int32(cpu.Regs[i.Rs1]) + i.Imm)
	cpu.Mem.StoreHalf(addr, cpu.Regs[i.Rs2])
}
func (Sh) ModifiesPC() bool { return false }

type Sw struct {
	Rs1 uint8
	Rs2 uint8
	Imm int32
}

func (i Sw) Execute(cpu *CPU) {
	addr := uint32(int32(cpu.Regs[i.Rs1]) + i.Imm)
	cpu.Mem.StoreWord(addr, cpu.Regs[i.Rs2])
}
func (Sw) ModifiesPC() bool { return false }

// Branches (op 0x63) - B-type imm is byte offset from current PC
type Beq struct {
	Rs1 uint8
	Rs2 uint8
	Imm int32
}

func (i Beq) Execute(cpu *CPU) {
	if cpu.Regs[i.Rs1] == cpu.Regs[i.Rs2] {
		cpu.PC += uint32(i.Imm)
	} else {
		cpu.PC += 4
	}
}
func (Beq) ModifiesPC() bool { return true }

type Bne struct {
	Rs1 uint8
	Rs2 uint8
	Imm int32
}

func (i Bne) Execute(cpu *CPU) {
	if cpu.Regs[i.Rs1] != cpu.Regs[i.Rs2] {
		cpu.PC += uint32(i.Imm)
	} else {
		cpu.PC += 4
	}
}
func (Bne) ModifiesPC() bool { return true }

type Blt struct {
	Rs1 uint8
	Rs2 uint8
	Imm int32
}

func (i Blt) Execute(cpu *CPU) {
	if int32(cpu.Regs[i.Rs1]) < int32(cpu.Regs[i.Rs2]) {
		cpu.PC += uint32(i.Imm)
	} else {
		cpu.PC += 4
	}
}
func (Blt) ModifiesPC() bool { return true }

type Bge struct {
	Rs1 uint8
	Rs2 uint8
	Imm int32
}

func (i Bge) Execute(cpu *CPU) {
	if int32(cpu.Regs[i.Rs1]) >= int32(cpu.Regs[i.Rs2]) {
		cpu.PC += uint32(i.Imm)
	} else {
		cpu.PC += 4
	}
}
func (Bge) ModifiesPC() bool { return true }

type Bltu struct {
	Rs1 uint8
	Rs2 uint8
	Imm int32
}

func (i Bltu) Execute(cpu *CPU) {
	if cpu.Regs[i.Rs1] < cpu.Regs[i.Rs2] {
		cpu.PC += uint32(i.Imm)
	} else {
		cpu.PC += 4
	}
}
func (Bltu) ModifiesPC() bool { return true }

type Bgeu struct {
	Rs1 uint8
	Rs2 uint8
	Imm int32
}

func (i Bgeu) Execute(cpu *CPU) {
	if cpu.Regs[i.Rs1] >= cpu.Regs[i.Rs2] {
		cpu.PC += uint32(i.Imm)
	} else {
		cpu.PC += 4
	}
}
func (Bgeu) ModifiesPC() bool { return true }

// JAL (op 0x6F) - J-type imm is byte offset from current PC
type Jal struct {
	Rd  uint8
	Imm int32
}

func (i Jal) Execute(cpu *CPU) {
	cpu.Regs[i.Rd] = cpu.PC + 4
	cpu.PC += uint32(i.Imm)
}
func (Jal) ModifiesPC() bool { return true }

// JALR (op 0x67)
type Jalr struct {
	Rd  uint8
	Rs1 uint8
	Imm int32
}

func (i Jalr) Execute(cpu *CPU) {
	target := uint32(int32(cpu.Regs[i.Rs1]) + i.Imm)
	target &^= 1 // clear LSB per spec
	cpu.Regs[i.Rd] = cpu.PC + 4
	cpu.PC = target
}
func (Jalr) ModifiesPC() bool { return true }

// System (op 0x73)
type Ecall struct{}
type Ebreak struct{}

func (Ecall) Execute(cpu *CPU) {
	// Handled by VM Run loop
}

func (Ebreak) Execute(cpu *CPU) {
	// Handled by VM Run loop - typically halt/debug
}
func (Ecall) ModifiesPC() bool  { return true }  // handled specially, doesn't advance
func (Ebreak) ModifiesPC() bool { return true }

// Fence - no-op for single-hart
type Fence struct{}
type FenceI struct{}

func (Fence) Execute(cpu *CPU)   {}
func (FenceI) Execute(cpu *CPU) {}
func (Fence) ModifiesPC() bool  { return false }
func (FenceI) ModifiesPC() bool { return false }
