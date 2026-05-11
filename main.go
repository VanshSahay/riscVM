package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"

	"github.com/VanshSahay/riscvm/vm"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "run":
		runCmd(os.Args[2:])
	case "evm":
		evmCmd(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	s := filepath.Base(os.Args[0])
	fmt.Fprintf(os.Stderr, "Usage:\n")
	fmt.Fprintf(os.Stderr, "  %s run <riscv32-elf-binary>            Run a RISC-V ELF binary\n", s)
	fmt.Fprintf(os.Stderr, "  %s evm <evm-bytecode>                  Run EVM bytecode on the zkVM\n", s)
	fmt.Fprintf(os.Stderr, "  %s evm -c <calldata> <evm-bytecode>    Run with calldata\n", s)
	fmt.Fprintf(os.Stderr, "\nEVM flags:\n")
	fmt.Fprintf(os.Stderr, "  --interpreter, -i <path>   EVM interpreter ELF (default: examples/evm.elf)\n")
	fmt.Fprintf(os.Stderr, "  --calldata, -c <path>      Calldata file (hex or raw bytes)\n")
	fmt.Fprintf(os.Stderr, "  --trace, -t                Enable execution tracing\n")
}


func runCmd(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s run <riscv32-elf-binary>\n", filepath.Base(os.Args[0]))
		os.Exit(1)
	}
	mem := vm.NewMemory(0)
	entry, err := vm.LoadELF(args[0], mem)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load ELF: %v\n", err)
		os.Exit(1)
	}

	cpu := vm.NewCPU(mem)
	cpu.PC = entry
	cpu.Regs[2] = uint32(len(mem.Data)) // sp = top of memory

	exitCode, err := cpu.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "VM error: %v\n", err)
		os.Exit(1)
	}
	os.Exit(exitCode)
}


const evmInputAddr = 0x800000
const storageSlots = 256
const storageByteSize = storageSlots * 32

func evmCmd(args []string) {
	interpreter := "examples/evm.elf"
	var calldataFile string
	var storageFile string
	trace := false
	var positional []string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--interpreter", "-i":
			i++
			if i < len(args) {
				interpreter = args[i]
			}
		case "--calldata", "-c":
			i++
			if i < len(args) {
				calldataFile = args[i]
			}
		case "--storage", "-s":
			i++
			if i < len(args) {
				storageFile = args[i]
			}
		case "--trace", "-t":
			trace = true
		default:
			positional = append(positional, args[i])
		}
	}

	if len(positional) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s evm [flags] <evm-bytecode>\n", filepath.Base(os.Args[0]))
		os.Exit(1)
	}

	code, err := os.ReadFile(positional[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read bytecode: %v\n", err)
		os.Exit(1)
	}

	code = maybeHexDecode(code)

	var calldata []byte
	if calldataFile != "" {
		calldata, err = os.ReadFile(calldataFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to read calldata: %v\n", err)
			os.Exit(1)
		}
		calldata = maybeHexDecode(calldata)
	}

	var storageInit []byte
	if storageFile != "" {
		storageInit, err = os.ReadFile(storageFile)
		if err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Failed to read storage: %v\n", err)
			os.Exit(1)
		}
	}

	finalStorage, exitCode := runEVM(interpreter, code, calldata, storageInit, trace)

	// Save storage if requested (before exit)
	if storageFile != "" && len(finalStorage) == storageByteSize {
		if err := os.WriteFile(storageFile, finalStorage, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write storage: %v\n", err)
		}
	}

	os.Exit(exitCode)
}

