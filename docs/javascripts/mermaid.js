window.addEventListener("load", () => {
  if (typeof mermaid === "undefined") {
    return;
  }
  mermaid.initialize({
    startOnLoad: true,
    securityLevel: "strict"
  });
});
