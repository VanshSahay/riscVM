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

	// 2. Decode Instr — assert all decoded fields match instruction bits
	bits := api.ToBinary(c.Instr, 32)
	api.AssertIsEqual(c.Opcode, bitsToVal(api, bits[0:7]))
	api.AssertIsEqual(c.Rd, bitsToVal(api, bits[7:12]))
	api.AssertIsEqual(c.Funct3, bitsToVal(api, bits[12:15]))
	api.AssertIsEqual(c.Rs1, bitsToVal(api, bits[15:20]))
	api.AssertIsEqual(c.Rs2, bitsToVal(api, bits[20:25]))
	api.AssertIsEqual(c.Funct7, bitsToVal(api, bits[25:32]))

	// 3. Constrain Imm from instruction bits per format.
	//
	// Each format reconstructs its immediate from the bits of Instr and asserts
	// equality with c.Imm.  Only one format's selector is 1 at a time, so we
	// compute all candidates and do a multiplexed assertion:
	//   constrainedImm = sum_over_formats(selector * candidate)
	// then assert constrainedImm == c.Imm.
	//
	// Formats covered: I, S, B, J, U.  R-type has no immediate (Imm must be 0).

	isOp := api.IsZero(api.Sub(c.Opcode, 0x33))    // R-type
	isOpImm := api.IsZero(api.Sub(c.Opcode, 0x13)) // I-type arith
	isLoad := api.IsZero(api.Sub(c.Opcode, 0x03))  // I-type load
	isStore := api.IsZero(api.Sub(c.Opcode, 0x23)) // S-type
	isLui := api.IsZero(api.Sub(c.Opcode, 0x37))   // U-type
	isAuipc := api.IsZero(api.Sub(c.Opcode, 0x17)) // U-type
	isJal := api.IsZero(api.Sub(c.Opcode, 0x6F))   // J-type
	isJalr := api.IsZero(api.Sub(c.Opcode, 0x67))  // I-type
	isBranch := api.IsZero(api.Sub(c.Opcode, 0x63))
	isFence := api.IsZero(api.Sub(c.Opcode, 0x0F)) // FENCE / FENCE.I (no-op)

	// I-type immediate: sign-extended bits[31:20]
	// Raw 12-bit value; sign bit is bits[31].
	iImmBits := make([]frontend.Variable, 32)
	for i := 0; i < 11; i++ {
		iImmBits[i] = bits[i+20]
	}
	// Sign-extend: replicate bits[31] for positions 11..31
	for i := 11; i < 32; i++ {
		iImmBits[i] = bits[31]
	}
	iImm := bitsToVal(api, iImmBits)

	// S-type immediate: {bits[31:25], bits[11:7]}, sign-extended
	sImmBits := make([]frontend.Variable, 32)
	for i := 0; i < 5; i++ {
		sImmBits[i] = bits[i+7]
	}
	for i := 5; i < 11; i++ {
		sImmBits[i] = bits[i+20]
	}
	for i := 11; i < 32; i++ {
		sImmBits[i] = bits[31]
	}
	sImm := bitsToVal(api, sImmBits)

	// B-type immediate: {bits[31], bits[7], bits[30:25], bits[11:8], 0}, sign-extended
	bImmBits := make([]frontend.Variable, 32)
	bImmBits[0] = frontend.Variable(0)
	for i := 1; i < 5; i++ {
		bImmBits[i] = bits[i+7]
	}
	for i := 5; i < 11; i++ {
		bImmBits[i] = bits[i+20]
	}
	bImmBits[11] = bits[7]
	for i := 12; i < 32; i++ {
		bImmBits[i] = bits[31]
	}
	bImm := bitsToVal(api, bImmBits)

	// J-type immediate: {bits[31], bits[19:12], bits[20], bits[30:21], 0}, sign-extended
	jImmBits := make([]frontend.Variable, 32)
	jImmBits[0] = frontend.Variable(0)
	for i := 1; i < 11; i++ {
		jImmBits[i] = bits[i+20]
	}
	jImmBits[11] = bits[20]
	for i := 12; i < 20; i++ {
		jImmBits[i] = bits[i]
	}
	for i := 20; i < 32; i++ {
		jImmBits[i] = bits[31]
	}
	jImm := bitsToVal(api, jImmBits)

	// U-type immediate: upper 20 bits placed at [31:12], lower 12 zero
	uImmBits := make([]frontend.Variable, 32)
	for i := 0; i < 12; i++ {
		uImmBits[i] = frontend.Variable(0)
	}
	for i := 12; i < 32; i++ {
		uImmBits[i] = bits[i]
	}
	uImm := bitsToVal(api, uImmBits)

	isItype := api.Add(isOpImm, isLoad, isJalr)  // all I-type opcodes
	constrainedImm := api.Add(
		api.Mul(isItype, iImm),
		api.Mul(isStore, sImm),
		api.Mul(isBranch, bImm),
		api.Mul(isJal, jImm),
		api.Mul(api.Add(isLui, isAuipc), uImm),
		// R-type: Imm must be 0 (no explicit term needed; absence enforces it)
	)
	api.AssertIsEqual(c.Imm, constrainedImm)

	// 4. Instruction Selection
	isF3_0 := api.IsZero(c.Funct3)
	isF3_1 := api.IsZero(api.Sub(c.Funct3, 1))
	isF3_2 := api.IsZero(api.Sub(c.Funct3, 2))
	isF3_4 := api.IsZero(api.Sub(c.Funct3, 4))
	isF3_5 := api.IsZero(api.Sub(c.Funct3, 5))
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
	IsSll := api.Mul(isOp, isF3_1, isF7_0)
	IsSrl := api.Mul(isOp, isF3_5, isF7_0)
	IsSra := api.Mul(isOp, isF3_5, isF7_20)
	IsSlt := api.Mul(isOp, isF3_2, isF7_0)
	IsSltu := api.Mul(isOp, isF3_3(api, c.Funct3), isF7_0)

	// I-type
	IsAddi := api.Mul(isOpImm, isF3_0)
	IsAndi := api.Mul(isOpImm, isF3_7)
	IsOri := api.Mul(isOpImm, isF3_6)
	IsXori := api.Mul(isOpImm, isF3_4)
	IsSlli := api.Mul(isOpImm, isF3_1)
	IsSrli := api.Mul(isOpImm, isF3_5, isF7_0)
	IsSrai := api.Mul(isOpImm, isF3_5, isF7_20)
	IsSlti := api.Mul(isOpImm, isF3_2)
	IsSltiu := api.Mul(isOpImm, isF3_3(api, c.Funct3))

	// 5. Arithmetic / Logic
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

	// Shifts: shamt is low 5 bits of Rs2 (R-type) or Imm (I-type)
	shamtBits2 := api.ToBinary(val2, 32)
	resSll := shiftLeft(api, bits1, shamtBits2[:5])
	shamtBitsImm := api.ToBinary(c.Imm, 32)
	resSlli := shiftLeft(api, bits1, shamtBitsImm[:5])

	resSrl := shiftRight(api, bits1, shamtBits2[:5], false)
	resSrli := shiftRight(api, bits1, shamtBitsImm[:5], false)
	resSra := shiftRight(api, bits1, shamtBits2[:5], true)
	resSrai := shiftRight(api, bits1, shamtBitsImm[:5], true)

	// SLT / SLTU: comparison using 33-bit arithmetic.
	//
	// To keep values in range for ToBinary, we add 2^32 before subtracting so
	// the result is always non-negative in the field (val1, val2 are 32-bit bounded
	// because they came through ToBinary(32) above).
	// Bit 32 of (2^32 + val1 - val2) is 1 iff val1 < val2 unsigned.
	// For signed SLT we reinterpret the 32-bit values as signed by subtracting
	// 2^31 from both before the unsigned comparison.
	two32 := frontend.Variable(uint64(1) << 32)
	two31 := frontend.Variable(uint64(1) << 31)

	// Unsigned: bit32 of (2^32 + val1 - val2) == 1 iff val1 <u val2
	diffU := api.Add(two32, api.Sub(val1, val2))
	diffUBits := api.ToBinary(diffU, 33)
	resSltu := api.Sub(1, diffUBits[32]) // borrow: 0 means val1 >= val2, 1 means val1 < val2
	// When val1 < val2 unsigned: 2^32 + val1 - val2 < 2^32, so bit32 == 0 → resSltu = 1. ✓

	// Signed: shift both operands by -2^31 to turn signed comparison into unsigned
	signedVal1 := api.Sub(val1, two31)
	signedVal2 := api.Sub(val2, two31)
	diffS := api.Add(two32, api.Sub(signedVal1, signedVal2))
	diffSBits := api.ToBinary(diffS, 33)
	resSlt := api.Sub(1, diffSBits[32])

	// Slti/Sltiu: same trick but compare val1 against Imm
	diffUI := api.Add(two32, api.Sub(val1, c.Imm))
	diffUIBits := api.ToBinary(diffUI, 33)
	resSltiu := api.Sub(1, diffUIBits[32])

	signedImmVal := api.Sub(c.Imm, two31)
	diffSI := api.Add(two32, api.Sub(signedVal1, signedImmVal))
	diffSIBits := api.ToBinary(diffSI, 33)
	resSlti := api.Sub(1, diffSIBits[32])

	// 6. Target Register Value
	targetVal := api.Add(
		api.Mul(IsAdd, resAdd),
		api.Mul(IsSub, resSub),
		api.Mul(IsAnd, resAnd),
		api.Mul(IsOr, resOr),
		api.Mul(IsXor, resXor),
		api.Mul(IsSll, resSll),
		api.Mul(IsSrl, resSrl),
		api.Mul(IsSra, resSra),
		api.Mul(IsSlt, resSlt),
		api.Mul(IsSltu, resSltu),
		api.Mul(IsAddi, resAddi),
		api.Mul(IsAndi, resAndi),
		api.Mul(IsOri, resOri),
		api.Mul(IsXori, resXori),
		api.Mul(IsSlli, resSlli),
		api.Mul(IsSrli, resSrli),
		api.Mul(IsSrai, resSrai),
		api.Mul(IsSlti, resSlti),
		api.Mul(IsSltiu, resSltiu),
		api.Mul(isLui, resLui),
		api.Mul(isAuipc, resAuipc),
		api.Mul(api.Add(isJal, isJalr), api.Add(c.PCBefore, 4)),
	)

	// 7. Constrain RegsAfter
	for i := 0; i < 32; i++ {
		if i == 0 {
			continue
		}
		isRd := api.IsZero(api.Sub(c.Rd, i))
		// Stores, branches, and fences don't write to Rd
		writesRd := api.Sub(1, api.Add(isStore, isBranch, isFence))
		expected := api.Select(api.Mul(isRd, writesRd), targetVal, c.RegsBefore[i])
		api.AssertIsEqual(c.RegsAfter[i], expected)
	}

	// 8. Control Flow (PCAfter)
	isBeq := api.Mul(isBranch, isF3_0)
	isBne := api.Mul(isBranch, isF3_1)
	isBlt := api.Mul(isBranch, isF3_4)
	isBge := api.Mul(isBranch, isF3_5)
	isBltu := api.Mul(isBranch, isF3_6)
	isBgeu := api.Mul(isBranch, isF3_7)

	condBeq := api.IsZero(api.Sub(val1, val2))
	condBne := api.Sub(1, condBeq)

	// Reuse the signed/unsigned comparisons already computed for SLT/SLTU above.
	condBlt := resSlt
	condBge := api.Sub(1, resSlt)
	condBltu := resSltu
	condBgeu := api.Sub(1, resSltu)

	branchTaken := api.Add(
		api.Mul(isBeq, condBeq),
		api.Mul(isBne, condBne),
		api.Mul(isBlt, condBlt),
		api.Mul(isBge, condBge),
		api.Mul(isBltu, condBltu),
		api.Mul(isBgeu, condBgeu),
	)

	pcBranch := api.Add(c.PCBefore, c.Imm)
	pcJal := api.Add(c.PCBefore, c.Imm)

	// Jalr: LSB clear
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

