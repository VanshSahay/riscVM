#!/usr/bin/env python3
"""EVM bytecode assembler for the riscVM zkVM.

Usage: python3 evm_asm.py <source.evm> -o <output.bin>

Source syntax (one instruction per line):
  PUSH1 0x42        Push 1-byte immediate
  PUSH4 0xa9059cbb  Push 4-byte immediate
  PUSH32 <hex>      Push 32-byte immediate
  ADD|MUL|SUB|...   Opcode by name
  label_name:       Define a jump target
  %label_name       Reference to a label (resolved to PUSH1 <offset>)

Blank lines and # comments are ignored.
"""

import sys
import re
import struct
import os


OPCODES = {
    "STOP": 0x00,
    "ADD": 0x01,
    "MUL": 0x02,
    "SUB": 0x03,
    "DIV": 0x04,
    "SDIV": 0x05,
    "MOD": 0x06,
    "SMOD": 0x07,
    "ADDMOD": 0x08,
    "MULMOD": 0x09,
    "EXP": 0x0A,
    "SIGNEXTEND": 0x0B,
    "LT": 0x10,
    "GT": 0x11,
    "SLT": 0x12,
    "SGT": 0x13,
    "EQ": 0x14,
    "ISZERO": 0x15,
    "AND": 0x16,
    "OR": 0x17,
    "XOR": 0x18,
    "NOT": 0x19,
    "BYTE": 0x1A,
    "SHL": 0x1B,
    "SHR": 0x1C,
    "SAR": 0x1D,
    "SHA3": 0x20,
    "ADDRESS": 0x30,
    "BALANCE": 0x31,
    "ORIGIN": 0x32,
    "CALLER": 0x33,
    "CALLVALUE": 0x34,
    "CALLDATALOAD": 0x35,
    "CALLDATASIZE": 0x36,
    "CALLDATACOPY": 0x37,
    "CODESIZE": 0x38,
    "CODECOPY": 0x39,
    "GASPRICE": 0x3A,
    "POP": 0x50,
    "MLOAD": 0x51,
    "MSTORE": 0x52,
    "MSTORE8": 0x53,
    "SLOAD": 0x54,
    "SSTORE": 0x55,
    "JUMP": 0x56,
    "JUMPI": 0x57,
    "PC": 0x58,
    "MSIZE": 0x59,
    "JUMPDEST": 0x5B,
    "PUSH1": 0x60,
    "PUSH2": 0x61,
    "PUSH3": 0x62,
    "PUSH4": 0x63,
    "PUSH5": 0x64,
    "PUSH6": 0x65,
    "PUSH7": 0x66,
    "PUSH8": 0x67,
    "PUSH9": 0x68,
    "PUSH10": 0x69,
    "PUSH11": 0x6A,
    "PUSH12": 0x6B,
    "PUSH13": 0x6C,
    "PUSH14": 0x6D,
    "PUSH15": 0x6E,
    "PUSH16": 0x6F,
    "PUSH17": 0x70,
    "PUSH18": 0x71,
    "PUSH19": 0x72,
    "PUSH20": 0x73,
    "PUSH21": 0x74,
    "PUSH22": 0x75,
    "PUSH23": 0x76,
    "PUSH24": 0x77,
    "PUSH25": 0x78,
    "PUSH26": 0x79,
    "PUSH27": 0x7A,
    "PUSH28": 0x7B,
    "PUSH29": 0x7C,
    "PUSH30": 0x7D,
    "PUSH31": 0x7E,
    "PUSH32": 0x7F,
    "DUP1": 0x80,
    "DUP2": 0x81,
    "DUP3": 0x82,
    "DUP4": 0x83,
    "DUP5": 0x84,
    "DUP6": 0x85,
    "DUP7": 0x86,
    "DUP8": 0x87,
    "DUP9": 0x88,
    "DUP10": 0x89,
    "DUP11": 0x8A,
    "DUP12": 0x8B,
    "DUP13": 0x8C,
    "DUP14": 0x8D,
    "DUP15": 0x8E,
    "DUP16": 0x8F,
    "SWAP1": 0x90,
    "SWAP2": 0x91,
    "SWAP3": 0x92,
    "SWAP4": 0x93,
    "SWAP5": 0x94,
    "SWAP6": 0x95,
    "SWAP7": 0x96,
    "SWAP8": 0x97,
    "SWAP9": 0x98,
    "SWAP10": 0x99,
    "SWAP11": 0x9A,
    "SWAP12": 0x9B,
    "SWAP13": 0x9C,
    "SWAP14": 0x9D,
    "SWAP15": 0x9E,
    "SWAP16": 0x9F,
    "RETURN": 0xF3,
    "REVERT": 0xFD,
}


