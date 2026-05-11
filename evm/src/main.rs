#![no_std]
#![no_main]

mod u256;

use u256::*;

// zkVM copies EVM bytecode + calldata + optional storage init to 0x800000:
//   [u32 LE: code_len][u32 LE: calldata_len][u32 LE: has_storage_init (0|1)]
//   [code...][calldata...]
//   [if has_storage_init: STORAGE_SLOTS*32 bytes of storage data]
// On exit, output is written to 0x800000:
//   [u32 LE: return_data_len][return data...][STORAGE_SLOTS*32 bytes final storage]
const INPUT_ADDR: usize = 0x800000;
const OUTPUT_ADDR: usize = 0x800000;

const MAX_STACK: usize = 1024;
const MEM_SIZE: usize = 65536; // 64 kB fixed memory
const STORAGE_SLOTS: usize = 256;

const STOP: u8 = 0x00;
const ADD: u8 = 0x01;
const MUL: u8 = 0x02;
const SUB: u8 = 0x03;
const DIV: u8 = 0x04;
const SDIV: u8 = 0x05;
const MOD: u8 = 0x06;
const SMOD: u8 = 0x07;
const ADDMOD: u8 = 0x08;
const MULMOD: u8 = 0x09;
const EXP: u8 = 0x0A;
const SIGNEXTEND: u8 = 0x0B;
const LT: u8 = 0x10;
const GT: u8 = 0x11;
const SLT: u8 = 0x12;
const SGT: u8 = 0x13;
const EQ: u8 = 0x14;
const ISZERO: u8 = 0x15;
const AND: u8 = 0x16;
const OR: u8 = 0x17;
const XOR: u8 = 0x18;
const NOT: u8 = 0x19;
const BYTE: u8 = 0x1A;
const SHL: u8 = 0x1B;
const SHR: u8 = 0x1C;
const SAR: u8 = 0x1D;
const SHA3: u8 = 0x20;
const CALLDATALOAD: u8 = 0x35;
const CALLDATASIZE: u8 = 0x36;
const CALLDATACOPY: u8 = 0x37;
const CODECOPY: u8 = 0x39;
const POP: u8 = 0x50;
const MLOAD: u8 = 0x51;
const MSTORE: u8 = 0x52;
const MSTORE8: u8 = 0x53;
const SLOAD: u8 = 0x54;
const SSTORE: u8 = 0x55;
const JUMP: u8 = 0x56;
const JUMPI: u8 = 0x57;
const PC: u8 = 0x58;
const MSIZE: u8 = 0x59;
const JUMPDEST: u8 = 0x5B;
const PUSH1: u8 = 0x60;
// PUSH2..PUSH32 follow consecutively
const DUP1: u8 = 0x80;
// DUP2..DUP16 follow consecutively
const SWAP1: u8 = 0x90;
// SWAP2..SWAP16 follow consecutively
const RETURN: u8 = 0xF3;
const REVERT: u8 = 0xFD;

struct Evm<'a> {
    pc: usize,
    stack: [U256; MAX_STACK],
    sp: usize,
    memory: [u8; MEM_SIZE],
    ms: usize, // active memory size (for MSIZE)
    code: &'a [u8],
    calldata: &'a [u8],
    storage: [U256; STORAGE_SLOTS],
    stopped: bool,
    reverted: bool,
}

impl<'a> Evm<'a> {
    fn new(code: &'a [u8], calldata: &'a [u8], storage_init: Option<&[u32; STORAGE_SLOTS * 8]>) -> Self {
        let mut storage = [[0u32; 8]; STORAGE_SLOTS];
        if let Some(init) = storage_init {
            for i in 0..STORAGE_SLOTS {
                for j in 0..8 {
                    storage[i][j] = init[i * 8 + j];
                }
            }
        }
        Evm {
            pc: 0,
            stack: [[0u32; 8]; MAX_STACK],
            sp: 0,
            memory: [0u8; MEM_SIZE],
            ms: 0,
            code,
            calldata,
            storage,
            stopped: false,
            reverted: false,
        }
    }

    // ── stack helpers ────────────────────────────────────────────
    fn push(&mut self, v: U256) {
        if self.sp < MAX_STACK {
            self.stack[self.sp] = v;
            self.sp += 1;
        } else {
            self.revert();
        }
    }

    fn pop(&mut self) -> U256 {
        if self.sp == 0 {
            self.revert();
            return zero();
        }
        self.sp -= 1;
        self.stack[self.sp]
    }

