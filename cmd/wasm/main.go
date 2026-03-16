//go:build js && wasm

package main

import (
	"fmt"
	"syscall/js"

	"github.com/VanshSahay/riscvm/vm"
	"github.com/VanshSahay/riscvm/zk"
)

var (
	cpu *vm.CPU
	mem *vm.Memory
)

func main() {
	js.Global().Set("riscvmLoadProgram", js.FuncOf(loadProgram))
	js.Global().Set("riscvmStep", js.FuncOf(step))
	js.Global().Set("riscvmGetPC", js.FuncOf(getPC))
	js.Global().Set("riscvmGetRegs", js.FuncOf(getRegs))
	js.Global().Set("riscvmGetMemory", js.FuncOf(getMemory))
	js.Global().Set("riscvmGetLastInstruction", js.FuncOf(getLastInstruction))
	js.Global().Set("riscvmGetExited", js.FuncOf(getExited))
	js.Global().Set("riscvmGetExitCode", js.FuncOf(getExitCode))
	js.Global().Set("riscvmVerifyLastStep", js.FuncOf(verifyLastStep))
	<-make(chan struct{})
}

func verifyLastStep(this js.Value, args []js.Value) interface{} {
	if cpu == nil || cpu.Trace == nil || len(*cpu.Trace) == 0 {
		return map[string]interface{}{"ok": false, "error": "no trace"}
	}
	trace := *cpu.Trace
	currentStep := trace[len(trace)-1]
	witness := zk.GenerateWitness(currentStep, cpu.PC, cpu.Regs)

	// In a real production zkVM, this is where we'd call gnark's groth16.Prove
	// For this sleek WASM demo, we verify the witness and return a success signal.
	ok, err := zk.ProveStep(witness)
	if err != nil {
		return map[string]interface{}{"ok": false, "error": err.Error()}
	}

	// Find register diffs
	diffs := make(map[string]interface{})
	for i := 0; i < 32; i++ {
		if currentStep.Regs[i] != cpu.Regs[i] {
			diffs[fmt.Sprintf("x%d", i)] = map[string]interface{}{
				"from": fmt.Sprintf("0x%08x", currentStep.Regs[i]),
				"to":   fmt.Sprintf("0x%08x", cpu.Regs[i]),
			}
		}
	}

	return map[string]interface{}{
		"ok": ok,
		"witness": map[string]interface{}{
			"pcBefore": fmt.Sprintf("0x%08x", currentStep.PC),
			"pcAfter":  fmt.Sprintf("0x%08x", cpu.PC),
			"instr":    fmt.Sprintf("0x%08x", currentStep.Instr),
			"asm":      vm.FormatInstruction(currentStep.Instr),
			"diffs":    diffs,
		},
	}
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
	cpu.Step()
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
