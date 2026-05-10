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

// ── run ──────────────────────────────────────────────────────────

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

// ── evm ──────────────────────────────────────────────────────────

const evmInputAddr = 0x800000

func evmCmd(args []string) {
	interpreter := "examples/evm.elf"
	var calldataFile string
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

	// Strip 0x prefix if hex; otherwise treat as raw bytes
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

	runEVM(interpreter, code, calldata, trace)
}

func runEVM(interpreterPath string, code, calldata []byte, trace bool) {
	mem := vm.NewMemory(0)

	// Load the EVM interpreter ELF
	entry, err := vm.LoadELF(interpreterPath, mem)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load EVM interpreter ELF (%s): %v\n", interpreterPath, err)
		os.Exit(1)
	}

	// Write EVM input at 0x800000: [code_len: u32 LE][calldata_len: u32 LE][code...][calldata...]
	writeEVMInput(mem, code, calldata)

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

	// Read return data
	retLen, retData := readEVMReturn(mem)
	switch exitCode {
	case 0:
		fmt.Fprintf(os.Stderr, "EVM: STOP (success)\n")
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
		os.Exit(1)
	default:
		fmt.Fprintf(os.Stderr, "EVM: exit code %d\n", exitCode)
	}
	os.Exit(exitCode)
}

func writeEVMInput(mem *vm.Memory, code, calldata []byte) {
	const addr = evmInputAddr
	binary.LittleEndian.PutUint32(mem.Data[addr:addr+4], uint32(len(code)))
	binary.LittleEndian.PutUint32(mem.Data[addr+4:addr+8], uint32(len(calldata)))
	copy(mem.Data[addr+8:], code)
	copy(mem.Data[addr+8+len(code):], calldata)
}

func readEVMReturn(mem *vm.Memory) (int, []byte) {
	const addr = evmInputAddr
	length := binary.LittleEndian.Uint32(mem.Data[addr : addr+4])
	data := make([]byte, length)
	copy(data, mem.Data[addr+4:addr+4+length])
	return int(length), data
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
