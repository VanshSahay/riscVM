# riscVM

A RISC-V zkVM with EVM bytecode proving, written in Go. Runs RISC-V programs, executes EVM bytecode via a built-in interpreter, and generates Groth16 proofs for every instruction step — verifiable on-chain over BN254.

## Architecture

The zkVM has three layers:

### Execution Engine (`vm/`)

Full RV32I emulator with 47 base instructions, 32 registers, 16 MB of byte-addressable memory, and ELF32 loading. Captures a trace of every step — before/after register state, PC, and memory accesses.

Key files: `cpu.go`, `decode.go`, `instruction.go`, `memory.go`, `elf.go`, `trace.go`

### Arithmetization (`zk/circuit.go`)

gnark R1CS circuit over the BN254 curve. Proves each RISC-V step by constraining instruction decode, immediate reconstruction, ALU result, register write, and PC transition. Uses one-hot selectors to multiplex all possible instruction outcomes — the circuit is static (no branches).

See `zk/README.md` for the full constraint walkthrough.

### Proving System (`zk/prover.go`)

Groth16 over BN254 (the same curve as Ethereum's alt_bn128 precompile). Compiles the circuit once, caches the proving and verification keys, and generates a fresh proof for every step. Also supports PLONK via gnark's backend.

```
Circuit → R1CS → Trusted Setup → PK + VK
                                      ↓
Witness ← Execution Trace    groth16.Prove() → Proof
                                      ↓
                              groth16.Verify() → ✓
```

## EVM Bytecode Proving

EVM bytecode is proven by running an EVM interpreter as a RISC-V program — the same model used by SP1 and Risc Zero. The interpreter (`evm/`) is written in `no_std` Rust targeting RV32I, compiled to a RISC-V ELF, and loaded into the zkVM alongside the target bytecode.

```
EVM bytecode → EVM interpreter (RISC-V ELF) → zkVM proves RISC-V trace
```

Supports ~50 EVM opcodes with full 256-bit arithmetic: arithmetic, bitwise, comparisons, stack/memory/storage ops, control flow, calldata, and return/revert. Persistent storage across calls via `--storage`.

### Solidity support

The web UI accepts Solidity source and compiles it to EVM bytecode using a built-in compiler. The resulting bytecode is fed into the EVM interpreter and proved step-by-step.

## Project Structure

```
riscVM/
├── main.go              CLI (run + evm subcommands)
├── build_evm.sh         Build script for the EVM interpreter
├── evm_asm.py           EVM bytecode assembler
├── evm/
│   ├── src/
│   │   ├── main.rs      EVM interpreter (~50 opcodes, 256-bit)
│   │   └── u256.rs      256-bit arithmetic
│   ├── link.ld          RISC-V linker script
│   └── Cargo.toml
├── vm/
│   ├── cpu.go           CPU, Step loop, syscalls
│   ├── decode.go        Instruction decoder
│   ├── instruction.go   RV32I Execute() implementations
│   ├── memory.go        16 MB byte-addressable memory
│   ├── trace.go         Execution trace
│   ├── format.go        Disassembler
│   ├── elf.go           ELF32 loader
│   └── cpu_test.go      VM unit tests
├── zk/
│   ├── circuit.go       gnark R1CS circuit
│   ├── prover.go        Groth16 prove/verify (cached setup)
│   ├── zk_test.go       Per-instruction tests
│   └── integration_test.go  Full-program end-to-end proof
├── cmd/wasm/
│   └── main.go          WASM bridge (syscall/js)
├── web/
│   ├── index.html       Dashboard with Solidity support
│   ├── main.js          UI: CPU, memory, proof panel
│   └── style.css
└── examples/
    ├── hello.s / hello.elf
    ├── fact.s / fact.elf
    ├── complexity.s / complexity.elf
    ├── evm.elf           EVM interpreter binary
    └── erc20.evm         ERC20 contract
```

## Quick Start

### CLI

```bash
go build -o riscvm .

# Run a RISC-V ELF
./riscvm run examples/complexity.elf

# Build the EVM interpreter
./build_evm.sh

# Run EVM bytecode
./riscvm evm examples/fact.evm -c examples/fact_calldata.bin

# ERC20 with persistent storage
./riscvm evm examples/erc20.bin -c mint.calldata -s /tmp/erc20.state
./riscvm evm examples/erc20.bin -c transfer.calldata -s /tmp/erc20.state
```

### Web Dashboard

```bash
GOOS=js GOARCH=wasm go build -o web/riscvm.wasm ./cmd/wasm
cd web && python3 -m http.server 8080
# Open http://localhost:8080
```

Paste a RISC-V ELF (base64), EVM bytecode (hex), or Solidity source — the dashboard shows register state, memory, per-step proof certificates, and Groth16 verification status.

### Tests

```bash
go test ./vm/...     # VM unit tests
go test -v ./zk/...  # ZK circuit tests + integration test
go test ./...        # Everything
```

## Further Reading

- [gnark](https://docs.gnark.consensys.io)
- [RISC-V ISA](https://github.com/riscv/riscv-isa-manual)
- [Groth16](https://eprint.iacr.org/2016/260)
- [From AIRs to RAPs](https://eprint.iacr.org/2023/1082)
- [Risc Zero](https://dev.risczero.com/)
- [SP1](https://docs.succinct.xyz/)
