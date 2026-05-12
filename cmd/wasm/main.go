//go:build js && wasm

package main

import (
	_ "embed"
	"encoding/binary"
	"fmt"
	"syscall/js"

	"github.com/VanshSahay/riscvm/vm"
	"github.com/VanshSahay/riscvm/zk"
)

//go:embed evm.elf
var evmELF []byte

const evmInputAddr = 0x800000

var (
	cpu *vm.CPU
	mem *vm.Memory
)

func main() {
	js.Global().Set("riscvmLoadProgram", js.FuncOf(loadProgram))
	js.Global().Set("riscvmLoadEVM", js.FuncOf(loadEVM))
	js.Global().Set("riscvmStep", js.FuncOf(step))
	js.Global().Set("riscvmGetPC", js.FuncOf(getPC))
	js.Global().Set("riscvmGetRegs", js.FuncOf(getRegs))
	js.Global().Set("riscvmGetMemory", js.FuncOf(getMemory))
	js.Global().Set("riscvmGetLastInstruction", js.FuncOf(getLastInstruction))
	js.Global().Set("riscvmGetExited", js.FuncOf(getExited))
	js.Global().Set("riscvmGetExitCode", js.FuncOf(getExitCode))
	js.Global().Set("riscvmVerifyLastStep", js.FuncOf(verifyLastStep))
	js.Global().Set("riscvmRunToExit", js.FuncOf(runToExit))
	js.Global().Set("riscvmWarmup", js.FuncOf(warmup))
	<-make(chan struct{})
}

func verifyLastStep(this js.Value, args []js.Value) interface{} {
	if cpu == nil || cpu.Trace == nil || len(*cpu.Trace) == 0 {
		return map[string]interface{}{"ok": false, "error": "no trace"}
	}
	trace := *cpu.Trace
	currentStep := trace[len(trace)-1]
	witness := zk.GenerateWitness(currentStep, cpu.PC, cpu.Regs)

	ok, err := zk.ProveStep(witness)
	if err != nil {
		return map[string]interface{}{"ok": false, "error": err.Error()}
	}

	diffs := make(map[string]interface{})
	for i := 0; i < 32; i++ {
		if currentStep.Regs[i] != cpu.Regs[i] {
			diffs[fmt.Sprintf("x%d", i)] = map[string]interface{}{
				"from": fmt.Sprintf("0x%08x", currentStep.Regs[i]),
				"to":   fmt.Sprintf("0x%08x", cpu.Regs[i]),
			}
		}
	}

	result := map[string]interface{}{
		"ok": ok,
		"witness": map[string]interface{}{
			"pcBefore": fmt.Sprintf("0x%08x", currentStep.PC),
			"pcAfter":  fmt.Sprintf("0x%08x", cpu.PC),
			"instr":    fmt.Sprintf("0x%08x", currentStep.Instr),
			"asm":      vm.FormatInstruction(currentStep.Instr),
			"diffs":    diffs,
		},
	}
	if currentStep.MemOp != 0 {
		w := result["witness"].(map[string]interface{})
		w["memAddr"] = fmt.Sprintf("0x%08x", currentStep.MemAddr)
		w["memVal"] = fmt.Sprintf("0x%08x", currentStep.MemVal)
		w["memOp"] = map[int]string{1: "read", 2: "write"}[currentStep.MemOp]
	}
	return result
}

func getExited(this js.Value, args []js.Value) interface{} {
	return cpu != nil && cpu.Exited
}

func getExitCode(this js.Value, args []js.Value) interface{} {
	if cpu == nil {
		return 0
	}
	return cpu.ExitCode
}

func loadProgram(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return map[string]interface{}{"ok": false, "error": "need bytes"}
	}
	buf := args[0]
	if buf.Type() != js.TypeObject {
		return map[string]interface{}{"ok": false, "error": "bytes must be Uint8Array"}
	}
	length := buf.Get("length").Int()
	data := make([]byte, length)
	js.CopyBytesToGo(data, buf)
	mem = vm.NewMemory(0)
	asELF := true
	if len(args) >= 2 && args[1].Type() == js.TypeBoolean {
		asELF = args[1].Bool()
	}
	var entry uint32
	if asELF {
		var err error
		entry, err = vm.LoadELFBytes(data, mem)
		if err != nil {
			return map[string]interface{}{"ok": false, "error": err.Error()}
		}
	} else {
		entry = vm.LoadRaw(data, mem, 0)
	}
	cpu = vm.NewCPU(mem)
	cpu.Trace = &vm.Trace{}
	cpu.Stdout = jsOutputWriter{}
	cpu.PC = entry
	cpu.Regs[2] = uint32(len(mem.Data))
	return map[string]interface{}{"ok": true, "entry": entry}
}

