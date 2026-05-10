# A Beginner's Guide to zkVMs — built with riscVM

This repo is a working, from-scratch zkVM (zero-knowledge virtual machine) written in Go. It runs real RISC-V programs and can cryptographically prove that each instruction was executed correctly — without revealing the execution trace itself.

If you've heard the words "zkVM", "ZK rollup", or "validity proof" and wanted to understand what they actually mean, this codebase is designed to show you, piece by piece.

---

## What problem does a zkVM solve?

Imagine you run a computation — say, a Fibonacci function that runs for a million steps — and you get an answer. Now imagine you want to convince someone else that your answer is correct, without making them re-run the entire computation themselves.

Traditionally you'd have to share all your work: every step, every intermediate value. The other person checks it all. This is slow and defeats the point of offloading computation.

A **zkVM** gives you a better deal: you run the program, and at the end you produce a short **proof** — a few hundred bytes — that cryptographically guarantees your answer is correct. The verifier checks only the proof (milliseconds of work), not the execution. This is the core idea behind ZK rollups, zkEVMs, and verifiable compute.

---

## The three layers every zkVM has

Every zkVM — from simple demos like this one to production systems like SP1, Risc Zero, or zkSync's ZKEVM — is built from the same three conceptual layers:

```
┌─────────────────────────────────────────────┐
│  1. Execution Engine                        │
│     Runs the program. Produces a trace.     │
├─────────────────────────────────────────────┤
│  2. Arithmetization                         │
│     Turns the trace into math constraints.  │
├─────────────────────────────────────────────┤
│  3. Proving System                          │
│     Proves the constraints are satisfied    │
│     without revealing the trace itself.     │
└─────────────────────────────────────────────┘
```

This repo implements all three. Let's walk through each one.

---

## Layer 1: The Execution Engine (`vm/`)

The VM is a RISC-V RV32I emulator. RISC-V is a real, open CPU instruction set with 47 base instructions. Programs compiled for RISC-V run here.

The **fetch-decode-execute cycle** is the heartbeat of any CPU:

```
┌──────────┐    ┌──────────┐    ┌──────────┐
│  Fetch   │───▶│  Decode  │───▶│ Execute  │
│          │    │          │    │          │
│ Read the │    │ Figure   │    │ Update   │
│ next     │    │ out what │    │ registers│
│ 32-bit   │    │ the bits │    │ and PC   │
│ instr    │    │ mean     │    │          │
└──────────┘    └──────────┘    └──────────┘
        ▲                              │
        └──────────────────────────────┘
                  (repeat)
```

The CPU state at any point is just:
- **PC** — the program counter (which instruction to fetch next)
- **32 registers** — `x0` through `x31` (x0 is always 0)
- **16 MB of memory**

Every instruction takes a "before" state and produces an "after" state. This is the key insight for ZK: each step is a deterministic state transition, which means it can be expressed as a mathematical constraint.

The emulator also captures a **trace** — a log of every step's before/after register values and memory accesses. This trace is the raw input to the ZK layer.

Key files:
- `vm/cpu.go` — the Step loop and syscall handler
- `vm/decode.go` — bitfield extraction for all instruction formats (R/I/S/B/U/J types)
- `vm/instruction.go` — Execute() implementations for all 47 instructions
- `vm/memory.go` — byte-addressable memory with OOB error reporting
- `vm/trace.go` — the execution trace data structure

---

## Layer 2: Arithmetization (`zk/circuit.go`)

This is where the magic happens, and also where most zkVM tutorials lose people. Let's go slow.

### What is a "constraint"?

A constraint is just an equation that must hold. For example:

```
result = left + right
```

If you can show that all your constraints are satisfied, you've proven the computation is correct.

The ZK proof system works over a **finite field** (in this project, BN254). Every value — register contents, PC, instruction bits — is treated as a number in this field. Constraints become polynomial equations.

### What is R1CS?

