# riscVM Verification Dashboard

A high-fidelity, WASM-powered dashboard designed for the visualization and cryptographic verification of RISC-V execution.

## The Web-ZK Pipeline

Every instruction executed in the dashboard flows through a complex pipeline to maintain state and generate proofs:

1. **Instruction Step:** The JS frontend calls the `riscvmStep()` function (exported from Go/WASM).
2. **Trace Capture:** Within the Go runtime, the `vm.TraceStep` is recorded, capturing the pre-execution CPU state and the instruction word.
3. **State Sync:** The new CPU state (PC, registers) is reflected in the UI's real-time grid.
4. **Proof Generation:** When **Verify Last Step** is triggered:
  - `riscvmVerifyLastStep()` is called.
  - `zk.GenerateWitness` is executed to construct the circuit inputs.
  - A **Proof Certificate** object is generated, containing the cryptographic state diff.
5. **Rendering:** The frontend renders the certificate using a structured, minimalist UI that highlights the exact registers modified.

## WASM Interface Implementation (`/cmd/wasm/main.go`)

The dashboard relies on the `syscall/js` package to expose a suite of internal VM functions to the browser global scope:

- `riscvmLoadProgram(Uint8Array, bool)`: Parses ELF or raw hex and initializes the CPU.
- `riscvmStep()`: Executes a single Fetch-Decode-Execute cycle.
- `riscvmVerifyLastStep()`: Proves the integrity of the most recent step.
- `riscvmGetMemory(offset, length)`: Synchronizes a view of the VM's 16MB heap with the browser.

## Component Architecture

### CPU Panel

- **Register Grid:** Displays all 32 general-purpose registers (`x0-x31`) in hexadecimal format.
- **PC Tracker:** A real-time indicator of the Program Counter.

### Memory View

- **Smart Windowing:** Only renders a 256-byte window around the current PC to maintain 60fps performance.
- **Instruction Highlighting:** Automatically highlights the memory line corresponding to the current `PC`.

### ZK Prover Console

- **Proof Status:** Shows the status of the prover ("Proving...", "Proof verified").
- **Proof Certificate:** A structured, high-fidelity view of the cryptographic proof:
  - Instruction disassembled view.
  - Before/After PC transition.
  - **Register Diffs:** Only registers changed by the instruction are shown (e.g., `x3: 0x0000000a -> 0x0000001e`).

## Build & Execution

### 1. Rebuild the WASM Engine

```bash
# Execute from the project root
GOOS=js GOARCH=wasm go build -o web/riscvm.wasm ./cmd/wasm
```

### 2. Local Environment Setup

Serve the assets over HTTP (required for WASM execution):

```bash
cd web
# Option A: Python
python3 -m http.server 8080
# Option B: Node.js
npx serve .
```

### 3. Loading a Program

- **ELF:** Compile with `riscv64-unknown-elf-gcc -march=rv32i -mabi=ilp32 -nostdlib -o prog.elf prog.s`. Get the base64 via `base64 < prog.elf`.
- **Hex:** Paste raw space-separated hex bytes.

## Design Philosophy

The interface follows a **Sleek Minimalist** aesthetic:

- **Contrast-Centric:** High-contrast text on a deep background (`#0a0a0a`).
- **Nord Palette:** Uses colors like `#88c0d0` (Frost) for verification headers and certificates.

