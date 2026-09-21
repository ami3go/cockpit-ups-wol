# SSH shutdown setup

`cockpit-ups-wol` uses SSH only for hosts configured with `shutdown.method: ssh`. Armed mode is fail-closed: the controller must be able to read the configured private key and the target host key must already be pinned in `/etc/cockpit-ups-wol/known_hosts`.

## 1. Create a dedicated remote account

Use a dedicated account such as `powerctl` on the target machine. Install only the controller's public key in that account's `authorized_keys`.

The controller private key should be stored under `/etc/cockpit-ups-wol/secrets/` and should normally be mode `0600`, owned by root. Do not place the key under `/root/.ssh`; the systemd service uses `ProtectHome=yes`.

## 2. Allow only the shutdown command without a password

Create a sudoers drop-in on the target host, for example `/etc/sudoers.d/cockpit-ups-wol`:

```text
powerctl ALL=(root) NOPASSWD: /sbin/shutdown -h now
```

Validate it with:

```bash
sudo visudo -cf /etc/sudoers.d/cockpit-ups-wol
```

If the target distribution enables `Defaults requiretty`, make sure it does not apply to the `powerctl` account.

The path must match the target system. The current v0.1 adapter intentionally uses `/sbin/shutdown -h now`; do not arm SSH shutdown for a target where that command does not exist.

## 3. Pin the host key

Do not automatically trust the first key seen on the network. Collect the key, inspect its fingerprint, and compare the fingerprint through an independent channel such as the target console.

Example:

```bash
host=server.lan
tmp="$(mktemp)"
ssh-keyscan -H "$host" > "$tmp"
ssh-keygen -lf "$tmp"
# Verify the displayed fingerprint against the target host before continuing.
sudo sh -c 'cat "$1" >> /etc/cockpit-ups-wol/known_hosts' sh "$tmp"
sudo chmod 0644 /etc/cockpit-ups-wol/known_hosts
rm -f "$tmp"
```

You can confirm the entry is resolvable without making a network connection:

```bash
ssh-keygen -F server.lan -f /etc/cockpit-ups-wol/known_hosts
```

## 4. Non-destructive remote preflight

Before a physical outage test, verify SSH connectivity, host-key pinning, key authentication, and the remote sudo policy without shutting the host down:

```bash
sudo ssh \
  -o BatchMode=yes \
  -o StrictHostKeyChecking=yes \
  -o UserKnownHostsFile=/etc/cockpit-ups-wol/known_hosts \
  -o GlobalKnownHostsFile=/dev/null \
  -o IdentitiesOnly=yes \
  -o ConnectTimeout=10 \
  -i /etc/cockpit-ups-wol/secrets/server \
  powerctl@server.lan \
  sudo -n -l /sbin/shutdown -h now
```

This checks whether the exact shutdown command is permitted; it does not execute the shutdown command.

## 5. What armed mode validates automatically

Before entering armed mode the agent checks that:

- the configured SSH private key exists and is readable from inside the systemd sandbox;
- the key is a regular file and is not group/world writable;
- the target address has a pinned entry in `/etc/cockpit-ups-wol/known_hosts`;
- OpenSSH client tooling needed for host-key verification is installed.

Remote connectivity and the remote sudoers rule must still be verified with the non-destructive preflight above before hardware acceptance.