// isF3_3 returns IsZero(funct3 - 3), avoiding a named variable collision
func isF3_3(api frontend.API, funct3 frontend.Variable) frontend.Variable {
	return api.IsZero(api.Sub(funct3, 3))
}

// shiftLeft computes val << shamt where shamt is 5 bits.
// Implemented as a barrel shifter over the bit decomposition.
func shiftLeft(api frontend.API, valBits []frontend.Variable, shamtBits []frontend.Variable) frontend.Variable {
	// Apply single-bit shifts: for each shamt bit i, conditionally shift by 2^i.
	cur := make([]frontend.Variable, 32)
	copy(cur, valBits)
	for s := 0; s < 5; s++ {
		shift := 1 << s
		next := make([]frontend.Variable, 32)
		for i := 0; i < 32; i++ {
			var shifted frontend.Variable
			if i-shift >= 0 {
				shifted = cur[i-shift]
			} else {
				shifted = frontend.Variable(0)
			}
			next[i] = api.Select(shamtBits[s], shifted, cur[i])
		}
		cur = next
	}
	return bitsToVal(api, cur)
}

// shiftRight computes val >> shamt (logical or arithmetic) where shamt is 5 bits.
func shiftRight(api frontend.API, valBits []frontend.Variable, shamtBits []frontend.Variable, arithmetic bool) frontend.Variable {
	cur := make([]frontend.Variable, 32)
	copy(cur, valBits)
	signBit := valBits[31]
	for s := 0; s < 5; s++ {
		shift := 1 << s
		next := make([]frontend.Variable, 32)
		for i := 0; i < 32; i++ {
			var fill frontend.Variable
			if arithmetic {
				fill = signBit
			} else {
				fill = frontend.Variable(0)
			}
			var shifted frontend.Variable
			if i+shift < 32 {
				shifted = cur[i+shift]
			} else {
				shifted = fill
			}
			next[i] = api.Select(shamtBits[s], shifted, cur[i])
		}
		cur = next
	}
	return bitsToVal(api, cur)
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
