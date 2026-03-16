package zk

import (
	"testing"

	"github.com/VanshSahay/riscvm/vm"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/test"
)

func TestStepCircuit(t *testing.T) {
	assert := test.NewAssert(t)

	var circuit StepCircuit

	// ADD x3, x1, x2 (Instr: 0x002081b3)
	// Before: PC=8, x1=10, x2=20
	// After: PC=12, x1=10, x2=20, x3=30
	current := vm.TraceStep{
		PC:    8,
		Instr: 0x002081b3,
	}
	current.Regs[1] = 10
	current.Regs[2] = 20
	
	nextPC := uint32(12)
	var nextRegs [32]uint32
	nextRegs = current.Regs
	nextRegs[3] = 30

	witnessAdd := GenerateWitness(current, nextPC, nextRegs)
	assert.ProverSucceeded(&circuit, &witnessAdd, test.WithCurves(ecc.BN254))

	// SUB x4, x3, x1 (Instr: 0x40118233)
	// Before: PC=12, x3=30, x1=10
	// After: PC=16, x4=20
	currentSub := vm.TraceStep{
		PC:    12,
		Instr: 0x40118233,
		Regs:  nextRegs,
	}
	nextPCSub := uint32(16)
	var nextRegsSub [32]uint32
	nextRegsSub = nextRegs
	nextRegsSub[4] = 20

	witnessSub := GenerateWitness(currentSub, nextPCSub, nextRegsSub)
	assert.ProverSucceeded(&circuit, &witnessSub, test.WithCurves(ecc.BN254))
}
