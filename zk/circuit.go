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

	// Witness: the raw instruction and its decoded fields
	Instr  frontend.Variable
	Rd     frontend.Variable
	Rs1    frontend.Variable
	Rs2    frontend.Variable
	Imm    frontend.Variable
	Opcode frontend.Variable
	Funct3 frontend.Variable
	Funct7 frontend.Variable
}

func (c *StepCircuit) Define(api frontend.API) error {
	// 1. x0 is always 0
	api.AssertIsEqual(c.RegsBefore[0], 0)
	api.AssertIsEqual(c.RegsAfter[0], 0)

	// 2. Decode Instr
	bits := api.ToBinary(c.Instr, 32)
	api.AssertIsEqual(c.Opcode, bitsToVal(api, bits[0:7]))
	api.AssertIsEqual(c.Rd, bitsToVal(api, bits[7:12]))
	api.AssertIsEqual(c.Funct3, bitsToVal(api, bits[12:15]))
	api.AssertIsEqual(c.Rs1, bitsToVal(api, bits[15:20]))
	api.AssertIsEqual(c.Rs2, bitsToVal(api, bits[20:25]))
	api.AssertIsEqual(c.Funct7, bitsToVal(api, bits[25:32]))

	// 3. Instruction Selection
	isOp := api.IsZero(api.Sub(c.Opcode, 0x33))
	isOpImm := api.IsZero(api.Sub(c.Opcode, 0x13))
	isLui := api.IsZero(api.Sub(c.Opcode, 0x37))
	isAuipc := api.IsZero(api.Sub(c.Opcode, 0x17))
	isJal := api.IsZero(api.Sub(c.Opcode, 0x6F))
	isJalr := api.IsZero(api.Sub(c.Opcode, 0x67))
	isBranch := api.IsZero(api.Sub(c.Opcode, 0x63))

	isF3_0 := api.IsZero(c.Funct3)
	isF3_1 := api.IsZero(api.Sub(c.Funct3, 1))
	isF3_4 := api.IsZero(api.Sub(c.Funct3, 4))
	isF3_6 := api.IsZero(api.Sub(c.Funct3, 6))
	isF3_7 := api.IsZero(api.Sub(c.Funct3, 7))

	isF7_0 := api.IsZero(c.Funct7)
	isF7_20 := api.IsZero(api.Sub(c.Funct7, 0x20))

	// R-type
	IsAdd := api.Mul(isOp, isF3_0, isF7_0)
	IsSub := api.Mul(isOp, isF3_0, isF7_20)
	IsAnd := api.Mul(isOp, isF3_7, isF7_0)
	IsOr := api.Mul(isOp, isF3_6, isF7_0)
	IsXor := api.Mul(isOp, isF3_4, isF7_0)

	// I-type
	IsAddi := api.Mul(isOpImm, isF3_0)
	IsAndi := api.Mul(isOpImm, isF3_7)
	IsOri := api.Mul(isOpImm, isF3_6)
	IsXori := api.Mul(isOpImm, isF3_4)

	// 4. Arithmetic / Logic
	val1 := selectReg(api, c.RegsBefore, c.Rs1)
	val2 := selectReg(api, c.RegsBefore, c.Rs2)

	resAdd := api.Add(val1, val2)
	resSub := api.Sub(val1, val2)

	// Bitwise logic (And, Or, Xor)
	bits1 := api.ToBinary(val1, 32)
	bits2 := api.ToBinary(val2, 32)
	bitsImm := api.ToBinary(c.Imm, 32)

	resAnd := bitsToVal(api, bitwiseOp(api, bits1, bits2, "and"))
	resOr := bitsToVal(api, bitwiseOp(api, bits1, bits2, "or"))
	resXor := bitsToVal(api, bitwiseOp(api, bits1, bits2, "xor"))

	resAddi := api.Add(val1, c.Imm)
	resAndi := bitsToVal(api, bitwiseOp(api, bits1, bitsImm, "and"))
	resOri := bitsToVal(api, bitwiseOp(api, bits1, bitsImm, "or"))
	resXori := bitsToVal(api, bitwiseOp(api, bits1, bitsImm, "xor"))

	resLui := c.Imm
	resAuipc := api.Add(c.PCBefore, c.Imm)

	// 5. Target Register Value
	targetVal := api.Add(
		api.Mul(IsAdd, resAdd),
		api.Mul(IsSub, resSub),
		api.Mul(IsAnd, resAnd),
		api.Mul(IsOr, resOr),
		api.Mul(IsXor, resXor),
		api.Mul(IsAddi, resAddi),
		api.Mul(IsAndi, resAndi),
		api.Mul(IsOri, resOri),
		api.Mul(IsXori, resXori),
		api.Mul(isLui, resLui),
		api.Mul(isAuipc, resAuipc),
		api.Mul(api.Add(isJal, isJalr), api.Add(c.PCBefore, 4)),
	)

	// 6. Constrain RegsAfter
	for i := 0; i < 32; i++ {
		if i == 0 {
			continue
		}
		isRd := api.IsZero(api.Sub(c.Rd, i))
		expected := api.Select(isRd, targetVal, c.RegsBefore[i])
		api.AssertIsEqual(c.RegsAfter[i], expected)
	}

	// 7. Control Flow (PCAfter)
	// Branch logic
	isBeq := api.Mul(isBranch, isF3_0)
	isBne := api.Mul(isBranch, isF3_1)
	
	condBeq := api.IsZero(api.Sub(val1, val2))
	condBne := api.Sub(1, condBeq)

	branchTaken := api.Add(
		api.Mul(isBeq, condBeq),
		api.Mul(isBne, condBne),
		// TODO: BLT, BGE, etc.
	)

	pcBranch := api.Add(c.PCBefore, c.Imm)
	pcJal := api.Add(c.PCBefore, c.Imm)
	
	// Jalr: LSB clear - bitsJalr[0] = 0
	targetJalr := api.Add(val1, c.Imm)
	bitsJalr := api.ToBinary(targetJalr, 32)
	bitsJalr[0] = 0
	pcJalr := bitsToVal(api, bitsJalr)

	pcNext := api.Add(c.PCBefore, 4)

	finalPC := api.Select(isJal, pcJal,
		api.Select(isJalr, pcJalr,
			api.Select(branchTaken, pcBranch, pcNext)))

	api.AssertIsEqual(c.PCAfter, finalPC)

	return nil
}

func bitwiseOp(api frontend.API, bits1, bits2 []frontend.Variable, op string) []frontend.Variable {
	res := make([]frontend.Variable, 32)
	for i := 0; i < 32; i++ {
		switch op {
		case "and":
			res[i] = api.Mul(bits1[i], bits2[i])
		case "or":
			res[i] = api.Add(bits1[i], bits2[i], api.Mul(bits1[i], bits2[i], -1))
		case "xor":
			res[i] = api.Add(bits1[i], bits2[i], api.Mul(bits1[i], bits2[i], -2))
		}
	}
	return res
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