    fn dup(&mut self, n: usize) {
        if self.sp >= n && self.sp > 0 {
            let v = self.stack[self.sp - n];
            self.push(v);
        } else {
            self.revert();
        }
    }

    fn swap(&mut self, n: usize) {
        if self.sp > n {
            let top = self.stack[self.sp - 1];
            self.stack[self.sp - 1] = self.stack[self.sp - 1 - n];
            self.stack[self.sp - 1 - n] = top;
        } else {
            self.revert();
        }
    }

    // ── memory ───────────────────────────────────────────────────
    fn mem_read(&self, offset: usize, size: usize) -> U256 {
        let mut buf = [0u8; 32];
        for i in 0..size {
            let addr = offset + i;
            buf[i] = if addr < MEM_SIZE { self.memory[addr] } else { 0 };
        }
        from_bytes_be(&buf[..size])
    }

    fn mem_write(&mut self, offset: usize, size: usize, val: &U256) {
        self.grow_memory(offset + size);
        let mut bytes = [0u8; 32];
        to_bytes_be(val, &mut bytes);
        for i in 0..size {
            let addr = offset + i;
            if addr < MEM_SIZE {
                self.memory[addr] = bytes[32 - size + i];
            }
        }
    }

    fn mem_write_byte(&mut self, offset: usize, val: &U256) {
        self.grow_memory(offset + 1);
        if offset < MEM_SIZE {
            self.memory[offset] = val[0] as u8;
        }
    }

    fn grow_memory(&mut self, needed: usize) {
        if needed > self.ms {
            self.ms = (needed + 31) & !31; // round up to 32-byte word
            if self.ms > MEM_SIZE {
                self.ms = MEM_SIZE;
            }
        }
    }

    // ── storage (simplified: key modulo 256, low byte of key) ────
    fn sstore(&mut self, key: &U256, val: &U256) {
        let idx = (key[0] as usize) % STORAGE_SLOTS;
        self.storage[idx] = *val;
    }

    fn sload(&self, key: &U256) -> U256 {
        let idx = (key[0] as usize) % STORAGE_SLOTS;
        self.storage[idx]
    }

    fn revert(&mut self) {
        self.stopped = true;
        self.reverted = true;
    }

