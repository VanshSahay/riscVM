# zkVM Arithmetization: The Step Circuit

The `StepCircuit` is the mathematical heart of riscVM. It defines the constraints for a single execution step, ensuring the transition from state $S_n$ to state $S_{n+1}$ is correct according to the RV32I specification.

## The State Transition Function: `f(Sn) -> Sn+1`

Each execution step is modeled as a transformation of the CPU state:
- **Input (Witnesses):** `PCBefore`, `RegsBefore[32]`, `Instr`.
- **Output (Constraints):** `PCAfter`, `RegsAfter[32]`.

The circuit guarantees that for any given instruction, the output state is the ONLY possible outcome.

## Arithmetization Strategies

### 1. Unified Decoding & Instruction Selection
To avoid massive branching (which is expensive in ZK), we use a **One-Hot Instruction Selector** strategy.
1.  **Bit Decomposition:** The 32-bit `Instr` is decomposed into bits.
2.  **Field Extraction:** `Opcode`, `Rd`, `Rs1`, `Rs2`, `Funct3`, `Funct7` are reconstructed from these bits and constrained to match the raw `Instr`.
3.  **Selector Logic:** A selector variable (e.g., `IsAdd`) is computed as a boolean product of opcode and funct flags.
    - Example: `IsAdd = (Opcode == 0x33) * (Funct3 == 0x0) * (Funct7 == 0x00)`

### 2. Register File Multiplexing (`selectReg`)
Selecting a value from the 32-register array based on a 5-bit `Rs1` is done using a large multiplexer:
- $Val = \sum_{i=0}^{31} RegsBefore[i] \cdot (Index == i)$
- The circuit iterates through all 32 registers and uses `api.Select` to pick the correct one.

### 3. Bitwise Logic Implementation
Since gnark operates over a large prime field, standard bitwise operations (`&`, `|`, `^`) must be manually arithmetized at the bit level:
- **AND:** $A \cdot B$
- **OR:** $A + B - (A \cdot B)$
- **XOR:** $A + B - 2(A \cdot B)$
For 32-bit integers, both operands are bit-decomposed, the bitwise gates are applied to each bit pair, and the result is re-composed into a field element.

### 4. Control Flow Integrity
The `PCAfter` is constrained through a hierarchical selector:
1.  **Branches:** If the instruction is a branch and the condition (e.g., $Reg[Rs1] == Reg[Rs2]$) is met, $PCAfter = PCBefore + Imm$.
2.  **Jumps (JAL/JALR):** $PCAfter$ is set to the target address. For `JALR`, the circuit bit-decomposes the target and forces the least significant bit (LSB) to zero.
3.  **Default:** $PCAfter = PCBefore + 4$.

## Circuit Validation (`zk/zk_test.go`)

We use the `gnark/test` package to verify the circuit against multiple backends.
- **BN254 Field:** Our primary target for Ethereum compatibility.
- **Groth16 Prover:** Generates succinct, fast-to-verify proofs.
- **Negative Tests:** We intentionally provide invalid witnesses to ensure the circuit rejects incorrect state transitions.

## Future Roadmap: Memory & Recursion

To handle an entire program's execution:
- **Memory Log (Plookup):** Every load/store will be recorded in a trace. A permutation argument will prove that every "read" from an address returns the value of the most recent "write".
- **Recursive Aggregation:** Each `StepCircuit` proof will be an input to a "Wrapper" circuit, which recursively verifies that proof and its predecessor, eventually producing a single proof for $N$ steps.

## Usage

Run the verification suite:
```bash
go test -v ./zk
```
To inspect a specific witness:
```bash
# Add fmt.Printf in prover.go and run
go test -v ./zk -run TestStepCircuit
```
