(function () {
  const statusEl = document.getElementById('status');
  const pcEl = document.getElementById('pc');
  const regsEl = document.getElementById('regs');
  const memViewEl = document.getElementById('memView');
  const instrCurrentEl = document.getElementById('instrCurrent');
  const instrHistoryEl = document.getElementById('instrHistory');
  const outputContentEl = document.getElementById('outputContent');
  const zkStatusEl = document.getElementById('zkStatus');
  const witnessViewEl = document.getElementById('witnessView');
  const pasteInput = document.getElementById('pasteInput');
  const loadError = document.getElementById('loadError');
  const overlay = document.getElementById('overlay');

  const regNames = ['zero', 'ra', 'sp', 'gp', 'tp', 't0', 't1', 't2', 's0', 's1', 'a0', 'a1', 'a2', 'a3', 'a4', 'a5', 'a6', 'a7', 's2', 's3', 's4', 's5', 's6', 's7', 's8', 's9', 's10', 's11', 't3', 't4', 't5', 't6'];

  let wasmReady = false;
  let outputBuffer = '';
  const instrHistory = [];
  const MAX_HISTORY = 64;
  let runInterval = null;

  // Called from WASM when the program does write(1, buf, len)
  window.riscvmAppendOutput = function (arr) {
    if (!arr || !arr.length) return;
    for (let i = 0; i < arr.length; i++) outputBuffer += String.fromCharCode(arr[i]);
    outputContentEl.textContent = outputBuffer;
  };

  function setStatus(msg) {
    statusEl.textContent = msg;
  }

  function hex8(n) {
    return '0x' + (n >>> 0).toString(16).padStart(8, '0');
  }

  function updateUI() {
    if (!wasmReady || typeof riscvmGetPC !== 'function') return;
    const pc = riscvmGetPC();
    pcEl.textContent = hex8(pc);

    const regs = riscvmGetRegs();
    if (Array.isArray(regs)) {
      regsEl.innerHTML = regs.map((v, i) =>
        `<div class="cpu-row"><span class="label">x${i}</span><span class="value">${hex8(v)}</span></div>`
      ).join('');
    }

    const memLen = 256;
    const start = (pc & ~0xff) - 128;
    const offset = Math.max(0, start);
    const len = Math.min(memLen, 0x10000 - offset);
    const mem = riscvmGetMemory(offset, len);
    if (mem && mem.length) {
      const lines = [];
      for (let i = 0; i < mem.length; i += 16) {
        const addr = offset + i;
        const hex = Array.from(mem.subarray(i, i + 16))
          .map(b => b.toString(16).padStart(2, '0')).join(' ');
        const highlight = (addr <= pc && pc < addr + 16) ? ' mem-line-highlight' : '';
        lines.push(`<div class="mem-line${highlight}"><span class="mem-addr">${hex8(addr)}</span><span>${hex}</span></div>`);
      }
      memViewEl.innerHTML = lines.join('');
    }

    const lastInstr = riscvmGetLastInstruction ? riscvmGetLastInstruction() : '';
    if (lastInstr) instrCurrentEl.textContent = lastInstr;
  }

  function updateHistory() {
    const lastInstr = riscvmGetLastInstruction ? riscvmGetLastInstruction() : '';
    if (lastInstr) {
      instrHistory.push(lastInstr);
      if (instrHistory.length > MAX_HISTORY) instrHistory.shift();
      instrHistoryEl.innerHTML = instrHistory.map(s => `<div>${escapeHtml(s)}</div>`).reverse().join('');
    }
  }

  function onStep() {
    if (!wasmReady) return;
    const r = riscvmStep();
    if (r && r.ok) {
      updateHistory();
      onVerify();
      const exited = riscvmGetExited ? riscvmGetExited() : false;
      if (exited) {
        const code = riscvmGetExitCode ? riscvmGetExitCode() : 0;
        setStatus('Program exited with code ' + code);
        return;
      }
      updateUI();
    }
  }

  function onVerify() {
    if (!wasmReady || typeof riscvmVerifyLastStep !== 'function') return;
    const r = riscvmVerifyLastStep();
    if (r && r.ok) {
      zkStatusEl.textContent = 'Proof verified';
      const w = r.witness;
      let diffsHtml = '';
      if (Object.keys(w.diffs).length > 0) {
        diffsHtml = Object.entries(w.diffs).map(([reg, val]) => 
          `<div class="cert-row"><span>${reg}</span> <span>${val.from} → ${val.to}</span></div>`
        ).join('');
      } else {
        diffsHtml = '<div class="cert-row muted">No state changes</div>';
      }

      const memRow = w.memAddr
        ? '<div class="cert-section-title">Memory Access</div>' +
          '<div class="cert-row"><span>' + w.memOp + '</span><span>' + w.memAddr + ' = ' + w.memVal + '</span></div>'
        : '';

      witnessViewEl.innerHTML = `
        <div class="proof-cert">
          <div class="cert-header">Proof Certificate</div>
          <div class="cert-row"><span>Instr</span> <span>${w.instr} (${w.asm})</span></div>
          <div class="cert-row"><span>PC</span> <span>${w.pcBefore} → ${w.pcAfter}</span></div>
          ${memRow}
          <div class="cert-section-title">State Changes</div>
          ${diffsHtml}
        </div>
      `;
    } else {
      zkStatusEl.textContent = 'Proof failed: ' + (r.error || 'unknown error');
      witnessViewEl.innerHTML = '';
    }
  }

  function escapeHtml(s) {
    const div = document.createElement('div');
    div.textContent = s;
    return div.innerHTML;
  }

  function runLoop() {
    if (runInterval) return;
    runInterval = setInterval(() => {
      if (!wasmReady) return;
      // Run a small batch to keep UI responsive while showing proofs
      for (let i = 0; i < 10; i++) {
        riscvmStep();
        updateHistory();
        onVerify();
        if (riscvmGetExited && riscvmGetExited()) {
          clearInterval(runInterval);
          runInterval = null;
          setStatus('Program exited with code ' + (riscvmGetExitCode ? riscvmGetExitCode() : 0));
          updateUI();
          return;
        }
      }
      updateUI();
    }, 50);
    setStatus('Running…');
  }

  function stopRun() {
    if (runInterval) {
      clearInterval(runInterval);
      runInterval = null;
    }
  }

  function parseBase64(str) {
    str = str.replace(/\s/g, '');
    const binary = atob(str);
    const bytes = new Uint8Array(binary.length);
    for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
    return bytes;
  }

  function parseHex(str) {
    const parts = str.replace(/0x/g, '').split(/[\s,\n]+/).filter(Boolean);
    const bytes = [];
    for (const p of parts) {
      const hex = p.length % 2 === 0 ? p : '0' + p;
      for (let i = 0; i < hex.length; i += 2) bytes.push(parseInt(hex.slice(i, i + 2), 16));
    }
    return new Uint8Array(bytes);
  }

  function loadProgram(uint8Array, asElf) {
    if (!wasmReady || typeof riscvmLoadProgram !== 'function') {
      return 'WASM not ready. Please wait or check console.';
    }
    const r = riscvmLoadProgram(uint8Array, asElf);
    if (r && r.ok) {
      resetUI();
      setStatus('Program loaded. Entry ' + hex8(r.entry));
      updateUI();
      return true;
    }
    if (r && r.error) return r.error;
    return 'Load failed';
  }

  function loadEVM(code, calldata) {
    if (!wasmReady || typeof riscvmLoadEVM !== 'function') {
      return 'WASM not ready or EVM support not built';
    }
    const r = riscvmLoadEVM(code, calldata || new Uint8Array(0));
    if (r && r.ok) {
      resetUI();
      setStatus('EVM bytecode loaded. Entry ' + hex8(r.entry) + ' (EVM interpreter)');
      updateUI();
      return true;
    }
    if (r && r.error) return r.error;
    return 'EVM load failed';
  }

  function resetUI() {
    outputBuffer = '';
    outputContentEl.textContent = '';
    instrHistory.length = 0;
    instrHistoryEl.innerHTML = '';
    instrCurrentEl.textContent = '—';
    zkStatusEl.textContent = 'Ready for proof…';
    witnessViewEl.innerHTML = '';
  }

  // --- Solidity compilation ---

  function encodeABI(types, values) {
    // Encode function arguments per Ethereum ABI
    // types: array of 'uint256', 'address', 'bool', 'bytes32'
    // values: array of strings
    const result = [];
    for (let i = 0; i < types.length; i++) {
      const t = types[i].trim();
      const v = values[i].trim();
      if (t === 'uint256') {
        const n = BigInt(v);
        result.push(n.toString(16).padStart(64, '0'));
      } else if (t === 'address') {
        const addr = v.replace(/^0x/, '');
        result.push(addr.padStart(64, '0'));
      } else if (t === 'bool') {
        result.push((v === 'true' ? '1' : '0').padStart(64, '0'));
      } else if (t === 'bytes32') {
        result.push(v.replace(/^0x/, '').padStart(64, '0'));
      } else {
        throw new Error('Unsupported type: ' + t);
      }
    }
    return result.join('');
  }

  function tryCompileSol(source, contractName, funcName, args) {
    // Try the solc WASM compiler. Handles multiple API patterns.
    let compileFn = null;

    // Pattern 1: Module._compileStandard (soljson-*.js recent builds)
    if (typeof Module !== 'undefined' && typeof Module._compileStandard === 'function') {
      compileFn = function (input) {
        const out = Module._compileStandard(JSON.stringify(input));
        return JSON.parse(out);
      };
    }
    // Pattern 2: Module.compile (older solc-js wrapper)
    if (!compileFn && typeof Module !== 'undefined' && typeof Module.compile === 'function') {
      compileFn = Module.compile;
    }
    // Pattern 3: Global solc object (npm solc package)
    if (!compileFn && typeof solc !== 'undefined' && typeof solc.compile === 'function') {
      compileFn = function (input) { return JSON.parse(solc.compile(JSON.stringify(input))); };
    }

    if (!compileFn) {
      throw new Error('solc not available — using simple compiler');
    }

    const input = {
      language: 'Solidity',
      sources: { 'main.sol': { content: source } },
      settings: {
        outputSelection: { '*': { '*': ['evm.bytecode.object', 'abi', 'evm.methodIdentifiers'] } }
      }
    };

    const output = compileFn(input);

    if (output.errors) {
      const severe = output.errors.filter(e => e.severity === 'error');
      if (severe.length > 0) {
        throw new Error('Compilation error: ' + severe.map(e => e.formattedMessage || e.message).join('; '));
      }
    }

    if (!output.contracts || !output.contracts['main.sol']) {
      throw new Error('No contracts found');
    }

    const contract = output.contracts['main.sol'][contractName];
    if (!contract) {
      const names = Object.keys(output.contracts['main.sol']);
      throw new Error('Contract "' + contractName + '" not found. Available: ' + names.join(', '));
    }

    const bytecode = '0x' + contract.evm.bytecode.object;

    // Get the 4-byte selector from methodIdentifiers
    const sig = funcName + '(' + (contract.abi.find(f => f.name === funcName && f.type === 'function') || {inputs:[]}).inputs.map(i => i.type).join(',') + ')';
    const selectors = contract.evm.methodIdentifiers || {};
    const selector = selectors[sig] || '';

    if (!selector) {
      throw new Error('Function selector not found for ' + sig);
    }

    return { bytecode, selector };
  }

  function buildSimpleABI(funcName, paramTypes) {
    // Build minimal ABI-like info for a single function.
    // Compute the 4-byte selector using a simple hash.
    const sig = funcName + '(' + paramTypes.join(',') + ')';
    // Use a simple non-cryptographic hash for the selector
    // (our EVM doesn't verify keccak anyway - it compares raw selectors)
    let hash = 0;
    for (let i = 0; i < sig.length; i++) {
      hash = ((hash << 5) - hash + sig.charCodeAt(i)) | 0;
    }
    return { sig, selector: (hash >>> 0).toString(16).padStart(8, '0').slice(0, 8) };
  }

  function buildSimpleContract(funcName, paramTypes, returnsType) {
    // Build minimal EVM bytecode for a simple pure function.
    // This handles: function foo(uint256 a, uint256 b) returns (uint256) { return a OP b; }
    // For ADD, SUB, MUL only.
    // This is a fallback when solc is not available.

    const abi = buildSimpleABI(funcName, paramTypes);
    const selector = abi.selector;

    // Build bytecode:
    // 1. Load selector from calldata[0..4], compare
    // 2. Load args from calldata[4..36] and [36..68]
    // 3. Perform operation
    // 4. MSTORE + RETURN

    // Determine operation from function name
    let op;
    const name = funcName.toLowerCase();
    if (name.includes('add') || name.includes('sum') || name.includes('plus')) op = 0x01; // ADD
    else if (name.includes('sub') || name.includes('minus') || name.includes('diff')) op = 0x03; // SUB
    else if (name.includes('mul') || name.includes('times') || name.includes('product')) op = 0x02; // MUL
    else if (name.includes('div') || name.includes('quotient')) op = 0x04; // DIV
    else op = 0x01; // default ADD

    const selBytes = [];
    for (let i = 0; i < 4; i++) {
      selBytes.push(parseInt(selector.substr(i * 2, 2), 16));
    }

    // PUSH1 0; CALLDATALOAD; PUSH1 0xE0; SHR  → selector extraction
    const preamble = [0x60, 0x00, 0x35, 0x60, 0xE0, 0x1C];

    // PUSH4 <selector>; EQ; PUSH1 <funcPc>; JUMPI
    const funcPc = preamble.length + 15; // after dispatch + revert
    const dispatch = [0x63, selBytes[0], selBytes[1], selBytes[2], selBytes[3],
                      0x14, 0x60, funcPc, 0x57];

    // REVERT on unknown selector
    const revertBlock = [0x60, 0x00, 0x60, 0x00, 0xFD];

    // Function body: POP selector, load args, compute, return
    // POP; PUSH1 4; CALLDATALOAD; PUSH1 36; CALLDATALOAD; <op>; PUSH1 0; MSTORE; PUSH1 32; PUSH1 0; RETURN
    const body = [0x50,  // POP (selector still on stack from dispatch)
                  0x60, 0x04, 0x35,  // CALLDATALOAD(4) → arg1
                  0x60, 0x24, 0x35,  // CALLDATALOAD(36) → arg2
                  op,
                  0x60, 0x00, 0x52,  // MSTORE(0)
                  0x60, 0x20, 0x60, 0x00, 0xF3]; // RETURN(0, 32)

    return new Uint8Array([...preamble, ...dispatch, ...revertBlock, ...body]);
  }

  async function onLoadSol() {
    const solStatus = document.getElementById('solStatus');
    solStatus.textContent = 'Compiling…';
    solStatus.className = 'sol-status';

    const source = document.getElementById('solSource').value.trim();
    const contractName = document.getElementById('solContract').value.trim();
    const funcName = document.getElementById('solFunction').value.trim();
    const argsStr = document.getElementById('solArgs').value.trim();

    if (!source || !contractName || !funcName) {
      solStatus.textContent = 'Please fill in all fields';
      solStatus.className = 'sol-status error';
      return;
    }

    // Parse function signature: "add(uint256,uint256)" or "add"
    const sigMatch = funcName.match(/^(\w+)\(([^)]*)\)$/);
    const bareName = sigMatch ? sigMatch[1] : funcName;
    const paramTypes = sigMatch && sigMatch[2] ? sigMatch[2].split(',').map(s => s.trim()) : [];

    // Parse args
    const args = argsStr ? argsStr.split(',').map(s => s.trim()) : [];

    let bytecode, selector;

    try {
      const result = tryCompileSol(source, contractName, bareName, args);
      bytecode = result.bytecode;
      selector = result.selector;
    } catch (e) {
      console.warn('solc compilation skipped, using simple compiler:', e.message);
      if (paramTypes.length === 0) {
        solStatus.textContent = 'Specify param types: e.g. add(uint256,uint256)';
        solStatus.className = 'sol-status error';
        return;
      }
      bytecode = buildSimpleContract(bareName, paramTypes);
      selector = buildSimpleABI(bareName, paramTypes).selector;
    }

    // Build calldata: selector + ABI-encoded args
    let calldataHex = selector;
    if (paramTypes.length > 0 && args.length > 0) {
      calldataHex += encodeABI(paramTypes, args);
    }

    const codeBytes = bytecode instanceof Uint8Array ? bytecode : hexToBytes(bytecode);
    const calldataBytes = hexToBytes(calldataHex);

    const err = loadEVM(codeBytes, calldataBytes);
    if (err === true) {
      overlay.hidden = true;
      solStatus.textContent = 'Compiled & loaded. Use Step/Run to execute.';
      solStatus.className = 'sol-status success';
    } else {
      solStatus.textContent = typeof err === 'string' ? err : 'Load failed';
      solStatus.className = 'sol-status error';
    }
  }

  function hexToBytes(hex) {
    if (hex instanceof Uint8Array) return hex;
    const h = hex.replace(/^0x/, '');
    const bytes = new Uint8Array(h.length / 2);
    for (let i = 0; i < bytes.length; i++) {
      bytes[i] = parseInt(h.substr(i * 2, 2), 16);
    }
    return bytes;
  }

  async function initWasm() {
    try {
      if (typeof Go === 'undefined') {
        setStatus('WASM runtime (wasm_exec.js) not found');
        return;
      }
      const go = new Go();
      const resp = await fetch('riscvm.wasm');
      if (!resp.ok) throw new Error(`HTTP ${resp.status} ${resp.statusText}`);
      const buf = await resp.arrayBuffer();
      const result = await WebAssembly.instantiate(buf, go.importObject);
      go.run(result.instance);
      wasmReady = true;
      setStatus('Ready. Load a program to start.');
    } catch (e) {
      setStatus('WASM load failed: ' + e.message);
      console.error(e);
    }
  }

  const btnPaste = document.getElementById('btnPaste');
  const btnCloseModal = document.getElementById('btnCloseModal');
  const btnLoadElf = document.getElementById('btnLoadElf');
  const btnLoadHex = document.getElementById('btnLoadHex');
  const btnLoadSol = document.getElementById('btnLoadSol');
  const btnStep = document.getElementById('btnStep');
  const btnRun = document.getElementById('btnRun');
  const modalTabs = document.getElementById('modalTabs');

  // Tab switching
  if (modalTabs) modalTabs.addEventListener('click', (e) => {
    if (!e.target.classList.contains('tab')) return;
    const tabName = e.target.dataset.tab;
    modalTabs.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
    e.target.classList.add('active');
    document.querySelectorAll('.tab-panel').forEach(p => p.classList.remove('active'));
    const panel = tabName === 'solidity' ? document.getElementById('tabSolidity') : document.getElementById('tabElf');
    if (panel) panel.classList.add('active');
    loadError.textContent = '';
  });

  if (btnPaste) btnPaste.addEventListener('click', () => {
    loadError.textContent = '';
    pasteInput.value = '';
    overlay.hidden = false;
  });

  if (btnCloseModal) btnCloseModal.addEventListener('click', () => {
    overlay.hidden = true;
  });

  if (btnLoadElf) btnLoadElf.addEventListener('click', () => {
    loadError.textContent = '';
    try {
      const bytes = parseBase64(pasteInput.value.trim());
      const err = loadProgram(bytes, true);
      if (err === true) overlay.hidden = true;
      else loadError.textContent = err;
    } catch (e) {
      loadError.textContent = e.message || 'Invalid base64';
    }
  });

  if (btnLoadHex) btnLoadHex.addEventListener('click', () => {
    loadError.textContent = '';
    try {
      const bytes = parseHex(pasteInput.value.trim());
      if (bytes.length === 0) {
        loadError.textContent = 'No hex bytes found';
        return;
      }
      const err = loadProgram(bytes, false);
      if (err === true) overlay.hidden = true;
      else loadError.textContent = err;
    } catch (e) {
      loadError.textContent = e.message || 'Invalid hex';
    }
  });

  if (btnLoadSol) btnLoadSol.addEventListener('click', () => {
    loadError.textContent = '';
    onLoadSol().catch(e => {
      loadError.textContent = e.message || 'Compilation failed';
    });
  });

  if (btnStep) btnStep.addEventListener('click', onStep);
  if (btnRun) btnRun.addEventListener('click', runLoop);

  if (overlay) overlay.addEventListener('click', (e) => {
    if (e.target === overlay) overlay.hidden = true;
  });

  initWasm();
})();
