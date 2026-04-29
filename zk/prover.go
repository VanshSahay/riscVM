package zk

import (
	"sync"

	"github.com/VanshSahay/riscvm/vm"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
)

var (
	setupOnce  sync.Once
	cachedR1CS constraint.ConstraintSystem
	cachedPK   groth16.ProvingKey
	cachedVK   groth16.VerifyingKey
	setupErr   error
)

func ensureSetup() error {
	setupOnce.Do(func() {
		cachedR1CS, setupErr = frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &StepCircuit{})
		if setupErr != nil {
			return
		}
		cachedPK, cachedVK, setupErr = groth16.Setup(cachedR1CS)
	})
	return setupErr
}

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

	instr := current.Instr
	opcode := instr & 0x7F
	w.Opcode = frontend.Variable(opcode)
	w.Rd = frontend.Variable((instr >> 7) & 0x1F)
	w.Funct3 = frontend.Variable((instr >> 12) & 0x7)
	w.Rs1 = frontend.Variable((instr >> 15) & 0x1F)
	w.Rs2 = frontend.Variable((instr >> 20) & 0x1F)
	w.Funct7 = frontend.Variable((instr >> 25) & 0x7F)

	w.Imm = frontend.Variable(0)

	switch opcode {
	case 0x13, 0x03, 0x67: // I-type
		imm := int32(instr) >> 20
		w.Imm = frontend.Variable(uint32(imm))
	case 0x37, 0x17: // U-type
		imm := (instr >> 12) << 12
		w.Imm = frontend.Variable(imm)
	case 0x23: // S-type
		imm := vm.DecodeSImm(instr)
		w.Imm = frontend.Variable(uint32(imm))
	case 0x63: // B-type
		imm := vm.DecodeBImm(instr)
		w.Imm = frontend.Variable(uint32(imm))
	case 0x6F: // J-type
		imm := vm.DecodeJImm(instr)
		w.Imm = frontend.Variable(uint32(imm))
	}

	return w
}

func ProveStep(w StepCircuit) (bool, error) {
	if err := ensureSetup(); err != nil {
		return false, err
	}

	fullWitness, err := frontend.NewWitness(&w, ecc.BN254.ScalarField())
	if err != nil {
		return false, err
	}

	proof, err := groth16.Prove(cachedR1CS, cachedPK, fullWitness)
	if err != nil {
		return false, err
	}

	publicWitness, err := fullWitness.Public()
	if err != nil {
		return false, err
	}

	if err := groth16.Verify(proof, cachedVK, publicWitness); err != nil {
		return false, err
	}

	return true, nil
}
