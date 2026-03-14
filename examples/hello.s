.section .text
.globl _start
_start:
    # write(1, msg, 13)  -- a7=64, a0=1, a1=msg, a2=13
    li   a7, 64
    li   a0, 1
    la   a1, msg
    li   a2, 13
    ecall

    # exit(0)  -- a7=93, a0=0
    li   a7, 93
    li   a0, 0
    ecall

.section .rodata
msg: .asciz "Hello, RISC-V!\n"