func loadEVM(this js.Value, args []js.Value) interface{} {
	// args[0]: EVM bytecode (Uint8Array)
	// args[1]: optional calldata (Uint8Array)
	if len(args) < 1 || args[0].Type() != js.TypeObject {
		return map[string]interface{}{"ok": false, "error": "need EVM bytecode Uint8Array"}
	}
	codeLen := args[0].Get("length").Int()
	code := make([]byte, codeLen)
	js.CopyBytesToGo(code, args[0])

	var calldata []byte
	if len(args) >= 2 && args[1].Type() == js.TypeObject {
		cdLen := args[1].Get("length").Int()
		calldata = make([]byte, cdLen)
		js.CopyBytesToGo(calldata, args[1])
	}

	mem = vm.NewMemory(0)
	entry, err := vm.LoadELFBytes(evmELF, mem)
	if err != nil {
		return map[string]interface{}{"ok": false, "error": "failed to load EVM interpreter: " + err.Error()}
	}

	// Write EVM input at 0x800000
	writeEVMInput(mem, code, calldata)

	cpu = vm.NewCPU(mem)
	cpu.Trace = &vm.Trace{}
	cpu.Stdout = jsOutputWriter{}
	cpu.PC = entry
	cpu.Regs[2] = uint32(len(mem.Data))
	return map[string]interface{}{"ok": true, "entry": entry}
}

func writeEVMInput(mem *vm.Memory, code, calldata []byte) {
	const addr = evmInputAddr
	binary.LittleEndian.PutUint32(mem.Data[addr:addr+4], uint32(len(code)))
	binary.LittleEndian.PutUint32(mem.Data[addr+4:addr+8], uint32(len(calldata)))
	binary.LittleEndian.PutUint32(mem.Data[addr+8:addr+12], 0) // no storage init
	copy(mem.Data[addr+12:], code)
	copy(mem.Data[addr+12+len(code):], calldata)
}

// jsOutputWriter sends write(1, ...) output to JS via window.riscvmAppendOutput(Uint8Array).
type jsOutputWriter struct{}

func (jsOutputWriter) Write(p []byte) (n int, err error) {
	cb := js.Global().Get("riscvmAppendOutput")
	if !cb.Truthy() {
		return len(p), nil
	}
	dst := js.Global().Get("Uint8Array").New(len(p))
	js.CopyBytesToJS(dst, p)
	cb.Invoke(dst)
	return len(p), nil
}

func step(this js.Value, args []js.Value) interface{} {
	if cpu == nil {
		return map[string]interface{}{"ok": false, "error": "no program loaded"}
	}
	if err := cpu.Step(); err != nil {
		return map[string]interface{}{"ok": false, "error": err.Error()}
	}
	return map[string]interface{}{"ok": true}
}

func getPC(this js.Value, args []js.Value) interface{} {
	if cpu == nil {
		return 0
	}
	return cpu.PC
}

func getRegs(this js.Value, args []js.Value) interface{} {
	if cpu == nil {
		return js.ValueOf([]interface{}{})
	}
	out := make([]interface{}, 32)
	for i := 0; i < 32; i++ {
		out[i] = cpu.Regs[i]
	}
	return js.ValueOf(out)
}

func getMemory(this js.Value, args []js.Value) interface{} {
	if mem == nil {
		return js.ValueOf(nil)
	}
	offset := 0
	length := len(mem.Data)
	if len(args) >= 1 {
		offset = args[0].Int()
	}
	if len(args) >= 2 {
		length = args[1].Int()
	}
	if offset < 0 || offset >= len(mem.Data) {
		return js.ValueOf(nil)
	}
	if offset+length > len(mem.Data) {
		length = len(mem.Data) - offset
	}
	dst := js.Global().Get("Uint8Array").New(length)
	js.CopyBytesToJS(dst, mem.Data[offset:offset+length])
	return dst
}

func getLastInstruction(this js.Value, args []js.Value) interface{} {
	if cpu == nil || mem == nil {
		return ""
	}
	instr := mem.LoadWord(cpu.LastPC)
	return vm.FormatInstruction(instr)
}

func runToExit(this js.Value, args []js.Value) interface{} {
	// Run a batch of steps without tracing/proving, yielding control
	// so the browser stays responsive. Returns whether the program exited.
	if cpu == nil {
		return map[string]interface{}{"ok": false, "error": "no program"}
	}
	batch := 50000
	if len(args) > 0 {
		batch = args[0].Int()
	}
	for i := 0; i < batch; i++ {
		if err := cpu.Step(); err != nil {
			return map[string]interface{}{"ok": false, "error": err.Error()}
		}
		if cpu.Exited {
			return map[string]interface{}{"ok": true, "exited": true, "exitCode": cpu.ExitCode}
		}
	}
	return map[string]interface{}{"ok": true, "exited": false}
}

func warmup(this js.Value, args []js.Value) interface{} {
	if cpu == nil || cpu.Trace == nil || len(*cpu.Trace) == 0 {
		// No trace yet — do one step to get a valid witness, then verify.
		if cpu == nil {
			return map[string]interface{}{"ok": false, "error": "no program"}
		}
		cpu.Step()
	}
	trace := *cpu.Trace
	currentStep := trace[len(trace)-1]
	w := zk.GenerateWitness(currentStep, cpu.PC, cpu.Regs)
	_, _ = zk.ProveStep(w)
	return map[string]interface{}{"ok": true}
}
