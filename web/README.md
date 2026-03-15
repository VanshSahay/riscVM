# riscVM Web UI

Single-page UI to run and inspect the RISC-V VM in the browser (WASM).

## Build

From repo root:

```bash
GOOS=js GOARCH=wasm go build -o web/riscvm.wasm ./cmd/wasm
```

Copy the Go WASM JS glue once (if not already in `web/`):

```bash
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" web/
```

## Run

Serve the `web/` directory over HTTP (WASM requires same-origin or CORS). For example:

```bash
cd web && python3 -m http.server 8080
```

Open http://localhost:8080

## Use

1. **Paste program** – Open the modal and paste either:
   - **ELF (base64)**: After building with e.g.  
     `riscv32-unknown-elf-gcc -march=rv32i -mabi=ilp32 -nostdlib -o prog.elf prog.s`, get base64 with  
     `base64 -i prog.elf` (macOS) or `base64 < prog.elf` (works on both).
   - **Hex bytes**: Space- or newline-separated hex (e.g. `93 00 10 00` or `0x00100093`), loaded at address 0.
2. **Step** – Execute one instruction and update CPU / memory / instruction view.
3. **Run** – Auto-step until the program exits (ecall exit).
