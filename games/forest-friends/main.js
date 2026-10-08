const go = new Go();
fetch("main.wasm")
  .then(r => { if (!r.ok) throw new Error(`main.wasm: HTTP ${r.status}`); return r.arrayBuffer(); })
  .then(buf => WebAssembly.instantiate(buf, go.importObject))
  .then(res => go.run(res.instance))
  .catch(err => {
    console.error(err);
    document.getElementById("load-error").textContent = "Failed to load game: " + err.message;
  });
