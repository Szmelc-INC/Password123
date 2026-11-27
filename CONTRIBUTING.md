# Contributing to Password123

Every change reaches `main` through a pull request with green CI: build, tests, `p123 validate`, vector verification, bundle reproducibility, gitleaks over full history, Nix and PKGBUILD checks.

## Ground rules

1. **Plaintext never enters Git.** Write challenge sources outside the checkout (for example `~/private-work/`), encrypt them, and commit only `payloads/*.age`. `.gitignore` blocks common paths, and `p123 validate` blocks plaintext payloads, forbidden paths and private keys. Treat both as a safety net, not a permission.
2. **Identities stay outside the repository.** Only public recipients go in `recipients/maintainers.txt` and `.sops.yaml`. The single exception is `demo/demo-identity.txt`, which is public by design and may open demo payloads only (CI enforces this).
3. **Never invent test vectors.** Copy them from the cited source, record the document, section, test case and URL, and mark them `verified` only after `p123 vectors verify` passes. Teaching examples use `kind: adapted`.
4. **Pin and review dependencies.** Actions are pinned to full commit SHAs; the portal bundle is built from `web/package-lock.json` and CI requires the rebuilt bundle to match the committed `docs/assets/age.js`.

## Lesson documentation style

Every lesson (`curriculum/0N-<module>/<name>.yaml`, id `0N-<name>`) explains:

1. what the primitive does, 2. what problem it solves, 3. inputs and outputs, 4. key creation and management, 5. common mistakes, 6. whether it suits new systems, 7. which standard or vector is used, and 8. how to reproduce it locally.

Use simple language first, then exact parameters. Status values: `draft`, `experimental`, `stable` (requires verified vectors), `deprecated` (historical only).

## Runbook: add a lesson

```sh
$EDITOR curriculum/02-stream-and-block/new-mode.yaml   # public fields only
$EDITOR vectors/symmetric/new-mode.yaml                 # sourced vectors with explicit encodings
p123 vectors verify --lesson 02-new-mode                # add a verifier in internal/vectors if needed
p123 validate
```

With a challenge, add `challenge:` (prompt and public inputs) and a `payload:` entry with any 64-hex placeholder digest. Then:

```sh
umask 077
$EDITOR ~/private-work/02-new-mode.json                 # matches schemas/payload.schema.json
p123 encrypt ~/private-work/02-new-mode.json --lesson 02-new-mode   # writes payloads/ and the digest
p123 decrypt 02-new-mode | head                         # test with your identity
shred -u ~/private-work/02-new-mode.json                # or your platform's secure delete
p123 validate && p123 pages build                       # the site must contain ciphertext only
```

## Runbook: join the team (keys for signing, encryption, authorization)

Every member, human or agent, registers official keypairs in [`team/`](team/README.md) with one PR: `scripts/team.sh keygen <handle>`, then `scripts/team.sh verify`. Only public keys enter the repository. Agents use [team/onboarding/AGENT-PROMPTS.md](team/onboarding/AGENT-PROMPTS.md), and [team/INTEGRATION.md](team/INTEGRATION.md) covers git signing, file signatures, age and SSH. Registration does not grant payload access: use the next runbook with the member's `age_recipient`.

## Runbook: enroll a recipient

1. Receive the new **public** recipient (`age1...` or `age1pq1...`) through a trusted channel and confirm it with its owner over a second channel.
2. `p123 recipients add age1... --comment "owner/device, YYYY-MM-DD"` (the comment is public).
3. `p123 rotate --all` with an already-enrolled identity, then update SOPS: add the key to `.sops.yaml`, run `sops updatekeys private-cli/maintainer.sops.yaml`.
4. Ask the new recipient to run `p123 decrypt <lesson>` before you merge.

Do not mix classic (`age1`) and post-quantum (`age1pq1`) recipients in one recipients file: age refuses it, because the classic key would be the weakest link.

## Runbook: remove a recipient

1. Delete the line from `recipients/maintainers.txt` and `.sops.yaml`.
2. `p123 rotate --all`, then `sops updatekeys private-cli/maintainer.sops.yaml && sops rotate -i private-cli/maintainer.sops.yaml`.
3. Test with each remaining identity.
4. If the removal is because of compromise, change the secrets themselves (regenerate challenges). Rotation cannot revoke copies the former recipient already has.

## Dependency updates

Dependabot opens weekly pull requests for actions, Go modules and the portal bundle. Any `go.mod` change also changes the Nix `vendorHash`: the `nix flake` job fails and prints `got: sha256-...`; put that value in `flake.nix`. A bundle update must keep `docs/assets/age.js` byte-identical to `npm run build`, or the rebuilt file must be committed and reviewed.

## Runbook: release

```sh
p123 validate && make test portal-test
git tag -s v0.2.0 -m v0.2.0 && git push origin v0.2.0   # release.yml builds, checksums and attests
```

Update `pkgver` in `packaging/arch/PKGBUILD` and `version` in `flake.nix` in the same pull request that prepares the tag.
