# riscVM: High-Performance RISC-V zkVM

riscVM is a specialized implementation of the RISC-V RV32I Instruction Set Architecture (ISA) designed from the ground up for Zero-Knowledge (ZK) provability. It features a high-fidelity Go-based emulator, a constraint-optimized ZK circuit backend using [gnark](https://github.com/ConsenSys/gnark), and a sleek, reactive web interface for real-time proof inspection.

## System Architecture

The project is structured into four distinct layers:

1.  **Execution Engine (`/vm`):** A robust RV32I emulator that handles the Fetch-Decode-Execute cycle. It includes a custom ELF loader and a Linux-compatible syscall interface.
2.  **Constraint System (`/zk`):** An arithmetized representation of the RISC-V state transition. Every instruction is decomposed into a set of R1CS constraints that verify the integrity of the execution.
3.  **WASM Bridge (`/cmd/wasm`):** A high-throughput bridge that exposes the VM's internal state and ZK prover to the browser environment.
4.  **Verification Dashboard (`/web`):** A minimalist frontend that visualizes the execution trace and displays cryptographic "Proof Certificates" for every instruction step.

## Technical Specifications

| Feature | Specification |
| :--- | :--- |
| **ISA Support** | RV32I Base Integer (excluding Fences/CSRs) |
| **Memory Model** | 16MB Byte-addressable with bounds checking |
| **ZK Backend** | gnark (Groth16 / PLONK) |
| **Field** | BN254 (alt_bn128) |
| **Host Language** | Go 1.25+ |
| **Target** | WASM / Native CLI |

## Directory Structure

- `main.go`: The CLI entry point for native RISC-V execution.
- `vm/`: The core virtual machine.
  - `cpu.go`: State management (PC, 32 registers, status).
  - `decode.go`: RISC-V instruction parser using bitmasking.
  - `instruction.go`: Concrete implementations for 40+ instructions.
  - `trace.go`: Captures the execution history required for ZK witness generation.
- `zk/`: The Zero-Knowledge proving system.
  - `circuit.go`: The gnark circuit defining the transition function.
  - `prover.go`: Witness generation logic and proof orchestration.
- `cmd/wasm/`: JS-Go interop layer.
- `web/`: Assets for the browser-based dashboard.

## Quick Start

### Native CLI
Compile and run a RISC-V ELF binary:
```bash
go build -o riscvm .
./riscvm examples/complexity.elf
```

### Web Interface
1. **Build WASM:**
   ```bash
   GOOS=js GOARCH=wasm go build -o web/riscvm.wasm ./cmd/wasm
   ```
2. **Serve:**
   ```bash
   cd web && python3 -m http.server 8080
   ```

## Development & Testing

The project maintains a rigorous testing suite for both the VM and the ZK circuits.

```bash
# Test VM instruction accuracy
go test ./vm/...

# Test ZK circuit constraints
go test -v ./zk
```

## Vision & Roadmap

riscVM aims to be the most accessible educational and production-ready zkVM.
- [x] **Phase 1:** Core RV32I Emulation & Tracing.
- [x] **Phase 2:** Single-step ZK Proofs for Arithmetic & Control Flow.
- [x] **Phase 3:** High-fidelity Web Visualization.
- [ ] **Phase 4:** Memory Log constraints (Permutation Arguments).
- [ ] **Phase 5:** Recursive proof aggregation for full-trace verification.
