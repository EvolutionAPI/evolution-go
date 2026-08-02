import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { App } from "./app";
import "./styles.css";

const root = document.getElementById("root");
if (!root) throw new Error("Manager V2 root element was not found");

createRoot(root).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