def parse_immediate(token):
    if token.startswith("0x"):
        return int(token, 16)
    return int(token)


class Assembler:
    def __init__(self, source):
        self.labels = {}
        self.backrefs = []
        self.output = bytearray()

    def assemble(self, source):
        lines = source.split("\n")
        # Pass 1: collect label positions and emit opcodes
        for line in lines:
            line = line.split("#")[0].strip()
            if not line:
                continue
            parts = line.split()
            instr = parts[0]

            if instr.endswith(":"):
                label = instr[:-1]
                self.labels[label] = len(self.output)
                continue

            if instr.startswith("%"):
                # Standalone label reference — shorthand for PUSH1 %label
                label = instr[1:]
                self.output.append(0x60)  # PUSH1 opcode
                self.backrefs.append((len(self.output), label, 1))
                self.output.append(0x00)  # placeholder
                continue

            opcode = OPCODES.get(instr.upper())
            if opcode is None:
                raise ValueError(f"Unknown instruction: {instr}")

            push_match = re.match(r"PUSH(\d+)", instr.upper())
            if push_match:
                n = int(push_match.group(1))
                operand = parts[1]
                self.output.append(opcode)
                if operand.startswith("%"):
                    # Label reference — emit placeholder bytes, patch in pass 2
                    label = operand[1:]
                    self.backrefs.append((len(self.output), label, n))
                    self.output.extend(b"\x00" * n)
                else:
                    val = parse_immediate(operand)
                    imm_bytes = val.to_bytes(n, "big")
                    self.output.extend(imm_bytes)
            else:
                self.output.append(opcode)

        # Pass 2: patch label references
        for offset, label, width in self.backrefs:
            if label not in self.labels:
                raise ValueError(f"Undefined label: {label}")
            target = self.labels[label]
            # PATCH PUSH<n> immediate bytes after the PUSH opcode (at offset).
            # offset points to the first placeholder byte (right after PUSH opcode).
            target_bytes = target.to_bytes(width, "big")
            for i in range(width):
                self.output[offset + i] = target_bytes[i]

        return bytes(self.output)


def main():
    if len(sys.argv) < 2:
        print(f"Usage: {sys.argv[0]} <source.evm> [-o output.bin]")
        sys.exit(1)

    src_file = sys.argv[1]
    out_file = None
    for i, arg in enumerate(sys.argv):
        if arg == "-o" and i + 1 < len(sys.argv):
            out_file = sys.argv[i + 1]

    if out_file is None:
        base = os.path.splitext(src_file)[0]
        out_file = base + ".bin"

    with open(src_file) as f:
        source = f.read()

    asm = Assembler(source)
    try:
        bytecode = asm.assemble(source)
    except ValueError as e:
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(1)

    with open(out_file, "wb") as f:
        f.write(bytecode)

    print(f"Assembled {len(bytecode)} bytes → {out_file}")
    print(f"Labels: { {k: hex(v) for k, v in asm.labels.items()} }")
    print(f"Hex: {bytecode.hex()}")


if __name__ == "__main__":
    main()
