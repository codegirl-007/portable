(() => {
  const statusEl = document.getElementById("status");
  const termEl = document.getElementById("term");
  const kb = document.getElementById("kb");
  const ctrlBtn = document.getElementById("ctrl");

  const term = new Terminal({
    cursorBlink: true,
    fontSize: 14,
    fontFamily: "ui-monospace, SFMono-Regular, Menlo, Consolas, monospace",
    theme: {
      background: "#0d1117",
      foreground: "#e6edf3",
      cursor: "#58a6ff",
    },
    scrollback: 5000,
  });
  const fit = new FitAddon.FitAddon();
  term.loadAddon(fit);
  term.open(termEl);

  let ws;
  let ctrl = false;
  let reconnectTimer;

  function setStatus(text, kind) {
    statusEl.textContent = text;
    statusEl.className = "status" + (kind ? " " + kind : "");
  }

  function sendBytes(s) {
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    ws.send(new TextEncoder().encode(s));
  }

  function applyCtrl(ch) {
    if (!ch) return ch;
    const code = ch.toLowerCase().charCodeAt(0);
    if (code >= 97 && code <= 122) {
      return String.fromCharCode(code - 96);
    }
    return ch;
  }

  function sendKey(name) {
    switch (name) {
      case "esc": sendBytes("\x1b"); break;
      case "tab": sendBytes("\t"); break;
      case "c-c": sendBytes("\x03"); break;
      case "up": sendBytes("\x1b[A"); break;
      case "down": sendBytes("\x1b[B"); break;
      case "right": sendBytes("\x1b[C"); break;
      case "left": sendBytes("\x1b[D"); break;
      case "ctrl":
        ctrl = !ctrl;
        ctrlBtn.classList.toggle("active", ctrl);
        break;
      case "kb":
        kb.focus();
        break;
    }
  }

  function sendResize() {
    fit.fit();
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    ws.send(JSON.stringify({ type: "resize", cols: term.cols, rows: term.rows }));
  }

  function connect() {
    const proto = location.protocol === "https:" ? "wss" : "ws";
    ws = new WebSocket(proto + "://" + location.host + "/pty");
    ws.binaryType = "arraybuffer";
    setStatus("connecting");

    ws.onopen = () => {
      setStatus("live", "ok");
      sendResize();
      term.focus();
    };
    ws.onmessage = (ev) => {
      if (ev.data instanceof ArrayBuffer) {
        term.write(new Uint8Array(ev.data));
        return;
      }
      if (typeof ev.data === "string") {
        term.write(ev.data);
      }
    };
    ws.onclose = () => {
      setStatus("reconnecting", "bad");
      clearTimeout(reconnectTimer);
      reconnectTimer = setTimeout(connect, 1200);
    };
    ws.onerror = () => ws.close();
  }

  term.onData((data) => {
    if (ctrl) {
      sendBytes(applyCtrl(data));
      ctrl = false;
      ctrlBtn.classList.remove("active");
      return;
    }
    sendBytes(data);
  });

  document.querySelectorAll(".keys button").forEach((btn) => {
    btn.addEventListener("click", (e) => {
      e.preventDefault();
      sendKey(btn.getAttribute("data-key"));
    });
  });

  kb.addEventListener("input", () => {
    const v = kb.value;
    if (v) {
      if (ctrl) {
        sendBytes(applyCtrl(v));
        ctrl = false;
        ctrlBtn.classList.remove("active");
      } else {
        sendBytes(v);
      }
    }
    kb.value = "";
  });

  window.addEventListener("resize", sendResize);
  if (window.visualViewport) {
    window.visualViewport.addEventListener("resize", sendResize);
  }
  termEl.addEventListener("click", () => {
    term.focus();
    if (/iPhone|iPad|Android/i.test(navigator.userAgent)) {
      kb.focus();
    }
  });

  fit.fit();
  connect();
})();