    // ── instruction dispatch ─────────────────────────────────────
    fn step(&mut self) {
        if self.stopped {
            return;
        }
        if self.pc >= self.code.len() {
            self.stopped = true;
            return;
        }
        let op = self.code[self.pc];
        self.pc += 1;

        match op {
            STOP => {
                self.stopped = true;
            }

            // Arithmetic
            ADD => {
                let a = self.pop();
                let b = self.pop();
                self.push(add(&a, &b));
            }
            MUL => {
                let a = self.pop();
                let b = self.pop();
                self.push(mul(&a, &b));
            }
            SUB => {
                let a = self.pop();
                let b = self.pop();
                self.push(sub(&a, &b));
            }
            DIV => {
                let a = self.pop();
                let b = self.pop();
                self.push(div(&a, &b));
            }
            SDIV => {
                let a = self.pop();
                let b = self.pop();
                self.push(sdiv(&a, &b));
            }
            MOD => {
                let a = self.pop();
                let b = self.pop();
                self.push(rem(&a, &b));
            }
            SMOD => {
                let a = self.pop();
                let b = self.pop();
                self.push(srem(&a, &b));
            }
            ADDMOD => {
                let a = self.pop();
                let b = self.pop();
                let m = self.pop();
                let s = add(&a, &b);
                if is_zero(&m) {
                    self.push(zero());
                } else {
                    self.push(rem(&s, &m));
                }
            }
            MULMOD => {
                let a = self.pop();
                let b = self.pop();
                let m = self.pop();
                self.push(mulmod(&a, &b, &m));
            }
            EXP => {
                let base = self.pop();
                let exp = self.pop();
                self.push(exp_op(&base, &exp));
            }
            SIGNEXTEND => {
                let n = self.pop();
                let a = self.pop();
                self.push(signextend(&a, &n));
            }

            // Comparisons
            LT => {
                let a = self.pop();
                let b = self.pop();
                self.push(if lt(&a, &b) { one() } else { zero() });
            }
            GT => {
                let a = self.pop();
                let b = self.pop();
                self.push(if gt(&a, &b) { one() } else { zero() });
            }
            SLT => {
                let a = self.pop();
                let b = self.pop();
                self.push(if slt(&a, &b) { one() } else { zero() });
            }
            SGT => {
                let a = self.pop();
                let b = self.pop();
                self.push(if sgt(&a, &b) { one() } else { zero() });
            }
            EQ => {
                let a = self.pop();
                let b = self.pop();
                self.push(if eq(&a, &b) { one() } else { zero() });
            }
            ISZERO => {
                let a = self.pop();
                self.push(if is_zero(&a) { one() } else { zero() });
            }

            // Bitwise
            AND => {
                let a = self.pop();
                let b = self.pop();
                self.push(and(&a, &b));
            }
            OR => {
                let a = self.pop();
                let b = self.pop();
                self.push(or(&a, &b));
            }
            XOR => {
                let a = self.pop();
                let b = self.pop();
                self.push(xor(&a, &b));
            }
            NOT => {
                let a = self.pop();
                self.push(not(&a));
            }
            BYTE => {
                let n = self.pop();
                let a = self.pop();
                self.push(byte(&a, &n));
            }
            SHL => {
                let shift = self.pop();
                let a = self.pop();
                self.push(shl(&a, &shift));
            }
            SHR => {
                let shift = self.pop();
                let a = self.pop();
                self.push(shr(&a, &shift));
            }
            SAR => {
                let shift = self.pop();
                let a = self.pop();
                self.push(sar(&a, &shift));
            }

            // SHA3: simplified — just hash the word itself (identity-like for demo)
            SHA3 => {
                let offset = self.pop();
                let size = self.pop();
                let mut h = zero();
                h[0] = offset[0] ^ size[0];
                self.push(h);
            }

            // Environment opcodes
            0x30 => { // ADDRESS — return self address
                self.push(zero());
            }
            0x33 => { // CALLER — always return address 0x01 for our test sender
                self.push([1u32, 0, 0, 0, 0, 0, 0, 0]);
            }
            0x34 => { // CALLVALUE
                self.push(zero());
            }
            0x32 => { // ORIGIN
                self.push([1u32, 0, 0, 0, 0, 0, 0, 0]);
            }
            0x31 => { // BALANCE — return 0
                self.pop(); // ignore address
                self.push(zero());
            }

            // Calldata
            CALLDATALOAD => {
                let offset_u = self.pop();
                let offset = offset_u[0] as usize;
                let mut buf = [0u8; 32];
                for i in 0..32 {
                    let pos = offset + i;
                    buf[i] = if pos < self.calldata.len() {
                        self.calldata[pos]
                    } else {
                        0
                    };
                }
                self.push(from_bytes_be(&buf));
            }
            CALLDATASIZE => {
                let sz = self.calldata.len() as u32;
                self.push([sz, 0, 0, 0, 0, 0, 0, 0]);
            }
            CALLDATACOPY => {
                let mem_dst = self.pop();
                let cd_src = self.pop();
                let size = self.pop();
                let dst = mem_dst[0] as usize;
                let src = cd_src[0] as usize;
                let n = size[0] as usize;
                for i in 0..n {
                    let b = if src + i < self.calldata.len() {
                        self.calldata[src + i]
                    } else {
                        0
                    };
                    if dst + i < MEM_SIZE {
                        self.memory[dst + i] = b;
                    }
                }
                if dst + n > self.ms {
                    self.grow_memory(dst + n);
                }
            }
            CODECOPY => {
                let mem_dst = self.pop();
                let code_src = self.pop();
                let size = self.pop();
                let dst = mem_dst[0] as usize;
                let src = code_src[0] as usize;
                let n = size[0] as usize;
                for i in 0..n {
                    let b = if src + i < self.code.len() {
                        self.code[src + i]
                    } else {
                        0
                    };
                    if dst + i < MEM_SIZE {
                        self.memory[dst + i] = b;
                    }
                }
                if dst + n > self.ms {
                    self.grow_memory(dst + n);
                }
            }

            // More environment / code opcodes
            0x38 => { // CODESIZE
                self.push([self.code.len() as u32, 0, 0, 0, 0, 0, 0, 0]);
            }
            0x3D => { // RETURNDATASIZE — no prior calls in our context
                self.push(zero());
            }
            0x3E => { // RETURNDATACOPY — pop args and ignore
                self.pop(); self.pop(); self.pop();
            }

            // Stack manipulation
            POP => {
                self.pop();
            }
            MLOAD => {
                let offset = self.pop();
                let v = self.mem_read(offset[0] as usize, 32);
                self.push(v);
            }
            MSTORE => {
                let offset = self.pop();
                let val = self.pop();
                self.mem_write(offset[0] as usize, 32, &val);
            }
            MSTORE8 => {
                let offset = self.pop();
                let val = self.pop();
                self.mem_write_byte(offset[0] as usize, &val);
            }

            // Storage
            SLOAD => {
                let key = self.pop();
                let v = self.sload(&key);
                self.push(v);
            }
            SSTORE => {
                let key = self.pop();
                let val = self.pop();
                self.sstore(&key, &val);
            }

            // Control flow
            JUMP => {
                let dst = self.pop();
                self.pc = dst[0] as usize;
            }
            JUMPI => {
                let dst = self.pop();
                let cond = self.pop();
                if !is_zero(&cond) {
                    self.pc = dst[0] as usize;
                }
            }
            PC => {
                self.push([(self.pc - 1) as u32, 0, 0, 0, 0, 0, 0, 0]);
            }
            MSIZE => {
                self.push([self.ms as u32, 0, 0, 0, 0, 0, 0, 0]);
            }
            JUMPDEST => {
                // NOP — valid jump target marker
            }

            // Push
            n if (PUSH1..=0x7F).contains(&n) => {
                let count = (n - PUSH1 + 1) as usize;
                let mut bytes = [0u8; 32];
                for i in 0..count {
                    bytes[32 - count + i] = if self.pc + i < self.code.len() {
                        self.code[self.pc + i]
                    } else {
                        0
                    };
                }
                self.pc += count;
                self.push(from_bytes_be(&bytes[..]));
            }

            // Dup
            n if (DUP1..=0x8F).contains(&n) => {
                let depth = (n - DUP1 + 1) as usize;
                self.dup(depth);
            }

            // Swap
            n if (SWAP1..=0x9F).contains(&n) => {
                let depth = (n - SWAP1 + 1) as usize;
                self.swap(depth);
            }

            // Return / Revert
            RETURN => {
                let offset = self.pop();
                let size = self.pop();
                let start = offset[0] as usize;
                let n = size[0] as usize;
                self.write_return_data(start, n);
                self.stopped = true;
            }
            REVERT => {
                let offset = self.pop();
                let size = self.pop();
                let start = offset[0] as usize;
                let n = size[0] as usize;
                self.write_return_data(start, n);
                self.stopped = true;
                self.reverted = true;
            }

            // Gas — return a large constant
            0x5A => { // GAS
                self.push([0xFFFF_FFFFu32, 0xFFFF_FFFFu32, 0, 0, 0, 0, 0, 0]);
            }

            // Block environment — return 0
            0x41 => { self.push(zero()); } // COINBASE
            0x42 => { self.push(zero()); } // TIMESTAMP
            0x43 => { self.push(zero()); } // NUMBER
            0x44 => { self.push(zero()); } // DIFFICULTY / PREVRANDAO
            0x45 => { self.push(zero()); } // GASLIMIT
            0x46 => { self.push(zero()); } // CHAINID

            // Logs — pop args and ignore (no-op)
            0xA0..=0xA4 => { // LOG0-LOG4
                self.pop(); // offset
                self.pop(); // size
            }

            _ => {
                // Unknown opcode — revert
                self.revert();
            }
        }
    }

