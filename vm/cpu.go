package vm

import (
	"errors"
	"fmt"
	"io"
	"os"
)

var ErrStepLimit = errors.New("step limit exceeded")

type CPU struct {
	PC       uint32
	Regs     [32]uint32
	Mem      *Memory
	Brk      uint32
	LastPC   uint32 // address of last executed instruction (for UI/disassembly)
	Exited   bool   // true after ecall exit
	ExitCode int
	// Stdout, when set (e.g. by WASM), receives write(1, ...) output instead of os.Stdout
	Stdout io.Writer
	Trace  *Trace
}

func NewCPU(mem *Memory) *CPU {
	return &CPU{
		PC:  0,
		Mem: mem,
	}
}

func (c *CPU) Step() error {
	c.LastPC = c.PC
	c.Mem.LastErr = nil
	instr := c.Mem.LoadWord(c.PC)
	if c.Mem.LastErr != nil {
		return fmt.Errorf("instruction fetch at 0x%x: %w", c.PC, c.Mem.LastErr)
	}
	if c.Trace != nil {
		step := TraceStep{
			PC:    c.PC,
			Instr: instr,
			Regs:  c.Regs,
		}
		*c.Trace = append(*c.Trace, step)
		// Temporarily set OnAccess to capture memory op for this instruction
		c.Mem.OnAccess = func(addr uint32, value uint32, op int) {
			idx := len(*c.Trace) - 1
			(*c.Trace)[idx].MemAddr = addr
			(*c.Trace)[idx].MemVal = value
			(*c.Trace)[idx].MemOp = op
		}
		defer func() { c.Mem.OnAccess = nil }()
	}
	decoded := Decode(instr)
	if _, isUnknown := decoded.(Unknown); isUnknown {
		return fmt.Errorf("illegal instruction 0x%08x at PC=0x%x", instr, c.PC)
	}
	// Handle ECALL
	if _, isEcall := decoded.(Ecall); isEcall {
		code, done, _ := c.handleSyscall()
		if done {
			c.Exited = true
			c.ExitCode = code
		} else {
			c.PC += 4
		}
		c.Regs[0] = 0
		return nil
	}
	if _, isEbreak := decoded.(Ebreak); isEbreak {
		c.Exited = true
		c.ExitCode = -1
		c.Regs[0] = 0
		return nil
	}
	c.Mem.LastErr = nil
	decoded.Execute(c)
	if c.Mem.LastErr != nil {
		return fmt.Errorf("memory fault at PC=0x%x: %w", c.LastPC, c.Mem.LastErr)
	}
	if !decoded.ModifiesPC() {
		c.PC += 4
	}
	c.Regs[0] = 0
	return nil
}

// Run executes instructions until ECALL exit(93) or EBREAK.
// Returns exit code when program exits, or error.
func (c *CPU) Run() (exitCode int, err error) {
	const maxSteps = 1 << 24 // safety limit
	for i := 0; i < maxSteps; i++ {
		if err := c.Step(); err != nil {
			return -1, err
		}
		if c.Exited {
			return c.ExitCode, nil
		}
	}
	return -1, ErrStepLimit
}

// RISC-V Linux ABI: a7=syscall#, a0-a5=args, a0=ret
func (c *CPU) handleSyscall() (exitCode int, done bool, err error) {
	a7 := c.Regs[17]
	a0 := c.Regs[10]
	a1 := c.Regs[11]
	a2 := c.Regs[12]

	switch a7 {
	case 93: // exit
		return int(int32(a0)), true, nil
	case 64: // write(fd, buf, len)
		fd := int(a0)
		buf := a1
		n := a2
		if fd == 1 || fd == 2 {
			out := c.Stdout
			if out == nil {
				if fd == 1 {
					out = os.Stdout
				} else {
					out = os.Stderr
				}
			}
			var written int
			for i := uint32(0); i < n; i++ {
				b := c.Mem.LoadByte(buf + i)
				if _, err := out.Write([]byte{byte(b)}); err == nil {
					written++
				}
			}
			c.Regs[10] = uint32(written)
		} else {
			c.Regs[10] = ^uint32(0) // -1 EBADF
		}
	case 63: // read(fd, buf, len)
		fd := int(a0)
		buf := a1
		n := a2
		if fd == 0 {
			var read int
			for i := uint32(0); i < n; i++ {
				b := make([]byte, 1)
				if _, e := io.ReadFull(os.Stdin, b); e != nil {
					break
				}
				c.Mem.StoreByte(buf+i, uint32(b[0]))
				read++
			}
			c.Regs[10] = uint32(read)
		} else {
			c.Regs[10] = ^uint32(0)
		}
	case 214: // brk(addr)
		if c.Brk == 0 {
			c.Brk = 0x10000 // default heap start
		}
		if a0 == 0 {
			c.Regs[10] = c.Brk
		} else {
			c.Brk = a0
			c.Regs[10] = a0
		}
	default:
		c.Regs[10] = ^uint32(0) // -1 ENOSYS
	}
	return 0, false, nil
}
