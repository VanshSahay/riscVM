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

// TestAddi proves ADDI x5, x1, 5 with x1=10 → x5=15, PC+=4.
func TestAddi(t *testing.T) {
	assert := test.NewAssert(t)
	var circuit StepCircuit

	// ADDI x5, x1, 5 = 0x00508293
	cur := vm.TraceStep{PC: 20, Instr: 0x00508293}
	cur.Regs[1] = 10

	var next [32]uint32
	next = cur.Regs
	next[5] = 15

	w := GenerateWitness(cur, 24, next)
	assert.ProverSucceeded(&circuit, &w, test.WithCurves(ecc.BN254))
}

// TestBeq proves BEQ x1, x2, +8 for both the taken and not-taken paths.
func TestBeq(t *testing.T) {
	assert := test.NewAssert(t)
	var circuit StepCircuit

	// BEQ x1, x2, +8 = 0x00208463
	// Taken: x1 == x2, PC jumps to PC+8
	curTaken := vm.TraceStep{PC: 24, Instr: 0x00208463}
	curTaken.Regs[1] = 5
	curTaken.Regs[2] = 5

	var nextTaken [32]uint32
	nextTaken = curTaken.Regs

	wTaken := GenerateWitness(curTaken, 32, nextTaken) // 24 + 8
	assert.ProverSucceeded(&circuit, &wTaken, test.WithCurves(ecc.BN254))

	// Not taken: x1 != x2, PC advances by 4
	curNot := vm.TraceStep{PC: 32, Instr: 0x00208463}
	curNot.Regs[1] = 5
	curNot.Regs[2] = 6

	var nextNot [32]uint32
	nextNot = curNot.Regs

	wNot := GenerateWitness(curNot, 36, nextNot) // 32 + 4
	assert.ProverSucceeded(&circuit, &wNot, test.WithCurves(ecc.BN254))
}

// TestLui proves LUI x6, 0x12345 → x6=0x12345000, PC+=4.
func TestLui(t *testing.T) {
	assert := test.NewAssert(t)
	var circuit StepCircuit

	// LUI x6, 0x12345 = 0x12345337
	cur := vm.TraceStep{PC: 36, Instr: 0x12345337}

	var next [32]uint32
	next = cur.Regs
	next[6] = 0x12345000

	w := GenerateWitness(cur, 40, next)
	assert.ProverSucceeded(&circuit, &w, test.WithCurves(ecc.BN254))
}

// TestJalr proves JALR x7, x1, 4 with x1=100 → x7=PC+4=44, PCAfter=104.
func TestJalr(t *testing.T) {
	assert := test.NewAssert(t)
	var circuit StepCircuit

	// JALR x7, x1, 4 = 0x004083e7
	cur := vm.TraceStep{PC: 40, Instr: 0x004083e7}
	cur.Regs[1] = 100

	var next [32]uint32
	next = cur.Regs
	next[7] = 44 // PC + 4

	// PCAfter = (x1 + 4) & ~1 = 104
	w := GenerateWitness(cur, 104, next)
	assert.ProverSucceeded(&circuit, &w, test.WithCurves(ecc.BN254))
}

// TestFence proves FENCE leaves all registers unchanged and advances PC by 4.
func TestFence(t *testing.T) {
	assert := test.NewAssert(t)
	var circuit StepCircuit

	// FENCE = 0x0000000f
	cur := vm.TraceStep{PC: 44, Instr: 0x0000000f}
	cur.Regs[1] = 5
	cur.Regs[2] = 10

	var next [32]uint32
	next = cur.Regs // nothing changes

	w := GenerateWitness(cur, 48, next) // PC + 4
	assert.ProverSucceeded(&circuit, &w, test.WithCurves(ecc.BN254))
}
