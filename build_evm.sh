#!/bin/bash
set -euo pipefail

cd "$(dirname "$0")/evm"
cargo build --target riscv32i-unknown-none-elf --release
cp target/riscv32i-unknown-none-elf/release/evm ../examples/evm.elf
echo "Built: examples/evm.elf"
