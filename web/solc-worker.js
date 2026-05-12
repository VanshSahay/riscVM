// solc Web Worker — loads the Solidity compiler in a background thread
// to avoid Chrome's 8MB synchronous WASM compilation limit.

let solcReady = false;
let pending = [];

function compile(input) {
  const fn = solcReady
    ? (typeof Module.cwrap === 'function'
        ? Module.cwrap('compileStandard', 'string', ['string', 'number'])
        : null)
    : null;
  if (fn) {
    return JSON.parse(fn(JSON.stringify(input)));
  }
  // Manual string marshaling fallback.
  if (!solcReady || typeof Module._malloc !== 'function') {
    throw new Error('solc runtime not initialized');
  }
  const json = JSON.stringify(input);
  const ptr = Module._malloc(json.length + 1);
  Module.stringToUTF8(json, ptr, json.length + 1);
  const outPtr = Module._compileStandard(ptr, 0);
  const out = Module.UTF8ToString(outPtr);
  Module._free(ptr);
  return JSON.parse(out);
}

function onReady() {
  solcReady = true;
  for (const { input, resolve, reject } of pending) {
    try { resolve(compile(input)); } catch (e) { reject(e); }
  }
  pending = [];
}

function enqueue(input) {
  return new Promise((resolve, reject) => {
    if (solcReady) {
      try { resolve(compile(input)); } catch (e) { reject(e); }
      return;
    }
    pending.push({ input, resolve, reject });
  });
}

// Load the solc WASM binary via importScripts.
// The Module object is set up by the solc script; we hook onRuntimeInitialized.
self.Module = {
  onRuntimeInitialized: onReady,
  locateFile: function (path) {
    return 'https://binaries.soliditylang.org/bin/' + path;
  }
};

try {
  importScripts('https://binaries.soliditylang.org/bin/soljson-v0.8.25+commit.b61c2a91.js');
} catch (e) {
  self.postMessage({ type: 'error', error: 'Failed to load solc: ' + e.message });
}

self.onmessage = function (e) {
  const { id, source, contractName } = e.data;

  const input = {
    language: 'Solidity',
    sources: { 'main.sol': { content: source } },
    settings: {
      outputSelection: { '*': { '*': ['evm.bytecode.object', 'abi', 'evm.methodIdentifiers'] } }
    }
  };

  enqueue(input).then(function (output) {
    if (output.errors) {
      const severe = output.errors.filter(function (e) { return e.severity === 'error'; });
      if (severe.length > 0) {
        self.postMessage({ id: id, type: 'error', error: severe.map(function (e) { return e.formattedMessage || e.message; }).join('; ') });
        return;
      }
    }

    const files = output.contracts && output.contracts['main.sol'];
    if (!files) {
      self.postMessage({ id: id, type: 'error', error: 'No contracts found in compilation output' });
      return;
    }

    const names = Object.keys(files);
    const name = contractName || names[0];
    const contract = files[name];
    if (!contract) {
      self.postMessage({ id: id, type: 'error', error: 'Contract "' + name + '" not found. Available: ' + names.join(', ') });
      return;
    }

    self.postMessage({
      id: id,
      type: 'result',
      bytecode: '0x' + contract.evm.bytecode.object,
      abi: contract.abi,
      methodIdentifiers: contract.evm.methodIdentifiers || {}
    });
  }).catch(function (e) {
    self.postMessage({ id: id, type: 'error', error: 'Compilation error: ' + e.message });
  });
};
