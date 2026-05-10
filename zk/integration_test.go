package zk

import (
	"testing"

	"github.com/VanshSahay/riscvm/vm"
)

// TestFullProgram runs a complete RISC-V program through the VM with tracing,
// then proves every step with the ZK circuit. The program exercises ALU, load,
// store, and branch instructions.
func TestFullProgram(t *testing.T) {
	// Hand-assembled program:
	//   0x00: addi x1, x0, 42        (x1 = 42)
	//   0x04: addi x2, x0, 58        (x2 = 58)
	//   0x08: add x3, x1, x2         (x3 = 100)
	//   0x0C: sw x3, 0(x0)           (mem[0] = 100)
	//   0x10: lw x4, 0(x0)           (x4 = 100)
	//   0x14: beq x4, x3, +8         (if x4==x3, skip next → taken)
	//   0x18: addi x5, x0, 999       (SKIPPED by branch)
	//   0x1C: addi x6, x0, 200       (x6 = 200)
	//   0x20: ebreak                 (halt)
	prog := []uint32{
		0x02A00093, // addi x1, x0, 42
		0x03A00113, // addi x2, x0, 58
		0x002081B3, // add x3, x1, x2
		0x00302023, // sw x3, 0(x0)
		0x00002203, // lw x4, 0(x0)
		0x00320463, // beq x4, x3, +8
		0x3E700293, // addi x5, x0, 999 (skipped)
		0x0C800313, // addi x6, x0, 200
		0x00100073, // ebreak
	}

	mem := vm.NewMemory(16 * 1024 * 1024) // 16 MB
	for i, word := range prog {
		mem.StoreWord(uint32(i*4), word)
	}

	cpu := vm.NewCPU(mem)
	cpu.PC = 0
	trace := vm.Trace{}
	cpu.Trace = &trace

	// Run the program to completion (stops at ebreak)
	exitCode, err := cpu.Run()
	if err != nil {
		t.Fatalf("program execution failed: %v", err)
	}
	if exitCode != -1 {
		t.Fatalf("expected ebreak exit code -1, got %d", exitCode)
	}

	// Verify final state
	if cpu.Regs[1] != 42 {
		t.Errorf("x1 = %d, want 42", cpu.Regs[1])
	}
	if cpu.Regs[2] != 58 {
		t.Errorf("x2 = %d, want 58", cpu.Regs[2])
	}
	if cpu.Regs[3] != 100 {
		t.Errorf("x3 = %d, want 100", cpu.Regs[3])
	}
	if cpu.Regs[4] != 100 {
		t.Errorf("x4 = %d, want 100", cpu.Regs[4])
	}
	// x5 should be 0 (the addi was skipped by the branch)
	if cpu.Regs[5] != 0 {
		t.Errorf("x5 = %d, want 0 (branch should have skipped this)", cpu.Regs[5])
	}
	if cpu.Regs[6] != 200 {
		t.Errorf("x6 = %d, want 200", cpu.Regs[6])
	}

	// Prove every step except the last (ebreak — circuit models PC+=4 but VM exits)
	stepsToProve := len(trace) - 1
	for i := 0; i < stepsToProve; i++ {
		cur := trace[i]
		var nextPC uint32
		var nextRegs [32]uint32
		if i+1 < len(trace) {
			nextPC = trace[i+1].PC
			nextRegs = trace[i+1].Regs
		} else {
			// Should not reach here since we stop before ebreak
			nextPC = cur.PC + 4
			nextRegs = cur.Regs
		}

		w := GenerateWitness(cur, nextPC, nextRegs)
		ok, err := ProveStep(w)
		if err != nil {
			t.Fatalf("step %d (PC=0x%x): prove error: %v", i, cur.PC, err)
		}
		if !ok {
			t.Fatalf("step %d (PC=0x%x): proof verification failed", i, cur.PC)
		}
	}

	t.Logf("proved %d steps of full program execution", stepsToProve)
}
