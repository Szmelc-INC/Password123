# Team key register

The team register is the public, reviewed list of **who is on the Password123 team and which keys they own**. Every member, human or AI agent, generates their own official keypairs on their own machine. Only the **public** parts enter this directory, through a pull request. Private keys never leave the owner's machine.

Keys in this register are used for:

| Purpose | Key | How it is verified |
|---|---|---|
| **Encryption** ("who can read this?") | age X25519 recipient, `members/<handle>/age.pub` | age encrypts to it; payload access still goes through `recipients/maintainers.txt` |
| **Signing** (commits, tags, files, releases) | ed25519 SSH key, `members/<handle>/ssh.pub` | `allowed_signers` + `ssh-keygen -Y verify` / `git verify-commit` |
| **Authorization** (SSH login, deploy hosts, CI gates) | the same SSH key, optionally certified by the team CA (`ssh-cert.pub`) | `TrustedUserCAKeys` / `@cert-authority`, or `allowed_signers` |
| **Proof of possession** | `member.yaml.sig`, signed by the member's own SSH key | `scripts/team.sh verify` (CI) |

Encryption and signing keys are separate on purpose (ARCHITECTURE.md §12): an age key answers "who can read", an SSH key answers "who published".

## Layout

```text
team/
├── README.md              this specification
├── INTEGRATION.md         how to use the keys: git signing, file signing, age, SSH, CI
├── onboarding/
│   └── AGENT-PROMPTS.md   copy-paste prompts for agents (enroll, integrate, review)
├── ca/
│   ├── README.md          team SSH certificate authority (phase 2)
│   └── team_ca.pub        appears after `scripts/team.sh ca-init`
├── members/<handle>/
│   ├── member.yaml        public record: kind, roles, fingerprints, dates, status
│   ├── member.yaml.sig    SSH signature over member.yaml (namespace password123-team)
│   ├── age.pub            one classic age1 recipient
│   ├── ssh.pub            one ssh-ed25519 public key
│   └── ssh-cert.pub       optional team-CA certificate for ssh.pub
├── allowed_signers        GENERATED: ssh-keygen ALLOWED SIGNERS for active members
├── recipients.txt         GENERATED: age recipients of active members (not auto-enrolled)
├── SHA256SUMS             GENERATED: sha256 of every file in team/ and scripts/team.sh
└── SHA256SUMS.sig         optional maintainer signature over SHA256SUMS
```

`scripts/team.sh update` writes the generated files. CI runs `scripts/team.sh verify`, which regenerates them and fails if anything differs.

## Rules

1. **One person or agent, one handle.** Handles match `^[a-z0-9][a-z0-9-]{1,31}$`. Agents use a handle that names them and their operator, for example `claude-sx` or `codex-ci`.
2. **Private keys live outside the repository**, under `~/.config/password123/team/<handle>/` (mode 600). `team.sh` refuses any key directory inside the checkout, and `p123 validate`, `team.sh verify` and gitleaks all scan for private key material.
3. **Classic age keys only** (`age1…`), to match the existing recipients. age refuses to mix classic and post-quantum recipients in one file. A move to `age1pq1…` keys is one coordinated rotation of the whole team.
4. **Agents name a human operator** (`operator:`), who is responsible for the agent's key, its revocation, and its access. An agent gets the `member` role only.
5. **Emails are plain addresses.** The email becomes a git signing principal, so `verify` rejects wildcards, commas and spaces. A record cannot claim another member's principal.
6. **The `maintainer` role** can sign `SHA256SUMS`, revoke members and issue certificates. Adding the role to someone is a PR reviewed by an existing maintainer.
7. **Keys expire** (default 365 days). After expiry `verify` warns, and `verify --strict` fails. Rotate before then.
8. **Being in the register does not grant payload access.** Reading the encrypted solutions requires a maintainer to enroll the recipient into `recipients/maintainers.txt` (and `.sops.yaml` for SOPS files) and run `p123 rotate --all`. See CONTRIBUTING.md. The register is the identity source that feeds that runbook. It is not a second access list.

## Joining the team (one PR per member)