    fn write_return_data(&self, mem_start: usize, len: usize) {
        let ptr = OUTPUT_ADDR as *mut u8;
        unsafe {
            // Write length as LE u32
            let len32 = len as u32;
            *ptr = len32 as u8;
            *ptr.add(1) = (len32 >> 8) as u8;
            *ptr.add(2) = (len32 >> 16) as u8;
            *ptr.add(3) = (len32 >> 24) as u8;
            // Write data
            for i in 0..len {
                let b = if mem_start + i < MEM_SIZE {
                    self.memory[mem_start + i]
                } else {
                    0
                };
                *ptr.add(4 + i) = b;
            }
        }
    }

    fn write_final_storage(&self) {
        // Write all STORAGE_SLOTS * 32 bytes after the return data.
        // First we need to find where the return data ends.
        let base = OUTPUT_ADDR as *const u8;
        let ret_len = unsafe { (base as *const u32).read_volatile() } as usize;
        let storage_out = unsafe { base.add(4 + ret_len) as *mut u32 };
        for i in 0..STORAGE_SLOTS {
            unsafe {
                let off = i * 8;
                let p = storage_out.add(off);
                for j in 0..8 {
                    *p.add(j) = self.storage[i][j];
                }
            }
        }
    }

    fn run(&mut self) -> i32 {
        let max_steps = 1_000_000;
        for _ in 0..max_steps {
            if self.stopped {
                break;
            }
            self.step();
        }
        if self.reverted {
            2
        } else if self.stopped {
            0
        } else {
            3 // ran out of steps
        }
    }
}

