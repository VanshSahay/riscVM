package vm

import (
	"io"
	"os"
)

type CPU struct {
	PC   uint32
	Regs [32]uint32
	Mem  *Memory
	// Heap break for brk syscall; 0 means not set
	Brk uint32
}

func NewCPU(mem *Memory) *CPU {
	return &CPU{
		PC:  0,
		Mem: mem,
	}
}

func (c *CPU) Step() {
	instr := c.Mem.LoadWord(c.PC)
	decoded := Decode(instr)
	decoded.Execute(c)
	if !decoded.ModifiesPC() {
		c.PC += 4
	}
	c.Regs[0] = 0 // x0 is always zero
}

// Run executes instructions until ECALL exit(93) or EBREAK.
// Returns exit code when program exits, or error.
func (c *CPU) Run() (exitCode int, err error) {
	const maxSteps = 1 << 24 // safety limit
	for i := 0; i < maxSteps; i++ {
		instr := c.Mem.LoadWord(c.PC)
		decoded := Decode(instr)

		// Handle ECALL before Execute
		if _, isEcall := decoded.(Ecall); isEcall {
			code, done, err := c.handleSyscall()
			if err != nil {
				return -1, err
			}
			if done {
				return code, nil
			}
			c.PC += 4
			c.Regs[0] = 0
			continue
		}
		if _, isEbreak := decoded.(Ebreak); isEbreak {
			return -1, nil // debugger trap, treat as exit
		}

		decoded.Execute(c)
		if !decoded.ModifiesPC() {
			c.PC += 4
		}
		c.Regs[0] = 0
	}
	return -1, nil // hit step limit
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
			var written int
			for i := uint32(0); i < n; i++ {
				b := c.Mem.LoadByte(buf + i)
				if fd == 1 {
					os.Stdout.Write([]byte{byte(b)})
				} else {
					os.Stderr.Write([]byte{byte(b)})
				}
				written++
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