```sh
git switch -c team/enroll-<handle> origin/main
scripts/team.sh keygen <handle> --email you@example.org --github <login>          # human
scripts/team.sh keygen <handle> --kind agent --operator <human-handle> --no-passphrase   # agent
scripts/team.sh verify
git add team && git commit -S -m "team: enroll <handle>"   # signed with the new key, see INTEGRATION.md
```

Then open a PR titled `team: enroll <handle>`. Enrollment PRs that run at the same time conflict in the generated files. If yours conflicts, run `git merge origin/main`, then `scripts/team.sh update` and `scripts/team.sh verify`, and commit. Never hand-merge `allowed_signers`, `recipients.txt` or `SHA256SUMS`.

Requirements: bash, OpenSSH 8.2+ (`ssh-keygen -Y`), GNU coreutils (`date -d`, `realpath -m`, `sha256sum`), and `age-keygen` or Go (the script then runs the age version pinned in `go.mod`).

A maintainer:

1. checks that the PR adds exactly one `members/<handle>/` directory, plus the regenerated `allowed_signers`, `recipients.txt` and `SHA256SUMS`, and touches nothing else;
2. confirms **both fingerprints** (`ssh_fingerprint` and the full `age_recipient`) with the owner over a second channel (for an agent, with its operator);
3. merges when CI is green, then on `main` runs `scripts/team.sh sign-sums --handle <maintainer>` in a follow-up PR;
4. if the member needs payload access, follows the CONTRIBUTING.md enrollment runbook with the `age_recipient` from `member.yaml`.

Humans should keep the passphrase prompt (`keygen` without `--no-passphrase`) and load the key into `ssh-agent`. Agents cannot type a passphrase, so their key is protected by file permissions and by the operator's machine. That is why agents never get the `maintainer` role.

## Already have keys?

Put them in the key directory and register them:

```sh
umask 077; mkdir -p ~/.config/password123/team/<handle>
age-keygen -y ~/.config/age/password123.txt > ~/.config/password123/team/<handle>/age.pub
cp ~/.ssh/id_ed25519 ~/.ssh/id_ed25519.pub ~/.config/password123/team/<handle>/
mv ~/.config/password123/team/<handle>/id_ed25519     ~/.config/password123/team/<handle>/ssh_ed25519
mv ~/.config/password123/team/<handle>/id_ed25519.pub ~/.config/password123/team/<handle>/ssh_ed25519.pub
scripts/team.sh register <handle> --email you@example.org
```

`register` needs `age.pub`, `ssh_ed25519` and `ssh_ed25519.pub`. It reads the private SSH key only to sign `member.yaml`. A dedicated team key is better than reusing a login key (ARCHITECTURE.md §7.4).

## Rotation and revocation

- **Rotate your own keys:** generate new keys in a fresh key directory, then delete your `members/<handle>/` directory and run `scripts/team.sh register <handle> --key-dir <new dir> ...` in the same PR. The maintainer re-checks both fingerprints and re-enrolls you for payloads if you had access.
- **Revoke someone** (lost device, left the team, agent decommissioned): a maintainer runs `scripts/team.sh revoke <handle> --handle <maintainer>`. This marks the record `status: revoked`, signs it with the maintainer key, and removes the member from `allowed_signers` and `recipients.txt`. Then the maintainer follows the CONTRIBUTING.md removal runbook (`recipients/maintainers.txt`, `.sops.yaml`, `p123 rotate --all`). Revocation stops future access only. It cannot take back what the member already decrypted.

## Checksums and their limits

`team/SHA256SUMS` lists every file under `team/` plus `scripts/team.sh`, with sorted relative paths, in `sha256sum` format:

```sh
sha256sum -c team/SHA256SUMS        # from the repository root
```

**A checksum next to the file it covers only detects accidents.** Anyone who can change a key can also change its checksum. Integrity against tampering comes from three other places: the PR review on a protected `main`, the out-of-band fingerprint check, and `SHA256SUMS.sig`, verified against a maintainer key that you obtained independently:

```sh
ssh-keygen -Y verify -f team/allowed_signers -I <maintainer> -n password123-file \
  -s team/SHA256SUMS.sig < team/SHA256SUMS
```

Every enrollment PR changes `SHA256SUMS`, so the signature goes stale until a maintainer signs again. `verify` reports that as a warning, and `verify --strict` (used for releases) as an error.
