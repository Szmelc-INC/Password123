# Team SSH certificate authority (phase 2)

A team CA signs each member's `ssh.pub` into a short-lived **SSH user certificate** (`members/<handle>/ssh-cert.pub`). Hosts and tools then trust **one** CA key instead of a list of member keys, and access ends when a certificate expires or is not renewed.

This slot stays empty until a maintainer creates the CA. **Nobody but the CA holder may generate the CA key**, so it is not created automatically.

## Create the CA (one maintainer, once)

```sh
scripts/team.sh ca-init           # private key: ~/.config/password123/ca/team_ca (passphrase prompt)
                                  # public key:  team/ca/team_ca.pub
```

Keep the private CA key offline (encrypted USB or hardware token), with a second, separately stored backup. Commit only `team_ca.pub` through a PR. Confirm its fingerprint (`ssh-keygen -l -f team/ca/team_ca.pub`) with a second maintainer over another channel.

## Issue and renew certificates

```sh
scripts/team.sh issue-cert <handle>                   # default validity +52w
scripts/team.sh issue-cert <handle> --validity +90d   # shorter for agents
```

Principals are `<handle>` and, if set, the member's email. The key ID is `<handle>@password123`, and the serial is the UTC issue time. `verify` checks that every certificate is signed by `team_ca.pub`, is for the member's `ssh.pub`, names the handle, and has not expired.

## Revoke

Certificates cannot be recalled. Keep validity short, revoke the member in the register (`scripts/team.sh revoke`), and for immediate effect publish an OpenSSH KRL (`ssh-keygen -k -f team/ca/revoked.krl -s team/ca/team_ca.pub <cert>`) that hosts load with `RevokedKeys`.

## Rotate the CA

A CA rotation is a reviewed PR: `ca-init` with a fresh `P123_TEAM_CA` directory, remove the old `team_ca.pub` first, then re-issue every certificate. Hosts trust both CA keys during the overlap.
