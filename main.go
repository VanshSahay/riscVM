package main

import (
	"fmt"
	"os"

	"github.com/VanshSahay/riscvm/vm"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <riscv32-elf-binary>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nExample:\n")
		fmt.Fprintf(os.Stderr, "  riscv32-unknown-elf-gcc -march=rv32i -mabi=ilp32 -nostdlib -o prog.elf prog.s\n")
		fmt.Fprintf(os.Stderr, "  %s prog.elf\n", os.Args[0])
		os.Exit(1)
	}

	mem := vm.NewMemory(0) // uses DefaultMemSize (16MB)
	entry, err := vm.LoadELF(os.Args[1], mem)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load ELF: %v\n", err)
		os.Exit(1)
	}

	cpu := vm.NewCPU(mem)
	cpu.PC = entry
	cpu.Regs[2] = uint32(len(mem.Data)) // sp = top of memory (stack grows down)

	exitCode, err := cpu.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "VM error: %v\n", err)
		os.Exit(1)
	}
	os.Exit(exitCode)
}
