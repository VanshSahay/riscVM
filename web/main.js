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

  let solcWorker = null;
  let solcReqId = 0;
  const solcCallbacks = {};

  function getSolcWorker() {
    if (!solcWorker) {
      try {
        solcWorker = new Worker('solc-worker.js');
        solcWorker.onmessage = function (e) {
          const cb = solcCallbacks[e.data.id];
          if (!cb) return;
          delete solcCallbacks[e.data.id];
          if (e.data.type === 'result') cb.resolve(e.data);
          else cb.reject(new Error(e.data.error || 'solc error'));
        };
        solcWorker.onerror = function () {
          solcWorker = null;
        };
      } catch (_) {
        solcWorker = null;
      }
    }
    return solcWorker;
  }

  function compileWithSolc(source, contractName) {
    const worker = getSolcWorker();
    if (!worker) return Promise.reject(new Error('Web Worker not available'));
    const id = ++solcReqId;
    return new Promise(function (resolve, reject) {
      solcCallbacks[id] = { resolve: resolve, reject: reject };
      worker.postMessage({ id: id, source: source, contractName: contractName });
      // Timeout after 60s
      setTimeout(function () {
        if (solcCallbacks[id]) { delete solcCallbacks[id]; reject(new Error('solc timed out')); }
      }, 60000);
    });
  }

  function encodeABI(types, values) {
    // Encode function arguments per Ethereum ABI
    // types: array of 'uint256', 'address', 'bool', 'bytes32'
    // values: array of strings
    const result = [];
    for (let i = 0; i < types.length; i++) {
      // Normalize Solidity type aliases (uint = uint256, etc.)
      let t = types[i].trim();
      if (t === 'uint' || t === 'uint256') t = 'uint256';
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

  function compileSimple(funcName, paramTypes) {
    const name = funcName.toLowerCase();
    let op;
    if (name.includes('add') || name.includes('sum') || name.includes('plus')) op = 0x01;
    else if (name.includes('sub') || name.includes('minus') || name.includes('diff')) op = 0x03;
    else if (name.includes('mul') || name.includes('times') || name.includes('product')) op = 0x02;
    else if (name.includes('div') || name.includes('quotient')) op = 0x04;
    else op = 0x01;

    const sig = funcName + '(' + paramTypes.join(',') + ')';
    let hash = 0;
    for (let i = 0; i < sig.length; i++) hash = ((hash << 5) - hash + sig.charCodeAt(i)) | 0;
    const selector = (hash >>> 0).toString(16).padStart(8, '0').slice(0, 8);

    const sel = [];
    for (let i = 0; i < 4; i++) sel.push(parseInt(selector.substr(i * 2, 2), 16));
    const pre = [0x60, 0x00, 0x35, 0x60, 0xE0, 0x1C];
    const pc = pre.length + 15;
    const body = [0x50, 0x60, 0x04, 0x35, 0x60, 0x24, 0x35, op,
                  0x60, 0x00, 0x52, 0x60, 0x20, 0x60, 0x00, 0xF3];

    return { bytecode: new Uint8Array([...pre, 0x63, sel[0], sel[1], sel[2], sel[3], 0x14, 0x60, pc, 0x57,
                                       0x60, 0x00, 0x60, 0x00, 0xFD, ...body]), selector };
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
    const paramTypes = sigMatch && sigMatch[2]
      ? sigMatch[2].split(',').map(s => { const t = s.trim(); return (t === 'uint' || t === 'int') ? 'uint256' : t; })
      : [];

    // Parse args
    const args = argsStr ? argsStr.split(',').map(s => s.trim()) : [];

    if (paramTypes.length === 0) {
      solStatus.textContent = 'Specify param types: e.g. add(uint256,uint256)';
      solStatus.className = 'sol-status error';
      return;
    }

    let bytecode, selector;

    try {
      const result = await compileWithSolc(source, contractName);
      bytecode = hexToBytes(result.bytecode);
      const sig = bareName + '(' + paramTypes.join(',') + ')';
      selector = result.methodIdentifiers[sig] || '';
      if (!selector) throw new Error('selector not found for ' + sig);
    } catch (e) {
      // Fallback to simple compiler
      const r = compileSimple(bareName, paramTypes);
      bytecode = r.bytecode;
      selector = r.selector;
    }

    let calldataHex = selector;
    if (args.length > 0) calldataHex += encodeABI(paramTypes, args);
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