func runEVM(interpreterPath string, code, calldata, storageInit []byte, trace bool) ([]byte, int) {
	mem := vm.NewMemory(0)

	// Load the EVM interpreter ELF
	entry, err := vm.LoadELF(interpreterPath, mem)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load EVM interpreter ELF (%s): %v\n", interpreterPath, err)
		os.Exit(1)
	}

	// Write EVM input at 0x800000:
	// [code_len: u32 LE][calldata_len: u32 LE][has_storage: u32 LE][code...][calldata...]
	// [if has_storage: STORAGE_SLOTS*32 bytes storage data]
	writeEVMInput(mem, code, calldata, storageInit)

	cpu := vm.NewCPU(mem)
	cpu.PC = entry
	cpu.Regs[2] = uint32(len(mem.Data))

	if trace {
		cpu.Trace = &vm.Trace{}
	}

	exitCode, vmErr := cpu.Run()
	if vmErr != nil {
		fmt.Fprintf(os.Stderr, "VM error: %v\n", vmErr)
		os.Exit(1)
	}

	if trace && cpu.Trace != nil && len(*cpu.Trace) > 0 {
		fmt.Fprintf(os.Stderr, "[trace] %d instructions executed\n", len(*cpu.Trace))
	}

	// Read return data + final storage
	retLen, retData, finalStorage := readEVMResult(mem)

	// Print result but don't exit — let caller handle storage persistence
	switch exitCode {
	case 0:
		if retLen > 0 {
			fmt.Printf("%x\n", retData)
		}
	case 2:
		fmt.Fprintf(os.Stderr, "EVM: REVERT\n")
		if retLen > 0 {
			fmt.Fprintf(os.Stderr, "revert data: %x\n", retData)
		}
	case 3:
		fmt.Fprintf(os.Stderr, "EVM: ran out of steps\n")
	default:
		fmt.Fprintf(os.Stderr, "EVM: exit code %d\n", exitCode)
	}
	return finalStorage, exitCode
}

func writeEVMInput(mem *vm.Memory, code, calldata, storageInit []byte) {
	const addr = evmInputAddr
	binary.LittleEndian.PutUint32(mem.Data[addr:addr+4], uint32(len(code)))
	binary.LittleEndian.PutUint32(mem.Data[addr+4:addr+8], uint32(len(calldata)))
	hasStorage := uint32(0)
	if len(storageInit) >= storageByteSize {
		hasStorage = 1
	}
	binary.LittleEndian.PutUint32(mem.Data[addr+8:addr+12], hasStorage)
	copy(mem.Data[addr+12:], code)
	copy(mem.Data[addr+12+len(code):], calldata)
	if hasStorage == 1 {
		copy(mem.Data[addr+12+len(code)+len(calldata):], storageInit[:storageByteSize])
	}
}

func readEVMResult(mem *vm.Memory) (int, []byte, []byte) {
	const addr = evmInputAddr
	length := int(binary.LittleEndian.Uint32(mem.Data[addr : addr+4]))
	data := make([]byte, length)
	copy(data, mem.Data[addr+4:addr+4+length])

	// Final storage is STORAGE_SLOTS*32 bytes after the return data
	storageOff := addr + 4 + length
	storage := make([]byte, storageByteSize)
	copy(storage, mem.Data[storageOff:storageOff+storageByteSize])
	return length, data, storage
}

func maybeHexDecode(b []byte) []byte {
	s := string(b)
	if len(s) >= 2 && s[:2] == "0x" {
		s = s[2:]
	}
	// Strip whitespace
	cleaned := make([]byte, 0, len(s))
	for _, c := range []byte(s) {
		if c != ' ' && c != '\n' && c != '\r' && c != '\t' {
			cleaned = append(cleaned, c)
		}
	}
	if len(cleaned)%2 != 0 {
		return b // not hex
	}
	out := make([]byte, len(cleaned)/2)
	for i := 0; i < len(out); i++ {
		var hi, lo byte
		h := cleaned[i*2]
		l := cleaned[i*2+1]
		if h >= '0' && h <= '9' {
			hi = h - '0'
		} else if h >= 'a' && h <= 'f' {
			hi = h - 'a' + 10
		} else if h >= 'A' && h <= 'F' {
			hi = h - 'A' + 10
		} else {
			return b
		}
		if l >= '0' && l <= '9' {
			lo = l - '0'
		} else if l >= 'a' && l <= 'f' {
			lo = l - 'a' + 10
		} else if l >= 'A' && l <= 'F' {
			lo = l - 'A' + 10
		} else {
			return b
		}
		out[i] = hi<<4 | lo
	}
	return out
}
