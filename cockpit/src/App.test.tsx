// @vitest-environment jsdom
import React from "react";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { SettingsPanel } from "./App";
import { applyConfig, getConfigText, validateConfig, type Plan } from "./api";

vi.mock("./api", async () => {
  const actual = await vi.importActual<typeof import("./api")>("./api");
  return {
    ...actual,
    getConfigText: vi.fn(),
    validateConfig: vi.fn(),
    applyConfig: vi.fn(),
  };
});

const plan: Plan = {
  mode: "dry-run",
  ups: {
    profile: "existing",
    target: "ups@localhost:3493",
    power_cycle_capability: "unknown",
    synology_compatibility: false,
  },
  outage: { grace_period_seconds: 0 },
  recovery: {
    enabled: true,
    utility_stable_seconds: 1,
    network_wait_seconds: 10,
  },
  shutdown: [],
  restore: [],
  network_dependencies: [],
};

const status = {
  active: "cfg-a",
  last_known_good: "cfg-a",
  previous_known_good: "",
  known_good_revisions: [],
};

describe("SettingsPanel validation/apply guard", () => {
  afterEach(() => cleanup());

  beforeEach(() => {
    vi.resetAllMocks();
    vi.mocked(getConfigText).mockResolvedValue("mode: dry-run\n");
    vi.mocked(validateConfig).mockResolvedValue({ valid: true, plan });
    vi.mocked(applyConfig).mockResolvedValue(status);
    vi.spyOn(window, "confirm").mockReturnValue(true);
  });

  it("never enables Apply for text changed after validation", async () => {
    render(
      <SettingsPanel
        plan={plan}
        refresh={vi.fn().mockResolvedValue(undefined)}
      />,
    );
    const editor = await screen.findByLabelText("Canonical YAML configuration");
    fireEvent.change(editor, { target: { value: "mode: armed\n" } });
    fireEvent.click(screen.getByRole("button", { name: "Validate candidate" }));
    await waitFor(() =>
      expect(screen.getByRole("button", { name: /Activate/ })).toBeEnabled(),
    );

    fireEvent.change(editor, {
      target: { value: "mode: armed\n# changed after validation\n" },
    });
    expect(screen.getByRole("button", { name: /Activate/ })).toBeDisabled();
    expect(applyConfig).not.toHaveBeenCalled();
  });

  it("applies exactly the text that was validated", async () => {
    render(
      <SettingsPanel
        plan={plan}
        refresh={vi.fn().mockResolvedValue(undefined)}
      />,
    );
    const editor = await screen.findByLabelText("Canonical YAML configuration");
    const candidate = "mode: armed\n";
    fireEvent.change(editor, { target: { value: candidate } });
    fireEvent.click(screen.getByRole("button", { name: "Validate candidate" }));
    const activate = screen.getByRole("button", { name: /Activate/ });
    await waitFor(() => expect(activate).toBeEnabled());
    fireEvent.click(activate);
    await waitFor(() => expect(applyConfig).toHaveBeenCalledWith(candidate));
  });
});
