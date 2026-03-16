package zk

import (
	"fmt"
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
	w.Opcode = frontend.Variable(instr & 0x7F)
	w.Rd = frontend.Variable((instr >> 7) & 0x1F)
	w.Funct3 = frontend.Variable((instr >> 12) & 0x7)
	w.Rs1 = frontend.Variable((instr >> 15) & 0x1F)
	w.Rs2 = frontend.Variable((instr >> 20) & 0x1F)
	w.Funct7 = frontend.Variable((instr >> 25) & 0x7F)

	fmt.Printf("DEBUG: Instr=%08x Opcode=%x F3=%x F7=%x\n", instr, w.Opcode, w.Funct3, w.Funct7)

	w.Imm = frontend.Variable(0)

	// Detect instruction type
	if (instr & 0x7F) == 0x13 {
		// I-type immediate
		imm := int32(instr) >> 20
		w.Imm = frontend.Variable(uint32(imm))
	}

	return w
}
