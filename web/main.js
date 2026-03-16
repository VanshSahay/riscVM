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

      witnessViewEl.innerHTML = `
        <div class="proof-cert">
          <div class="cert-header">Proof Certificate</div>
          <div class="cert-row"><span>Instr</span> <span>${w.instr} (${w.asm})</span></div>
          <div class="cert-row"><span>PC</span> <span>${w.pcBefore} → ${w.pcAfter}</span></div>
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
      outputBuffer = '';
      outputContentEl.textContent = '';
      instrHistory.length = 0;
      instrHistoryEl.innerHTML = '';
      instrCurrentEl.textContent = '—';
      setStatus('Program loaded. Entry ' + hex8(r.entry));
      updateUI();
      return true;
    }
    if (r && r.error) return r.error;
    return 'Load failed';
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
  const btnStep = document.getElementById('btnStep');
  const btnRun = document.getElementById('btnRun');

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

  if (btnStep) btnStep.addEventListener('click', onStep);
  if (btnRun) btnRun.addEventListener('click', runLoop);

  if (overlay) overlay.addEventListener('click', (e) => {
    if (e.target === overlay) overlay.hidden = true;
  });

  initWasm();
})();
