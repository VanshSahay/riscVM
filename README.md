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

The circuit proves that one RISC-V instruction executed correctly. It works over the BN254 finite field and compiles to a Rank-1 Constraint System (R1CS) via [gnark](https://github.com/ConsenSys/gnark).

For each instruction, the circuit enforces five things:

1. **Instruction decoding** — opcode, rd, rs1, rs2, funct3, funct7 all match the raw 32-bit instruction word.
2. **Immediate reconstruction** — the Imm field matches the instruction bits for the active format (I/S/B/J/U-type).
3. **Register result** — the ALU output is correct for the decoded opcode (e.g. ADD, SUB, shift, comparison).
4. **Register write** — only `RegsAfter[rd]` changes; all other registers stay the same.
5. **PC transition** — PC advances by 4, a branch offset, or a jump target depending on the opcode.

The circuit is static (no branches) — it computes every possible result and multiplexes the correct one using one-hot selectors (`IsAdd`, `IsSub`, etc.). Only one selector is 1, so only one term contributes to the output.

For a detailed walkthrough, see `zk/README.md`.

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
├── main.go              CLI entry point (run + evm subcommands)
├── build_evm.sh         Build script for the EVM interpreter
├── evm_asm.py           EVM bytecode assembler
├── evm/
│   ├── src/
│   │   ├── main.rs      EVM interpreter (no_std Rust for RISC-V)
│   │   └── u256.rs      256-bit arithmetic
│   ├── link.ld          RISC-V linker script
│   └── Cargo.toml
├── vm/
│   ├── cpu.go           CPU state, Step loop, run loop, syscalls
│   ├── decode.go        Instruction decoder + immediate extractors
│   ├── instruction.go   Execute() for all 47 RV32I instructions
│   ├── memory.go        16MB memory with OOB error reporting
│   ├── trace.go         Execution trace (per-step snapshots)
│   ├── format.go        Disassembler (for UI labels)
│   ├── elf.go           ELF32 loader
│   └── cpu_test.go      VM instruction-level unit tests
├── zk/
│   ├── circuit.go       gnark R1CS circuit — the heart of the zkVM
│   ├── prover.go        Witness generator + cached Groth16 prove/verify
│   ├── zk_test.go       Per-instruction prove/verify tests
│   └── integration_test.go  Full-program proof: 7-step program with ALU,
│                         load, store, and branch, proved end-to-end
├── cmd/wasm/
│   └── main.go          Go→JS bridge (syscall/js)
├── web/
│   ├── index.html
│   ├── main.js          UI: register grid, memory window, proof panel
│   └── style.css
└── examples/
    ├── hello.s / hello.elf        Hello world
    ├── fact.s / fact.elf          Factorial
    ├── complexity.s / complexity.elf  Fibonacci demo
    ├── evm.elf                    EVM interpreter RISC-V binary
    └── erc20.evm                  ERC20 contract (assembly)
```

---

## Quick Start

### Native CLI

```bash
# Build
go build -o riscvm .

# Run a RISC-V ELF binary
./riscvm run examples/complexity.elf

# Build the EVM interpreter
./build_evm.sh

# Run EVM bytecode
./riscvm evm examples/fact.evm -c examples/fact_calldata.bin

# Chain EVM calls with persistent storage
./riscvm evm examples/erc20.bin -c mint.calldata -s /tmp/erc20.state
./riscvm evm examples/erc20.bin -c transfer.calldata -s /tmp/erc20.state
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


---

## Proving EVM Bytecode

The zkVM proves EVM execution by running an EVM interpreter *as a RISC-V program* — the same approach production zkVMs like SP1 and Risc Zero use. The interpreter (`evm/`) is written in `no_std` Rust targeting RV32I, compiled to a RISC-V ELF, and loaded into the zkVM.

```
EVM bytecode ──▶ EVM interpreter (RISC-V ELF) ──▶ zkVM proves RISC-V trace
```

The interpreter supports ~50 EVM opcodes: arithmetic (ADD, MUL, SUB, DIV, EXP, MOD...), bitwise ops (AND, OR, XOR, NOT, SHL, SHR, SAR), comparisons (LT, GT, EQ, ISZERO), stack manipulation (POP, DUP1–16, SWAP1–16, PUSH1–32), memory (MLOAD, MSTORE, MSTORE8), storage (SLOAD, SSTORE), control flow (JUMP, JUMPI, JUMPDEST), calldata (CALLDATALOAD, CALLDATASIZE, CALLDATACOPY), and RETURN/REVERT. All values are full 256-bit (8 × u32 limbs).

At runtime the zkVM:
1. Loads the interpreter ELF and the target EVM bytecode
2. Places the bytecode + calldata in VM memory at a known address (0x800000)
3. Runs the interpreter, which reads the bytecode, executes it, and writes return data back
4. Each RISC-V instruction of the interpreter is cryptographically proven by the circuit

The integration test (`zk/integration_test.go`) already verifies that the zkVM can prove complete programs end-to-end — proving EVM execution is a direct application of that capability at a larger scale.

---

## What to implement next

Here are concrete next steps, roughly in order of difficulty:

### Intermediate

**1. Memory permutation argument**
The circuit now constrains individual load and store semantics (effective address, store value = rs2, load value written to rd). But it does not prove that a load returns the same value that a prior store wrote to that address. The standard technique is a **memory permutation argument**: sort all (address, timestamp, value) memory access tuples and prove with a grand-product check that no value was forged. This is what PLOOKUP, Halo2's lookup tables, and similar systems are designed for.

\*\*2\. Recursive proof aggregation**
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
- [Risc Zero](https://dev.risczero.com/) — a production zkVM using RISC-V
- [SP1](https://docs.succinct.xyz/) — another RISC-V zkVM, heavily optimized
