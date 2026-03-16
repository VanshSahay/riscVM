# zkVM Architecture & Implementation

This directory contains the Zero-Knowledge Virtual Machine (zkVM) implementation for riscVM. It leverages the [gnark](https://github.com/ConsenSys/gnark) proof system to generate and verify proofs of correct execution for RISC-V RV32I instructions.

## Overview

The zkVM works by converting the execution of a RISC-V program into a set of arithmetic constraints (a "circuit"). If a prover can provide a valid witness (the values for all registers and intermediate states) that satisfies these constraints, it proves that the program was executed correctly according to the RISC-V specification.

## Components

### 1. Execution Tracing (`vm/trace.go`)
Before proving, the standard VM runs the program and records a **Trace**. Each `TraceStep` contains:
- `PC`: The program counter at the start of the step.
- `Instr`: The 32-bit raw instruction word.
- `Regs`: The state of all 32 registers before execution.
- `MemAddr/Val/Op`: Details of any memory access performed.

### 2. Step Circuit (`zk/circuit.go`)
The `StepCircuit` is the core gnark circuit that defines the transition from one state to the next for a single instruction.

**Constraints implemented:**
- **Instruction Decoding:** The 32-bit `Instr` is bit-decomposed. We constrain that the `Opcode`, `Rd`, `Rs1`, `Rs2`, `Funct3`, and `Funct7` witnesses match the bits of the raw instruction.
- **Register Access:** A "mux" logic (`selectReg`) ensures that the values used in arithmetic match the values stored in the `RegsBefore` array at indices `Rs1` and `Rs2`.
- **Instruction Logic:**
    - `ADD`: `RegsAfter[Rd] = RegsBefore[Rs1] + RegsBefore[Rs2]`
    - `SUB`: `RegsAfter[Rd] = RegsBefore[Rs1] - RegsBefore[Rs2]`
    - `ADDI`: `RegsAfter[Rd] = RegsBefore[Rs1] + Imm`
- **One-Hot Selection:** We compute "selectors" (boolean variables) based on the `Opcode` and `Funct` fields. We constrain that exactly one instruction type is active per step.
- **State Transition:**
    - `x0` is always constrained to be `0` in both `RegsBefore` and `RegsAfter`.
    - Only the register at index `Rd` can change; all others must be equal to their values in `RegsBefore`.
    - The `PCAfter` is currently constrained to `PCBefore + 4` (non-branching logic).

### 3. Witness Generation (`zk/prover.go`)
The `GenerateWitness` function acts as the bridge between the VM's execution trace and the ZK circuit. It takes a `TraceStep` and the resulting state and populates the `StepCircuit` struct with the concrete values required by the prover.

### 4. Verification (`zk/zk_test.go`)
Tests verify that valid execution steps (e.g., an `ADD` or `SUB`) result in satisfied circuits across different curves (BN254) and backend systems (Groth16, PLONK).

## Current Roadmap

- [x] Basic RV32I Step Circuit (ADD, SUB, ADDI)
- [x] Instruction bit-decomposition constraints
- [x] Automatic Witness Generation from Trace
- [ ] **Memory Constraints:** Implement a memory sub-circuit to prove correct Loads/Stores (using a Memory Log / Permutation Argument).
- [ ] **Control Flow:** Support Branching (`BEQ`, `BNE`, etc.) and Jumps (`JAL`, `JALR`) by constraining `PCAfter` based on arithmetic results.
- [ ] **Recursive Proofs:** Aggregate individual `StepCircuit` proofs into a single proof for the entire program execution.
- [ ] **Full RV32I Support:** Add remaining arithmetic, logical, and comparison instructions.

## Usage

To run the ZK tests and verify the circuit constraints:

```bash
go test -v ./zk
```
