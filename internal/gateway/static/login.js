(() => {
  const hash = location.hash || "";
  if (!hash.startsWith("#t=")) {
    return;
  }
  const token = hash.slice(3);
  history.replaceState(null, "", location.pathname + location.search);
  if (!token) {
    return;
  }
  const form = document.createElement("form");
  form.method = "POST";
  form.action = "/login";
  form.style.display = "none";
  const input = document.createElement("input");
  input.name = "t";
  input.value = token;
  form.appendChild(input);
  document.body.appendChild(form);
  form.submit();
})();
