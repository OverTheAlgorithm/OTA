import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import "./index.css";
import App from "./App.tsx";

// ── Anti-cheat: disable right-click & DevTools shortcuts ──────────────────────
document.addEventListener("contextmenu", (e) => e.preventDefault());
document.addEventListener("keydown", (e) => {
  // F12
  if (e.key === "F12") {
    e.preventDefault();
    return;
  }
  // Ctrl+Shift+I / Ctrl+Shift+J / Ctrl+Shift+C (DevTools)
  if (e.ctrlKey && e.shiftKey && ["I", "J", "C"].includes(e.key)) {
    e.preventDefault();
    return;
  }
  // Ctrl+U (View Source)
  if (e.ctrlKey && e.key === "u") {
    e.preventDefault();
    return;
  }
});

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
