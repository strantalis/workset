# Terminal rendering follow-ups

Linear unavailable (auth required). Suggested issue details:

Title: TerminalPane: improve xterm refresh/DPI handling and renderer auto mode

Labels: improvements, ai-generated, ui, performance (adjust to workspace labels)

Body:
- Implement true auto renderer: try WebGL, fallback to canvas; expose a debug toggle.
- Add a relayout helper: recompute DPR-aware lineHeight, call fitAddon.fit(), then terminal.refresh after font load, resize, visibility change, renderer change, and bootstrap.
- Detect DPR/font changes via matchMedia and document.fonts events to trigger relayout.
- Add debug metrics: lastRenderAt from terminal.onRender, current DPR, cell dims; force refresh when output arrives but no render occurs.
- In bootstrap, await terminal.write callbacks before marking replayState live to avoid partial renders.
