import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { App } from "./app";
import "./styles.css";
import "./lab.css";
import "./api-lab-layout.css";

const root = document.getElementById("root");
if (!root) throw new Error("Manager V2 root element was not found");

createRoot(root).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
