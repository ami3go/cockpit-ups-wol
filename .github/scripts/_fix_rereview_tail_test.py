from pathlib import Path

p = Path('cockpit/src/App.test.tsx')
s = p.read_text()
s = s.replace(
    "import { fireEvent, render, screen, waitFor } from '@testing-library/react';",
    "import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';",
)
s = s.replace(
    "import { beforeEach, describe, expect, it, vi } from 'vitest';",
    "import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';",
)
s = s.replace(
    "describe('SettingsPanel validation/apply guard', () => {\n  beforeEach(() => {",
    "describe('SettingsPanel validation/apply guard', () => {\n  afterEach(() => cleanup());\n\n  beforeEach(() => {",
)
p.write_text(s)
