package vm

// decodeBImm decodes B-type immediate: bits [12|11|10:5|4:1|0=0]
func decodeBImm(instr uint32) int32 {
	imm := ((instr >> 31) << 12) | (((instr >> 7) & 1) << 11) |
		(((instr >> 25) & 0x3F) << 5) | (((instr >> 8) & 0xF) << 1)
	return int32(imm<<19) >> 19
}

// decodeJImm decodes J-type immediate: bits [20|10:1|11|19:12|0=0]
func decodeJImm(instr uint32) int32 {
	imm := ((instr >> 31) << 20) | (((instr >> 12) & 0xFF) << 12) |
		(((instr >> 20) & 1) << 11) | (((instr >> 21) & 0x3FF) << 1)
	return int32(imm<<11) >> 11
}

func Decode(instr uint32) Instruction {
	opcode := instr & 0x7F
	rd := uint8((instr >> 7) & 0x1F)
	funct3 := (instr >> 12) & 0x7
	rs1 := uint8((instr >> 15) & 0x1F)
	rs2 := uint8((instr >> 20) & 0x1F)
	funct7 := (instr >> 25) & 0x7F

	// I-type imm: sign-extended 12 bits
	iImm := int32(instr) >> 20
	// U-type imm: upper 20 bits
	uImm := (instr >> 12) << 12

	switch opcode {
	case 0x33: // R-type
		switch funct3 {
		case 0:
			if funct7 == 0 {
				return Add{Rd: rd, Rs1: rs1, Rs2: rs2}
			}
			return Sub{Rd: rd, Rs1: rs1, Rs2: rs2}
		case 1:
			return Sll{Rd: rd, Rs1: rs1, Rs2: rs2}
		case 2:
			return Slt{Rd: rd, Rs1: rs1, Rs2: rs2}
		case 3:
			return Sltu{Rd: rd, Rs1: rs1, Rs2: rs2}
		case 4:
			return Xor{Rd: rd, Rs1: rs1, Rs2: rs2}
		case 5:
			if funct7 == 0 {
				return Srl{Rd: rd, Rs1: rs1, Rs2: rs2}
			}
			return Sra{Rd: rd, Rs1: rs1, Rs2: rs2}
		case 6:
			return Or{Rd: rd, Rs1: rs1, Rs2: rs2}
		case 7:
			return And{Rd: rd, Rs1: rs1, Rs2: rs2}
		}
	case 0x13: // I-type arithmetic
		shamt := uint8((instr >> 20) & 0x1F)
		switch funct3 {
		case 0:
			return Addi{Rd: rd, Rs1: rs1, Imm: iImm}
		case 2:
			return Slti{Rd: rd, Rs1: rs1, Imm: iImm}
		case 3:
			return Sltiu{Rd: rd, Rs1: rs1, Imm: uint32(iImm)}
		case 4:
			return Xori{Rd: rd, Rs1: rs1, Imm: iImm}
		case 6:
			return Ori{Rd: rd, Rs1: rs1, Imm: iImm}
		case 7:
			return Andi{Rd: rd, Rs1: rs1, Imm: iImm}
		case 1:
			return Slli{Rd: rd, Rs1: rs1, Shamt: shamt}
		case 5:
			if funct7 == 0 {
				return Srli{Rd: rd, Rs1: rs1, Shamt: shamt}
			}
			return Srai{Rd: rd, Rs1: rs1, Shamt: shamt}
		}
	case 0x17:
		return Auipc{Rd: rd, Imm: uImm}
	case 0x37:
		return Lui{Rd: rd, Imm: uImm}
	case 0x03: // Loads
		switch funct3 {
		case 0:
			return Lb{Rd: rd, Rs1: rs1, Imm: iImm}
		case 1:
			return Lh{Rd: rd, Rs1: rs1, Imm: iImm}
		case 2:
			return Lw{Rd: rd, Rs1: rs1, Imm: iImm}
		case 4:
			return Lbu{Rd: rd, Rs1: rs1, Imm: iImm}
		case 5:
			return Lhu{Rd: rd, Rs1: rs1, Imm: iImm}
		}
	case 0x23: // Stores
		sImm := int32(((instr >> 25) << 5) | ((instr >> 7) & 0x1F))
		sImm = sImm << 20 >> 20
		switch funct3 {
		case 0:
			return Sb{Rs1: rs1, Rs2: rs2, Imm: sImm}
		case 1:
			return Sh{Rs1: rs1, Rs2: rs2, Imm: sImm}
		case 2:
			return Sw{Rs1: rs1, Rs2: rs2, Imm: sImm}
		}
	case 0x63: // Branches
		bImm := decodeBImm(instr)
		switch funct3 {
		case 0:
			return Beq{Rs1: rs1, Rs2: rs2, Imm: bImm}
		case 1:
			return Bne{Rs1: rs1, Rs2: rs2, Imm: bImm}
		case 4:
			return Blt{Rs1: rs1, Rs2: rs2, Imm: bImm}
		case 5:
			return Bge{Rs1: rs1, Rs2: rs2, Imm: bImm}
		case 6:
			return Bltu{Rs1: rs1, Rs2: rs2, Imm: bImm}
		case 7:
			return Bgeu{Rs1: rs1, Rs2: rs2, Imm: bImm}
		}
	case 0x6F:
		return Jal{Rd: rd, Imm: decodeJImm(instr)}
	case 0x67:
		return Jalr{Rd: rd, Rs1: rs1, Imm: iImm}
	case 0x73:
		if (instr>>20)&0xFFF == 0 {
			return Ecall{}
		}
		if (instr>>20)&0xFFF == 1 {
			return Ebreak{}
		}
	case 0x0F:
		if funct3 == 0 {
			return Fence{}
		}
		return FenceI{}
	}
	return Unknown{}
}
