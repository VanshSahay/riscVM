MEMORY
{
    RAM (rwx) : ORIGIN = 0x10000, LENGTH = 15M
}

ENTRY(_start)

SECTIONS
{
    .text : {
        *(.text._start)
        *(.text*)
    } > RAM

    .rodata : {
        *(.rodata*)
        *(.srodata*)
    } > RAM

    .data : {
        *(.data*)
        *(.sdata*)
    } > RAM

    .bss : {
        _bss_start = .;
        *(.bss*)
        *(.sbss*)
        _bss_end = .;
    } > RAM

    . = ALIGN(8);
    _stack_start = ORIGIN(RAM) + LENGTH(RAM);
    _sheap = .;
}
