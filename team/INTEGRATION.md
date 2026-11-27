# Integrating team keys

How to use the keys from the team register (`team/README.md`) for signing, encryption, authorization and verification. Every snippet uses only public data from `team/`, plus your own private key from `~/.config/password123/team/<handle>/`.

Throughout: `K=~/.config/password123/team/<handle>` and `H=<handle>`.

## 1. Git commit and tag signing

```sh
git config --local gpg.format ssh
git config --local user.signingkey "$K/ssh_ed25519.pub"
git config --local commit.gpgsign true
git config --local tag.gpgsign true
git config --local gpg.ssh.allowedSignersFile team/allowed_signers
```

- `user.email` must equal the `email:` in your `member.yaml`. Git matches the committer email against the principals in `allowed_signers`. If you have no email in the record, git still signs, but verification shows "no principal matched".
- Humans: `ssh-add "$K/ssh_ed25519"` once per session, so git does not ask for the passphrase on every commit.
- Upload `ssh.pub` to GitHub as a **Signing key** (Settings → SSH and GPG keys → New SSH key → Key type: Signing key), so GitHub shows "Verified". Do this with `gh ssh-key add "$K/ssh_ed25519.pub" --type signing --title "password123-$H"`. An agent's operator does this on the agent's bot account, if it has one.

Verify locally:

```sh
git log --show-signature -1
git verify-commit HEAD
git verify-tag v1.2.3
```

## 2. Signing and verifying files

Signatures use the `password123-file` namespace. A signature made for one namespace does not verify in another, so a commit signature cannot be replayed as a release signature.

```sh
scripts/team.sh sign-file dist/SHA256SUMS --handle "$H"          # writes dist/SHA256SUMS.sig
scripts/team.sh verify-file dist/SHA256SUMS --handle "$H"

# the same with ssh-keygen only (any OpenSSH 8.1+):
ssh-keygen -Y sign -f "$K/ssh_ed25519" -n password123-file dist/SHA256SUMS
ssh-keygen -Y verify -f team/allowed_signers -I "$H" -n password123-file \
  -s dist/SHA256SUMS.sig < dist/SHA256SUMS
```

Use it for release checksum files, signed manifests (ARCHITECTURE.md §12), handover notes, and anything else where "who published this" matters.

## 3. Encryption with age

To one member, or to every active member:

```sh
age -R team/members/<handle>/age.pub -o note.age note.txt
age -R team/recipients.txt -o team-note.age team-note.txt
age -d -i "$K/age.key" team-note.age
```

With p123, encrypt an ad-hoc file (not a lesson payload) to the team:

```sh
p123 encrypt notes.json --out /tmp/notes.age -R team/recipients.txt
p123 decrypt --help        # decrypt uses -i "$K/age.key"
```

**Payload access is a separate step.** Lesson payloads are encrypted to `recipients/maintainers.txt`, not to `team/recipients.txt`. A maintainer enrolls a member's `age_recipient` there with the CONTRIBUTING.md runbook:

```sh
p123 recipients add "$(cat team/members/<handle>/age.pub)" --comment "<handle> (team), $(date -u +%F)"
p123 rotate --all                                       # with an already-enrolled identity
# add the same recipient to .sops.yaml, then:
sops updatekeys private-cli/maintainer.sops.yaml
```

For SOPS, point `SOPS_AGE_KEY_FILE` at your key: `export SOPS_AGE_KEY_FILE="$K/age.key"`.

## 4. Authorization

**SSH login on a team host, without a CA**, from `allowed_signers`:

```sh
awk '!/^#/{print $3, $4, $1}' team/allowed_signers >> ~/.ssh/authorized_keys     # review before appending
```

**With the team CA (phase 2, `team/ca/README.md`)**, the host trusts one key, and access ends when a certificate expires:

```text
# /etc/ssh/sshd_config on the host
TrustedUserCAKeys /etc/ssh/password123_team_ca.pub
AuthorizedPrincipalsFile /etc/ssh/principals/%u        # lists the handles allowed per account
```

Members log in with `ssh -i "$K/ssh_ed25519" -o CertificateFile=team/members/$H/ssh-cert.pub user@host`.

**Gate a CI job or script on a team signature:** require a signed artifact and verify it against the committed register.

```sh
ssh-keygen -Y verify -f team/allowed_signers -I "$SIGNER" -n password123-file -s artifact.sig < artifact
```

The register at the commit under test is only as trustworthy as that commit. For a gate that matters, pin the expected signer fingerprints in the job, or verify against `allowed_signers` from `main`.

## 5. Checksums

```sh
sha256sum -c team/SHA256SUMS                    # files match the published sums
scripts/team.sh verify                          # sums, signatures, records, generated files
scripts/team.sh verify --strict                 # also fails on warnings: expired keys, stale SHA256SUMS.sig
```

To compare with an independent source, check the fingerprint in `member.yaml` with the owner over a second channel:

```sh
ssh-keygen -l -E sha256 -f team/members/<handle>/ssh.pub
cat team/members/<handle>/age.pub
```

## 6. CI

The `ci` workflow runs on every PR:

```sh
shellcheck -S warning scripts/team.sh scripts/team_test.sh
bash scripts/team.sh verify --no-color
bash scripts/team_test.sh          # end-to-end run with throwaway keys
```

Releases should run `scripts/team.sh verify --strict` and fail when `SHA256SUMS.sig` is stale.

## 7. Next steps (not part of v1)

- A `p123 team` subcommand (list, verify, sign, verify-file) with `--json` output and completions. It would replace the shell script with the same file formats.
- Release signing in `release.yml`: sign `dist/SHA256SUMS` with a maintainer key, or with Sigstore keyless signing that names the workflow identity.
- A move of the whole team to post-quantum `age1pq1…` recipients, as one coordinated rotation.
