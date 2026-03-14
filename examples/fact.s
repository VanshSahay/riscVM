.section .text
.globl _start

_start:
    # n = 5
    li a0, 5

    # call factorial
    jal ra, factorial

    # store result in memory
    la t0, result
    sw a0, 0(t0)

    # write "OK\n"
    li a7, 64
    li a0, 1
    la a1, msg
    li a2, 3
    ecall

    # exit
    li a7, 93
    li a0, 0
    ecall


# factorial(n)
# a0 = n
# return a0 = n!
factorial:
    addi sp, sp, -16
    sw ra, 12(sp)
    sw s0, 8(sp)
    sw s1, 4(sp)

    mv s0, a0       # s0 = n
    li s1, 1        # result = 1

fact_loop:
    beq s0, x0, fact_done

    mv a0, s1       # a0 = result
    mv a1, s0       # a1 = n
    jal ra, multiply

    mv s1, a0       # result = result * n
    addi s0, s0, -1
    jal x0, fact_loop

fact_done:
    mv a0, s1

    lw ra, 12(sp)
    lw s0, 8(sp)
    lw s1, 4(sp)
    addi sp, sp, 16
    jalr x0, ra, 0


# multiply(a,b)
# a0=a , a1=b
# return a0=a*b
multiply:
    li t0, 0

mul_loop:
    beq a1, x0, mul_done
    add t0, t0, a0
    addi a1, a1, -1
    jal x0, mul_loop

mul_done:
    mv a0, t0
    jalr x0, ra, 0


.section .data
result: .word 0

.section .rodata
msg: .asciz "OK\n"




