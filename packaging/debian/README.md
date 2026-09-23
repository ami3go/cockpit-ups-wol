# Debian package

The release pipeline builds `cockpit-ups-wol` `.deb` packages for `amd64`, `arm64`, and `riscv64`.

The package deliberately separates **software installation** from **UPS enrollment/arming**:

- `dpkg`/`apt` installs the application payload under `/usr/lib/cockpit-ups-wol/`.
- Runtime dependencies are declared in the Debian control metadata.
- No UPS is reconfigured and no host is armed from a Debian maintainer script.
- Administrators run `cockpit-ups-wol-setup` explicitly after package installation.

Typical testing installation:

```bash
sudo apt install ./cockpit-ups-wol_<version>_amd64.deb
sudo cockpit-ups-wol-setup --tui
```

For non-destructive validation only:

```bash
sudo cockpit-ups-wol-setup --check --profile local-server
```

The setup wrapper selects the package's prebuilt binary directory for the current Debian architecture, so Go is not required on the target system.

Testing releases are GitHub **pre-releases** and must not be treated as the stable release channel. New installations still default to `dry-run`; arming remains an explicit post-validation administrator action.
