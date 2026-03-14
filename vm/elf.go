package vm

import (
	"debug/elf"
	"fmt"
	"os"
)

// LoadELF loads a RISC-V 32-bit ELF binary into memory and returns
// the entry point address. Compatible with binaries from riscv32-unknown-elf-gcc.
func LoadELF(path string, mem *Memory) (entry uint32, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	ef, err := elf.NewFile(f)
	if err != nil {
		return 0, fmt.Errorf("parse ELF: %w", err)
	}

	if ef.Class != elf.ELFCLASS32 {
		return 0, fmt.Errorf("not 32-bit ELF (class %v)", ef.Class)
	}
	if ef.Machine != elf.EM_RISCV {
		return 0, fmt.Errorf("not RISC-V ELF (machine %v)", ef.Machine)
	}

	entry = uint32(ef.Entry)

	for _, prog := range ef.Progs {
		if prog.Type != elf.PT_LOAD {
			continue
		}
		vaddr := uint32(prog.Vaddr)
		filesz := prog.Filesz
		memsz := prog.Memsz

		if vaddr+uint32(memsz) > uint32(len(mem.Data)) {
			return 0, fmt.Errorf("segment overflow: vaddr=0x%x memsz=0x%x", vaddr, memsz)
		}

		n, err := prog.ReadAt(mem.Data[vaddr:vaddr+uint32(filesz)], 0)
		if err != nil && err.Error() != "EOF" {
			return 0, fmt.Errorf("read segment: %w", err)
		}
		_ = n
		// Zero-fill remaining memsz beyond filesz
		for i := filesz; i < memsz; i++ {
			mem.Data[vaddr+uint32(i)] = 0
		}
	}

	return entry, nil
}
