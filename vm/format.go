package vm

import "fmt"

// FormatInstruction returns a short disassembly of the instruction (e.g. "add x1, x2, x3").
func FormatInstruction(instr uint32) string {
	d := Decode(instr)
	switch v := d.(type) {
	case Add:
		return fmt.Sprintf("add x%d, x%d, x%d", v.Rd, v.Rs1, v.Rs2)
	case Sub:
		return fmt.Sprintf("sub x%d, x%d, x%d", v.Rd, v.Rs1, v.Rs2)
	case Sll:
		return fmt.Sprintf("sll x%d, x%d, x%d", v.Rd, v.Rs1, v.Rs2)
	case Slt:
		return fmt.Sprintf("slt x%d, x%d, x%d", v.Rd, v.Rs1, v.Rs2)
	case Sltu:
		return fmt.Sprintf("sltu x%d, x%d, x%d", v.Rd, v.Rs1, v.Rs2)
	case Xor:
		return fmt.Sprintf("xor x%d, x%d, x%d", v.Rd, v.Rs1, v.Rs2)
	case Srl:
		return fmt.Sprintf("srl x%d, x%d, x%d", v.Rd, v.Rs1, v.Rs2)
	case Sra:
		return fmt.Sprintf("sra x%d, x%d, x%d", v.Rd, v.Rs1, v.Rs2)
	case Or:
		return fmt.Sprintf("or x%d, x%d, x%d", v.Rd, v.Rs1, v.Rs2)
	case And:
		return fmt.Sprintf("and x%d, x%d, x%d", v.Rd, v.Rs1, v.Rs2)
	case Addi:
		return fmt.Sprintf("addi x%d, x%d, %d", v.Rd, v.Rs1, v.Imm)
	case Slti:
		return fmt.Sprintf("slti x%d, x%d, %d", v.Rd, v.Rs1, v.Imm)
	case Sltiu:
		return fmt.Sprintf("sltiu x%d, x%d, %d", v.Rd, v.Rs1, v.Imm)
	case Xori:
		return fmt.Sprintf("xori x%d, x%d, %d", v.Rd, v.Rs1, v.Imm)
	case Ori:
		return fmt.Sprintf("ori x%d, x%d, %d", v.Rd, v.Rs1, v.Imm)
	case Andi:
		return fmt.Sprintf("andi x%d, x%d, %d", v.Rd, v.Rs1, v.Imm)
	case Slli:
		return fmt.Sprintf("slli x%d, x%d, %d", v.Rd, v.Rs1, v.Shamt)
	case Srli:
		return fmt.Sprintf("srli x%d, x%d, %d", v.Rd, v.Rs1, v.Shamt)
	case Srai:
		return fmt.Sprintf("srai x%d, x%d, %d", v.Rd, v.Rs1, v.Shamt)
	case Lui:
		return fmt.Sprintf("lui x%d, 0x%x", v.Rd, v.Imm>>12)
	case Auipc:
		return fmt.Sprintf("auipc x%d, 0x%x", v.Rd, v.Imm>>12)
	case Lb:
		return fmt.Sprintf("lb x%d, %d(x%d)", v.Rd, v.Imm, v.Rs1)
	case Lh:
		return fmt.Sprintf("lh x%d, %d(x%d)", v.Rd, v.Imm, v.Rs1)
	case Lw:
		return fmt.Sprintf("lw x%d, %d(x%d)", v.Rd, v.Imm, v.Rs1)
	case Lbu:
		return fmt.Sprintf("lbu x%d, %d(x%d)", v.Rd, v.Imm, v.Rs1)
	case Lhu:
		return fmt.Sprintf("lhu x%d, %d(x%d)", v.Rd, v.Imm, v.Rs1)
	case Sb:
		return fmt.Sprintf("sb x%d, %d(x%d)", v.Rs2, v.Imm, v.Rs1)
	case Sh:
		return fmt.Sprintf("sh x%d, %d(x%d)", v.Rs2, v.Imm, v.Rs1)
	case Sw:
		return fmt.Sprintf("sw x%d, %d(x%d)", v.Rs2, v.Imm, v.Rs1)
	case Beq:
		return fmt.Sprintf("beq x%d, x%d, %d", v.Rs1, v.Rs2, v.Imm)
	case Bne:
		return fmt.Sprintf("bne x%d, x%d, %d", v.Rs1, v.Rs2, v.Imm)
	case Blt:
		return fmt.Sprintf("blt x%d, x%d, %d", v.Rs1, v.Rs2, v.Imm)
	case Bge:
		return fmt.Sprintf("bge x%d, x%d, %d", v.Rs1, v.Rs2, v.Imm)
	case Bltu:
		return fmt.Sprintf("bltu x%d, x%d, %d", v.Rs1, v.Rs2, v.Imm)
	case Bgeu:
		return fmt.Sprintf("bgeu x%d, x%d, %d", v.Rs1, v.Rs2, v.Imm)
	case Jal:
		return fmt.Sprintf("jal x%d, %d", v.Rd, v.Imm)
	case Jalr:
		return fmt.Sprintf("jalr x%d, %d(x%d)", v.Rd, v.Imm, v.Rs1)
	case Ecall:
		return "ecall"
	case Ebreak:
		return "ebreak"
	case Fence:
		return "fence"
	case FenceI:
		return "fence.i"
	default:
		return fmt.Sprintf(".word 0x%08x", instr)
	}
}