This project uses **Rank-1 Constraint Systems (R1CS)** via the [gnark](https://github.com/ConsenSys/gnark) library. An R1CS constraint looks like:

```
(a · x) * (b · y) = (c · z)
```

The clever part: you can express any computation as a collection of these. Addition is free; multiplication costs one constraint.

### How one RISC-V step becomes constraints

For each instruction step, the circuit (`zk/circuit.go`) must prove:

1. **Instruction decoding is correct** — the opcode, rd, rs1, rs2, funct3, funct7 fields all match the raw 32-bit instruction word (enforced by bit decomposition constraints).

2. **The immediate value is honest** — the Imm field is reconstructed inside the circuit from the raw Instr bits according to the instruction format (I/S/B/J/U-type). A prover cannot supply a fake Imm and have the circuit accept it.

3. **The register result is correct** — for an ADD instruction, `RegsAfter[rd] == RegsBefore[rs1] + RegsBefore[rs2]`. For a shift, the barrel-shifter constraints fire. For a comparison, a 33-bit subtraction captures the borrow/sign bit.

4. **The right register was written** — a multiplexer checks that only `RegsAfter[rd]` changed; all other registers must equal their before values.

5. **The PC advanced correctly** — whether by 4 (sequential), by a branch offset (if the branch condition holds), or to a jump target.

Here is what a single ADD step looks like as a constraint, conceptually:

```
RegsBefore[rs1] + RegsBefore[rs2] == RegsAfter[rd]   (if opcode is ADD)
RegsAfter[i] == RegsBefore[i]                         (for all i ≠ rd)
PCAfter == PCBefore + 4                               (no branch)
```

The circuit enforcese all of these simultaneously for every possible instruction, using boolean selectors (`IsAdd`, `IsSub`, `isBranch`, etc.) multiplied against the relevant result.

### Key concept: one circuit, all instructions

The circuit doesn't branch for different opcodes — it **computes all possible outcomes** and then selects the right one using multiplexers. This is required because ZK circuits are static; they have no "if/else" at the circuit level, only linear combinations and products.

```
targetVal = IsAdd * resAdd
          + IsSub * resSub  
          + IsAnd * resAnd
          + IsAddi * resAddi
          + ...
```

Because exactly one selector is 1 and the rest are 0, only one term survives.

---

## Layer 3: The Proving System

### What proof system is used?

gnark supports two backends, both used in tests:
- **Groth16** — a pairing-based SNARK. Very short proofs (~200 bytes), fast verification, but requires a per-circuit trusted setup.
- **PLONK** — a universal SNARK. No per-circuit setup needed (uses a universal SRS), slightly larger proofs.

Both operate over the **BN254 elliptic curve** (also called alt_bn128), the same curve used by Ethereum's precompiles — which is why Groth16 proofs can be verified on-chain cheaply.

### The workflow

```
Circuit definition (circuit.go)
         │
         ▼
   Compile to R1CS
         │
         ▼
 Trusted Setup / SRS ──────► Proving key  + Verification key
         │                         │                │
         ▼                         ▼                ▼
  Witness (prover fills      groth16.Prove()   groth16.Verify()
  in concrete values)             │                │
                                  ▼                ▼
                               Proof ─────────► true / false
```

In this repo, the gnark `test` harness runs the full prove/verify cycle automatically in `zk/zk_test.go`. The WASM dashboard calls the real Groth16 prover on every step — `ProveStep` compiles the circuit once, runs `groth16.Setup` once (cached via `sync.Once`), then calls `groth16.Prove` and `groth16.Verify` for each instruction. The "Proof Verified" badge in the dashboard reflects a genuine cryptographic proof.

---

## Project Structure

```
riscVM/
├── main.go              CLI entry point
├── vm/
│   ├── cpu.go           CPU state, Step loop, run loop, syscalls
│   ├── decode.go        Instruction decoder + immediate extractors
│   ├── instruction.go   Execute() for all 47 RV32I instructions
│   ├── memory.go        16MB memory with OOB error reporting
│   ├── trace.go         Execution trace (per-step snapshots)
│   ├── format.go        Disassembler (for UI labels)
│   ├── elf.go           ELF32 loader
│   └── cpu_test.go      33 VM instruction-level unit tests
├── zk/
│   ├── circuit.go       gnark R1CS circuit — the heart of the zkVM
│   ├── prover.go        Witness generator + cached Groth16 prove/verify
│   ├── zk_test.go       11 single-step prove/verify tests (ADD, SUB, ADDI,
│   │                     BEQ, LUI, JALR, FENCE, LW, SW, ECALL, EBREAK, CSRRW)
│   └── integration_test.go  Full-program proof: 7-step program with ALU,
│                         load, store, and branch, proved end-to-end
├── cmd/wasm/
│   └── main.go          Go→JS bridge (syscall/js)
├── web/
│   ├── index.html
│   ├── main.js          UI: register grid, memory window, proof panel
│   └── style.css
└── examples/
    ├── hello.s          Hello world
    ├── fact.s           Factorial
    └── complexity.elf   Pre-compiled demo binary
```

---

## Quick Start

### Native CLI

```bash
# Build
go build -o riscvm .

# Run a RISC-V ELF binary
./riscvm examples/complexity.elf
```

### Web Dashboard

```bash
# Build WASM
GOOS=js GOARCH=wasm go build -o web/riscvm.wasm ./cmd/wasm

# Serve locally
cd web && python3 -m http.server 8080
# Open http://localhost:8080
```

### Tests

```bash
# 33 VM instruction unit tests
go test ./vm/...

# ZK circuit prove/verify (11 single-step + 1 full-program integration)
go test -v ./zk/...

# Everything
go test ./...
```

---

## What the tests prove

The ZK test suite (`zk/zk_test.go`) exercises the full gnark prove/verify pipeline:

- **ADD x3, x1, x2** with `x1=10, x2=20` → expects `x3=30, PC+=4`
- **SUB x4, x3, x1** with `x3=30, x1=10` → expects `x4=20, PC+=4`
- **ADDI x5, x1, 5** with `x1=10` → expects `x5=15, PC+=4`
- **BEQ x1, x2, +8** — tested for both taken (`x1==x2`, PC jumps) and not-taken (`x1!=x2`, PC advances normally)
- **LUI x6, 0x12345** → expects `x6=0x12345000, PC+=4`
- **JALR x7, x1, 4** with `x1=100` → expects `x7=PC+4, PCAfter=104`
- **FENCE** — treated as a no-op; all registers unchanged, PC advances by 4
- **LW / SW** — load constrains `MemVal → rd` and effective address `rs1 + imm`; store constrains `MemVal == rs2` and effective address
- **ECALL / EBREAK** — treated as no-ops; registers unchanged, PC advances by 4
- **CSRRW** — CSR instructions treated as no-ops (register state unchanged)

All cases are checked under the BN254 curve. If you supply a wrong witness (e.g. lie about the result), gnark rejects it.

The integration test (`zk/integration_test.go`) runs a complete 7-step RISC-V program — ALU ops, a store, a load, and a taken branch that skips an instruction — through the VM with tracing enabled, then proves every step end-to-end.

---

## What to implement next

Here are concrete next steps, roughly in order of difficulty:

### Intermediate

**4. Memory permutation argument**
The circuit now constrains individual load and store semantics (effective address, store value = rs2, load value written to rd). But it does not prove that a load returns the same value that a prior store wrote to that address. The standard technique is a **memory permutation argument**: sort all (address, timestamp, value) memory access tuples and prove with a grand-product check that no value was forged. This is what PLOOKUP, Halo2's lookup tables, and similar systems are designed for.

**5. Recursive proof aggregation**
Instead of proving N steps separately (as the integration test does now), use gnark's `std/recursion` package to recursively aggregate them: prove that a Groth16 proof is valid inside another Groth16 circuit. This compresses N proofs into 1 constant-size proof regardless of program length.

**6. Compressed witness representation**
The current witness sends all 32 registers for every step. In practice, most instructions touch at most 3 registers. Use a sparse representation and constraint that untouched registers carry over unchanged.

### Advanced

**7. On-chain verifier**
Export the Groth16 verification key and generate a Solidity verifier with `gnark`'s `backend/groth16/bn254/solidity` exporter. Deploy it and submit your proof transaction — the EVM will verify it for ~250k gas.

**8. RV32IM extension**
Add the M extension: MUL, MULH, MULHU, MULHSU, DIV, DIVU, REM, REMU. Multiplication in a ZK circuit is expensive (each bit of the product needs a constraint), so this is where circuit optimization starts to matter.

**9. Continuations / segmented proving**
Programs longer than ~100k steps become impractical to prove in one shot (memory and time). Split execution into fixed-size **segments**, prove each segment separately, then prove that segments stitch together (matching boundary state). This is how SP1 and Risc Zero handle arbitrary-length programs.

---

## Further Reading

- [gnark documentation](https://docs.gnark.consensys.io) — the constraint library used here
- [RISC-V ISA Specification](https://github.com/riscv/riscv-isa-manual) — the official ISA manual
- [From AIRs to RAPs](https://eprint.iacr.org/2023/1082) — how execution traces become polynomial constraints
- [Groth16 paper](https://eprint.iacr.org/2016/260) — the proof system behind Groth16
- [Risc Zero whitepaper](https://www.risczero.com/whitepaper.pdf) — a production zkVM using RISC-V
- [SP1 book](https://succinctlabs.github.io/sp1/) — another RISC-V zkVM, heavily optimized
