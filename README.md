# riscVM

A RISC-V RV32I-compatible VM written in Go. Designed as a foundation for building a zkVM.

## Features

- **RV32I base instruction set** (47 instructions): arithmetic, loads/stores, branches, jumps, fences
- **ELF loader** for RISC-V 32-bit executables (from `riscv32-unknown-elf-gcc`)
- **Linux-style syscalls** via ECALL: exit, write, read, brk
- **16MB memory** with bounds checking

## Usage

```bash
go build -o riscvm .
./riscvm <riscv32-elf-binary>
```

### Building RISC-V programs

Install the RISC-V GNU toolchain, then compile with **RV32I only** (no M/A/C extensions):

```bash
riscv64-unknown-elf-gcc -march=rv32i -mabi=ilp32 -nostdlib -nostartfiles -o hello.elf hello.s
../riscvm hello.elf
```

## Project layout

```
.
├── main.go           # Entry point, loads ELF and runs
├── vm/
│   ├── cpu.go       # CPU state, Step, Run, syscall handling
│   ├── decode.go    # RV32I instruction decoder
│   ├── elf.go       # ELF binary loader
│   ├── instruction.go  # Instruction types and Execute
│   └── memory.go    # Memory with Load/Store byte/half/word
└── examples/
    └── hello.s      # Minimal "Hello World" assembly
```

