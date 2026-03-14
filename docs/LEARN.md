# RISC-V and riscVM: A Complete Beginner's Guide

This document explains RISC-V from scratch and how this project (riscVM) implements it. **No prior knowledge of assembly or CPU design is required.** If you know basic programming (variables, functions) and want to understand how a computer actually runs code—this is for you.

---

## Table of Contents

1. [The Big Picture](#part-1-the-big-picture) — What is RISC-V? Why a VM?
2. [How a CPU Works](#part-2-how-a-cpu-works-high-level) — Fetch, decode, execute
3. [RISC-V Basics](#part-3-risc-v-basics) — Registers, memory, byte order
4. [Instruction Formats](#part-4-instruction-formats) — How 32-bit instructions are structured
5. [The RV32I Instruction Set](#part-5-the-rv32i-instruction-set-what-riscvm-implements) — Every instruction we support
6. [ELF Files](#part-6-elf-files--how-programs-are-stored) — How compiled programs are stored on disk
7. [Syscalls](#part-7-syscalls--talking-to-the-environment) — How programs talk to the outside world
8. [How riscVM Fits Together](#part-8-how-riscvm-fits-together) — The big picture of this codebase
9. [Walking Through hello.s](#part-9-walking-through-exampleshellos) — Line-by-line explanation of our example
10. [Building and Running](#part-10-building-and-running) — Get it working on your machine
11. [Extending Toward a zkVM](#part-11-extending-riscvm-eg-toward-a-zkvm) — Where you might go next
12. [Further Reading](#part-12-further-reading) — Official specs and manuals

---

## Part 1: The Big Picture

### What is RISC-V?

**RISC-V** (pronounced "risk-five") is an **instruction set architecture (ISA)**. That’s a technical way of saying: *the interface between software and hardware*.

- The ISA defines:
  - What instructions the CPU understands
  - How data and addresses are represented
  - How programs are stored in memory
- An ISA is like the API of a CPU:
  - Software (compilers, programs) targets this API
  - Hardware (chips) implements it

So RISC-V is a **specification**, not a specific chip. Different companies build CPUs that implement this spec, and software written for RISC-V can run on any of them.

### Why RISC-V matters

- **Open and royalty-free** – anyone can implement it
- **Modular** – small base, optional extensions (M, A, F, D, C…)
- **Widely used** – embedded, servers, IoT, research, zkVMs
- **Clean design** – relatively small and regular, good for learning

### Why a “virtual machine” (VM)?

A **virtual machine** (or “emulator”) is a program that *pretends* to be a CPU. It:

1. Reads instructions one by one
2. Figures out what each instruction does
3. Updates internal state (registers, memory) to match

So you get a software CPU that runs RISC-V programs without real RISC-V hardware.

---

## Part 2: How a CPU Works (High Level)

A CPU does one thing, repeatedly:

```
1. FETCH   – read the next instruction from memory at address PC
2. DECODE  – figure out what that instruction means
3. EXECUTE – perform the operation, update state
4. Update PC (usually advance to the next instruction)
```

**PC = Program Counter** – the address in memory of the instruction currently being executed.

Everything is driven by **instructions**: fixed-size patterns of bits that tell the CPU what to do. In RISC-V, every instruction is **32 bits**.

---

## Part 3: RISC-V Basics

### Registers

Registers are the CPU’s built-in storage. They hold values the CPU is currently using (numbers, addresses, etc.).

RISC-V has **32 general-purpose registers** named `x0` through `x31`:


| Number | Name   | Role (by convention)  |
| ------ | ------ | --------------------- |
| x0     | zero   | Always 0 (read-only)  |
| x1     | ra     | Return address        |
| x2     | sp     | Stack pointer         |
| x3     | gp     | Global pointer        |
| x4     | tp     | Thread pointer        |
| x5–7   | t0–t2  | Temporary             |
| x8     | s0/fp  | Saved / frame pointer |
| x9     | s1     | Saved                 |
| x10–11 | a0–a1  | Arguments, return val |
| x12–17 | a2–a7  | Arguments, syscall #  |
| x18–27 | s2–s11 | Saved                 |
| x28–31 | t3–t6  | Temporary             |


Each register holds a **32-bit value** in RV32I. The riscVM stores these in `cpu.Regs[0]` through `cpu.Regs[31]`.

### Memory

- Memory is a large array of **bytes**, each with an **address** (starting at 0).
- The CPU reads and writes memory in units of 1, 2, or 4 bytes.
- Addresses in RV32I are **32-bit**, so up to about 4 GB.
- In riscVM, memory is a byte slice (`mem.Data`), 16 MB by default.

### Byte order (endianness)

RISC-V is **little-endian**: the least significant byte is at the lowest address.  
Example: value `0x12345678` at address `0x100`:

- `0x100` → `0x78`
- `0x101` → `0x56`
- `0x102` → `0x34`
- `0x103` → `0x12`

---

## Part 4: Instruction Formats

All RISC-V base instructions are **32 bits**. Different bits mean different things depending on the format.

There are **6 main formats**:


| Format | Bits layout (rough)                                | Used for                   |
| ------ | -------------------------------------------------- | -------------------------- |
| R-type | opcode | rd | funct | rs1 | rs2 | funct7           | Register ops (add, sub, …) |
| I-type | opcode | rd | funct3 | rs1 | imm[11:0]             | Immediate ops, loads, jalr |
| S-type | opcode | imm[11:5] | rs2 | rs1 | funct3 | imm[4:0] | Stores                     |
| B-type | opcode | imm | rs2 | rs1 | funct3 | imm            | Branches                   |
| U-type | opcode | rd | imm[31:12]                           | lui, auipc                 |
| J-type | opcode | rd | imm[20:1]                            | jal                        |


- **opcode** (7 bits): identifies the instruction type.
- **rd**: destination register.
- **rs1**, **rs2**: source registers.
- **imm**: immediate (constant) encoded in the instruction.
- **funct3**, **funct7**: extra control bits for related instructions.

The decoder in `vm/decode.go` extracts these fields from each 32-bit instruction and chooses the right instruction type.

---

## Part 5: The RV32I Instruction Set (What riscVM Implements)

RV32I is the **base 32-bit integer** ISA: only these instructions, no extensions.

### Arithmetic (register with register)


| Instruction       | Meaning                                             |
| ----------------- | --------------------------------------------------- |
| add rd, rs1, rs2  | rd = rs1 + rs2                                      |
| sub rd, rs1, rs2  | rd = rs1 - rs2                                      |
| and rd, rs1, rs2  | rd = rs1 & rs2 (bitwise AND)                        |
| or rd, rs1, rs2   | rd = rs1 | rs2                                      |
| xor rd, rs1, rs2  | rd = rs1 ^ rs2                                      |
| sll rd, rs1, rs2  | rd = rs1 << (rs2 & 0x1F)                            |
| srl rd, rs1, rs2  | rd = rs1 >> (rs2 & 0x1F) (logical)                  |
| sra rd, rs1, rs2  | rd = rs1 >> (rs2 & 0x1F) (arithmetic, sign-extends) |
| slt rd, rs1, rs2  | rd = 1 if (signed) rs1 < rs2 else 0                 |
| sltu rd, rs1, rs2 | rd = 1 if (unsigned) rs1 < rs2 else 0               |


### Arithmetic (register with immediate)


| Instruction                     | Meaning                              |
| ------------------------------- | ------------------------------------ |
| addi rd, rs1, imm               | rd = rs1 + imm                       |
| andi, ori, xori rd, rs1, imm    | Bitwise ops with constant            |
| slti, sltiu rd, rs1, imm        | Set if less than (signed / unsigned) |
| slli, srli, srai rd, rs1, shamt | Shifts by constant (0–31)            |


### Load upper immediate and PC-relative


| Instruction   | Meaning               |
| ------------- | --------------------- |
| lui rd, imm   | rd = imm << 12        |
| auipc rd, imm | rd = PC + (imm << 12) |


### Memory


| Instruction                 | Meaning                                |
| --------------------------- | -------------------------------------- |
| lb rd, offset(rs1)          | Load byte (sign-extended)              |
| lh rd, offset(rs1)          | Load halfword (16 bits, sign-extended) |
| lw rd, offset(rs1)          | Load word (32 bits)                    |
| lbu rd, offset(rs1)         | Load byte unsigned                     |
| lhu rd, offset(rs1)         | Load halfword unsigned                 |
| sb, sh, sw rs2, offset(rs1) | Store byte, half, word                 |


Effective address = `rs1 + offset` (offset is signed 12-bit).

### Control flow


| Instruction                 | Meaning                                     |
| --------------------------- | ------------------------------------------- |
| beq rs1, rs2, offset        | if rs1 == rs2: PC += offset                 |
| bne rs1, rs2, offset        | if rs1 != rs2: PC += offset                 |
| blt, bge rs1, rs2, offset   | if (signed) rs1 < resp. ≥ rs2: PC += offset |
| bltu, bgeu rs1, rs2, offset | Same, unsigned                              |
| jal rd, offset              | rd = PC+4; PC += offset                     |
| jalr rd, rs1, offset        | rd = PC+4; PC = (rs1 + offset) & ~1         |


### System


| Instruction     | Meaning                                    |
| --------------- | ------------------------------------------ |
| ecall           | Call OS / environment (syscalls in riscVM) |
| ebreak          | Breakpoint / debug trap                    |
| fence / fence.i | Ordering (no-ops in single-hart riscVM)    |


---

## Part 6: ELF Files – How Programs Are Stored

When you compile a program, the linker produces an **ELF** (Executable and Linking Format) file. It contains:

1. **Headers** – machine type, entry point, etc.
2. **Segments (PT_LOAD)** – code and data to load into memory
3. **Symbol table, debug info, etc.** – for linking and debugging

What riscVM cares about:

- **Machine**: RISC-V
- **Class**: 32-bit (ELFCLASS32)
- **Entry point**: address of the first instruction
- **Loadable segments**: what bytes go where in memory

The loader in `vm/elf.go`:

1. Opens the file and parses it as ELF.
2. Checks it’s RISC-V 32-bit.
3. For each `PT_LOAD` segment, copies its bytes into `mem.Data` at the segment’s virtual address.
4. Returns the entry point so `main` can set `cpu.PC`.

---

## Part 7: Syscalls – Talking to the Environment

Programs often need to:

- Print to the console
- Read input
- Exit cleanly
- Allocate memory

The CPU doesn’t do those directly. Instead, it uses **syscalls**: special instructions that hand control to the OS (or, in our case, the VM).

On RISC-V/Linux the convention is:

- **a7** = syscall number
- **a0–a5** = arguments
- **a0** = return value after the call

riscVM supports:


| Syscall # | Name  | a0   | a1  | a2  | Meaning                  |
| --------- | ----- | ---- | --- | --- | ------------------------ |
| 63        | read  | fd   | buf | len | Read into buffer from fd |
| 64        | write | fd   | buf | len | Write buffer to fd       |
| 93        | exit  | code | -   | -   | Exit with code           |
| 214       | brk   | addr | -   | -   | Set/query heap break     |


Standard file descriptors: 0 = stdin, 1 = stdout, 2 = stderr.

When the program hits `ecall`, the VM checks `a7` and runs the matching handler in `handleSyscall()`.

---

## Part 8: How riscVM Fits Together

### Flow

```
main.go
  → vm.LoadELF(elfPath, mem)   → loads binary into mem, returns entry
  → cpu.PC = entry
  → cpu.Run()
      → loop: fetch at PC → decode → execute (or handle ecall)
      → until exit(93) or ebreak
  → os.Exit(exitCode)
```

### Files


| File                | Role                                                             |
| ------------------- | ---------------------------------------------------------------- |
| `main.go`           | Load ELF, set entry point, run VM, exit with program’s exit code |
| `vm/memory.go`      | Byte array, Load/Store for byte, half, word                      |
| `vm/cpu.go`         | CPU state (PC, Regs), Step(), Run(), syscall handling            |
| `vm/decode.go`      | Turns 32-bit instruction into an Instruction value               |
| `vm/instruction.go` | Instruction types and Execute() for each                         |
| `vm/elf.go`         | Load ELF, map segments into memory, return entry                 |


### Execution loop (simplified)

```go
for {
    instr := mem.LoadWord(PC)
    decoded := Decode(instr)
    if ecall → handle syscall, maybe exit
    else
        decoded.Execute(cpu)
        if !decoded.ModifiesPC() { PC += 4 }
}
```

---

## Part 9: Walking Through `examples/hello.s`

```assembly
.section .text
.globl _start
_start:
    # write(1, msg, 13)  -- stdout, buffer, length
    li   a7, 64          # syscall number for write
    li   a0, 1           # fd 1 = stdout
    la   a1, msg         # address of string (pseudo-instruction → auipc + addi)
    li   a2, 13          # length
    ecall

    # exit(0)
    li   a7, 93
    li   a0, 0
    ecall

.section .rodata
msg: .asciz "Hello, RISC-V!\n"
```

- `.section .text`: code goes here
- `.globl _start`: make `_start` visible to the linker
- `_start`: where execution begins (ELF entry point)
- `li rd, imm`: load immediate (pseudo-instruction for addi/addi/ori/lui)
- `la a1, msg`: load address of `msg` (pseudo-instruction)
- `ecall`: perform syscall using a7, a0, a1, a2
- `.section .rodata`: read-only data
- `.asciz`: null-terminated string

Sequence: setup a7, a0, a1, a2 → `ecall` → write to stdout; then setup for exit → `ecall` → exit(0).

---

## Part 10: Building and Running

### 1. Build riscVM

```bash
cd /path/to/riscVM
go build -o riscvm .
```

### 2. Install RISC-V toolchain (macOS)

```bash
brew install riscv-gnu-toolchain
```

### 3. Build a RISC-V program

```bash
cd examples
riscv64-unknown-elf-gcc -march=rv32i -mabi=ilp32 -nostdlib -nostartfiles -o hello.elf hello.s
```

- `-march=rv32i`: base 32-bit ISA only (matches riscVM)
- `-mabi=ilp32`: 32-bit pointers and longs
- `-nostdlib -nostartfiles`: no C runtime, use our `_start`

### 4. Run in the VM

```bash
../riscvm hello.elf
```

You should see: `Hello, RISC-V!`

---

## Part 11: Extending riscVM (e.g., Toward a zkVM)

A zkVM proves that a program was run correctly without rerunning it. To add that layer:

1. **Execution trace** – log every instruction and state change.
2. **Constraints** – turn each step into arithmetic constraints for a proof system.
3. **Proof generation** – run a proving backend (e.g., Groth16, PLONK) on those constraints.

The current VM gives you:

- A clear decode → execute pipeline
- All RV32I instructions in one place
- State (PC, Regs, Mem) that a prover can reference

The design keeps decode, execute, and state separate, so you can later plug in trace recording and constraint generation without rewriting the whole VM.

---

## Part 12: Further Reading

- [RISC-V Specification](https://riscv.org/technical/specifications/)
- [RISC-V Assembly Programmer’s Manual](https://github.com/riscv-non-isa/riscv-asm-manual)
- [RISC-V ELF psABI](https://riscv-non-isa.github.io/riscv-elf-psabi-doc/) (calling conventions, ELF layout)

---

*This guide was written for the riscVM project. If something is unclear or you find an error, open an issue or submit a PR.*