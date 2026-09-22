import React from "react";
import { createRoot } from "react-dom/client";
import "@patternfly/patternfly/patternfly.css";
import "./styles.css";
import { App } from "./App";

const node = document.getElementById("app");
if (!node) throw new Error("app root not found");
createRoot(node).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
