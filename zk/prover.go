package zk

import (
	"github.com/VanshSahay/riscvm/vm"
	"github.com/consensys/gnark/frontend"
)

func GenerateWitness(current vm.TraceStep, nextPC uint32, nextRegs [32]uint32) StepCircuit {
	w := StepCircuit{
		PCBefore: frontend.Variable(current.PC),
		PCAfter:  frontend.Variable(nextPC),
		Instr:    frontend.Variable(current.Instr),
	}
	for i := 0; i < 32; i++ {
		w.RegsBefore[i] = frontend.Variable(current.Regs[i])
		w.RegsAfter[i] = frontend.Variable(nextRegs[i])
	}

	// Simple manual decoding for the witness fields (should match what the circuit does)
	instr := current.Instr
	opcode := instr & 0x7F
	w.Opcode = frontend.Variable(opcode)
	w.Rd = frontend.Variable((instr >> 7) & 0x1F)
	w.Funct3 = frontend.Variable((instr >> 12) & 0x7)
	w.Rs1 = frontend.Variable((instr >> 15) & 0x1F)
	w.Rs2 = frontend.Variable((instr >> 20) & 0x1F)
	w.Funct7 = frontend.Variable((instr >> 25) & 0x7F)

	w.Imm = frontend.Variable(0)

	// Decode immediates
	switch opcode {
	case 0x13, 0x03, 0x67: // I-type
		imm := int32(instr) >> 20
		w.Imm = frontend.Variable(uint32(imm))
	case 0x37, 0x17: // U-type
		imm := (instr >> 12) << 12
		w.Imm = frontend.Variable(imm)
	case 0x63: // B-type
		imm := DecodeBImm(instr)
		w.Imm = frontend.Variable(uint32(imm))
	case 0x6F: // J-type
		imm := DecodeJImm(instr)
		w.Imm = frontend.Variable(uint32(imm))
	}

	return w
}

func ProveStep(w StepCircuit) (bool, error) {
	// For a real zkVM we would pre-compile and cache the CCS.
	// For this WASM demo, we simulate the verification.
	// In production, gnark's Groth16.Prove/Verify would be used.
	return true, nil
}

func DecodeBImm(instr uint32) int32 {
	imm := ((instr >> 31) << 12) | (((instr >> 7) & 1) << 11) |
		(((instr >> 25) & 0x3F) << 5) | (((instr >> 8) & 0xF) << 1)
	return int32(imm<<19) >> 19
}

func DecodeJImm(instr uint32) int32 {
	imm := ((instr >> 31) << 20) | (((instr >> 12) & 0xFF) << 12) |
		(((instr >> 20) & 1) << 11) | (((instr >> 21) & 0x3FF) << 1)
	return int32(imm<<11) >> 11
}
