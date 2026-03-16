package zk

import (
	"github.com/consensys/gnark/frontend"
)

// StepCircuit proves a single execution step for a subset of RV32I.
type StepCircuit struct {
	// Public inputs
	PCBefore frontend.Variable `gnark:",public"`
	PCAfter  frontend.Variable `gnark:",public"`

	// Registers (32 registers)
	RegsBefore [32]frontend.Variable `gnark:",public"`
	RegsAfter  [32]frontend.Variable `gnark:",public"`

	// Witness: the raw instruction
	Instr frontend.Variable

	// Intermediate witnesses (to be constrained against Instr)
	Rd     frontend.Variable
	Rs1    frontend.Variable
	Rs2    frontend.Variable
	Imm    frontend.Variable
	Opcode frontend.Variable
	Funct3 frontend.Variable
	Funct7 frontend.Variable
}

func (c *StepCircuit) Define(api frontend.API) error {
	// 1. Constrain x0 is always 0
	api.AssertIsEqual(c.RegsBefore[0], 0)
	api.AssertIsEqual(c.RegsAfter[0], 0)

	// 2. Bit decomposition of Instr (32 bits)
	bits := api.ToBinary(c.Instr, 32)

	// Opcode: bits[0:7]
	api.AssertIsEqual(c.Opcode, bitsToVal(api, bits[0:7]))
	// Rd: bits[7:12]
	api.AssertIsEqual(c.Rd, bitsToVal(api, bits[7:12]))
	// Funct3: bits[12:15]
	api.AssertIsEqual(c.Funct3, bitsToVal(api, bits[12:15]))
	// Rs1: bits[15:20]
	api.AssertIsEqual(c.Rs1, bitsToVal(api, bits[15:20]))
	// Rs2: bits[20:25]
	api.AssertIsEqual(c.Rs2, bitsToVal(api, bits[20:25]))
	// Funct7: bits[25:32]
	api.AssertIsEqual(c.Funct7, bitsToVal(api, bits[25:32]))

	// TODO: Constrain Imm based on Opcode type (I-type, S-type, etc.)
	// For now, we still trust Imm as witness but we should constrain it eventually.

	// 3. Select source register values
	val1 := selectReg(api, c.RegsBefore, c.Rs1)
	val2 := selectReg(api, c.RegsBefore, c.Rs2)

	// 4. Compute results
	resAdd := api.Add(val1, val2)
	resAddi := api.Add(val1, c.Imm)
	resSub := api.Sub(val1, val2)

	// 5. Instruction Selectors
	// For OP (0x33): Add (F3=0, F7=0x00), Sub (F3=0, F7=0x20)
	// For OP-IMM (0x13): Addi (F3=0)
	
	isOp := api.IsZero(api.Sub(c.Opcode, 0x33))
	isOpImm := api.IsZero(api.Sub(c.Opcode, 0x13))
	isF3Zero := api.IsZero(c.Funct3)
	isF7Zero := api.IsZero(c.Funct7)
	isF7Sub := api.IsZero(api.Sub(c.Funct7, 0x20))

	IsAdd := api.Mul(isOp, isF3Zero, isF7Zero)
	IsSub := api.Mul(isOp, isF3Zero, isF7Sub)
	IsAddi := api.Mul(isOpImm, isF3Zero)

	api.AssertIsEqual(api.Add(IsAdd, IsAddi, IsSub), 1)

	// 6. Target Value
	targetVal := api.Add(
		api.Mul(IsAdd, resAdd),
		api.Mul(IsAddi, resAddi),
		api.Mul(IsSub, resSub),
	)

	// 7. Constrain RegsAfter
	for i := 0; i < 32; i++ {
		if i == 0 {
			continue
		}
		isRd := api.IsZero(api.Sub(c.Rd, i))
		expected := api.Select(isRd, targetVal, c.RegsBefore[i])
		api.AssertIsEqual(c.RegsAfter[i], expected)
	}

	// 8. Constrain PCAfter (for non-jump/branch)
	api.AssertIsEqual(c.PCAfter, api.Add(c.PCBefore, 4))

	return nil
}

func bitsToVal(api frontend.API, bits []frontend.Variable) frontend.Variable {
	res := frontend.Variable(0)
	for i := 0; i < len(bits); i++ {
		res = api.Add(res, api.Mul(bits[i], 1<<i))
	}
	return res
}

func selectReg(api frontend.API, regs [32]frontend.Variable, index frontend.Variable) frontend.Variable {
	res := frontend.Variable(0)
	for i := 0; i < 32; i++ {
		isIndex := api.IsZero(api.Sub(index, i))
		res = api.Select(isIndex, regs[i], res)
	}
	return res
}
