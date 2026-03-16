.section .text
.globl _start

_start:
    # Complexity: Iterative Fibonacci(10) followed by nested jumps.
    # f(0)=0, f(1)=1, ..., f(10)=55.
    
    li t0, 0    # f_n_minus_2
    li t1, 1    # f_n_minus_1
    li t2, 2    # counter
    li t3, 11   # limit (n+1)

fib_loop:
    beq t2, t3, fib_done
    add t4, t0, t1   # f_n = f_n_minus_1 + f_n_minus_2
    add t0, t1, x0   # f_n_minus_2 = f_n_minus_1
    add t1, t4, x0   # f_n_minus_1 = f_n
    addi t2, t2, 1
    jal x0, fib_loop

fib_done:
    # Result should be 55 in t1.
    # Start nested call sequence to test JAL/JALR depth.
    mv a0, t1
    jal ra, layer_1

    # Test U-type instructions.
    lui s0, 0xABCDE
    auipc s1, 0x123

    # Success exit.
    li a7, 93
    li a0, 0
    ecall

layer_1:
    addi sp, sp, -16
    sw ra, 12(sp)
    addi a0, a0, 10  # a0 = 65
    jal ra, layer_2
    addi a0, a0, 5   # a0 = 80
    lw ra, 12(sp)
    addi sp, sp, 16
    jalr x0, ra, 0

layer_2:
    addi a0, a0, 10  # a0 = 75
    jalr x0, ra, 0