core::arch::global_asm!(
    ".section .text._start",
    ".globl _start",
    "_start:",
    "la sp, _stack_start",
    "jal main",
    "unimp",
);

extern "C" {
    static _stack_start: u32;
    static _bss_start: u8;
    static _bss_end: u8;
}

#[panic_handler]
fn panic(_: &core::panic::PanicInfo) -> ! {
    unsafe {
        core::arch::asm!("ecall", in("a7") 93, in("a0") 99u32);
    }
    loop {}
}

#[no_mangle]
pub extern "C" fn main() -> ! {
    unsafe {
        let bss_start = &_bss_start as *const u8 as usize;
        let bss_end = &_bss_end as *const u8 as usize;
        let len = bss_end - bss_start;
        core::ptr::write_bytes(bss_start as *mut u8, 0, len);
    }

    let (code, calldata, storage_init) = unsafe { read_input() };

    let mut evm = Evm::new(code, calldata, storage_init);
    let exit_code = evm.run();

    evm.write_final_storage();

    unsafe {
        core::arch::asm!("ecall", in("a7") 93, in("a0") exit_code);
    }
    loop {}
}

unsafe fn read_input<'a>() -> (&'a [u8], &'a [u8], Option<&'a [u32; STORAGE_SLOTS * 8]>) {
    let ptr = INPUT_ADDR as *const u8;
    let code_len = (ptr as *const u32).read_volatile() as usize;
    let calldata_len = (ptr.add(4) as *const u32).read_volatile() as usize;
    let has_storage = (ptr.add(8) as *const u32).read_volatile();
    let code_start = ptr.add(12);
    let calldata_start = ptr.add(12 + code_len);
    let code = core::slice::from_raw_parts(code_start, code_len);
    let calldata = core::slice::from_raw_parts(calldata_start, calldata_len);
    let storage_init = if has_storage != 0 {
        let storage_ptr = calldata_start.add(calldata_len) as *const u32;
        Some(&*(storage_ptr as *const [u32; STORAGE_SLOTS * 8]))
    } else {
        None
    };
    (code, calldata, storage_init)
}

fn one() -> U256 {
    [1u32, 0, 0, 0, 0, 0, 0, 0]
}

fn slt(a: &U256, b: &U256) -> bool {
    let sign_a = (a[7] >> 31) != 0;
    let sign_b = (b[7] >> 31) != 0;
    if sign_a != sign_b {
        return sign_a; // negative < positive
    }
    lt(a, b)
}

fn sgt(a: &U256, b: &U256) -> bool {
    slt(b, a)
}

fn sdiv(a: &U256, b: &U256) -> U256 {
    if is_zero(b) {
        return zero();
    }
    // Two's complement: negate if MSB set
    let sign_a = (a[7] >> 31) != 0;
    let sign_b = (b[7] >> 31) != 0;
    let abs_a = if sign_a { twos_complement(a) } else { *a };
    let abs_b = if sign_b { twos_complement(b) } else { *b };
    let q = div(&abs_a, &abs_b);
    if sign_a ^ sign_b {
        twos_complement(&q)
    } else {
        q
    }
}

fn srem(a: &U256, b: &U256) -> U256 {
    if is_zero(b) {
        return zero();
    }
    let sign_a = (a[7] >> 31) != 0;
    let sign_b = (b[7] >> 31) != 0;
    let abs_a = if sign_a { twos_complement(a) } else { *a };
    let abs_b = if sign_b { twos_complement(b) } else { *b };
    let r = rem(&abs_a, &abs_b);
    if sign_a {
        twos_complement(&r)
    } else {
        r
    }
}

fn twos_complement(a: &U256) -> U256 {
    add(&not(a), &one())
}

fn exp_op(base: &U256, exp: &U256) -> U256 {
    let mut result = one();
    let mut b = *base;
    let mut e = *exp;
    while !is_zero(&e) {
        if e[0] & 1 == 1 {
            result = mul(&result, &b);
        }
        b = mul(&b, &b);
        e = shr(&e, &one());
    }
    result
}
