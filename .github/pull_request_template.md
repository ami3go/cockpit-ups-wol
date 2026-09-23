## Summary

<!-- What changed and why? -->

## Safety / recovery impact

<!-- Describe any effect on outage handling, shutdown ordering, NUT/FSD, recovery, rollback, privileged operations, or state persistence. Use "None" if not applicable. -->

## Validation

- [ ] Relevant unit/integration tests pass
- [ ] Installer/shell checks pass when applicable
- [ ] Cockpit UI checks pass when applicable
- [ ] Failure/rollback path tested for safety-sensitive changes
- [ ] Documentation updated when behavior changed

## Hardware impact

<!-- Does this require new physical UPS, architecture, DSM, or outage/recovery acceptance? -->

## Checklist

- [ ] No secrets, real credentials, private keys, or sensitive production inventory committed
- [ ] Change remains fail-safe on unknown or ambiguous state
- [ ] `CHANGELOG.md` updated when user-visible behavior changed
