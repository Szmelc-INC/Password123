# Password123

[![ci](https://github.com/Szmelc-INC/Password123/actions/workflows/ci.yml/badge.svg)](https://github.com/Szmelc-INC/Password123/actions/workflows/ci.yml)
[![secret-scan](https://github.com/Szmelc-INC/Password123/actions/workflows/secret-scan.yml/badge.svg)](https://github.com/Szmelc-INC/Password123/actions/workflows/secret-scan.yml)
[![pages](https://github.com/Szmelc-INC/Password123/actions/workflows/deploy-pages.yml/badge.svg)](https://szmelc-inc.github.io/Password123/)
[![license: MIT](https://img.shields.io/badge/license-MIT-105c5c.svg)](LICENSE)
[![age](https://img.shields.io/badge/encryption-age%201.3-105c5c.svg)](https://github.com/FiloSottile/age)

> **A practical, zero-knowledge cryptography lab.** Public lessons. Public, verified test vectors. Encrypted challenge solutions. A local CLI and TUI. Browser-side decryption on a static GitHub Pages portal.

**Publish the explanation. Encrypt the answer. Keep the identity. Verify the code. Rotate when access changes.**

GitHub cannot make one folder of a public repository private. Password123 separates data by **content** instead of permissions. Lesson metadata and test vectors are plaintext and public. Challenge solutions are [age](https://github.com/FiloSottile/age) ciphertext that only enrolled recipients can open. GitHub stores ciphertext, never the secret, and private identities never enter the repository.

- **Portal:** <https://szmelc-inc.github.io/Password123/>
- **Specification and threat model:** [ARCHITECTURE.md](ARCHITECTURE.md)
- **Adding lessons, enrolling recipients, rotation:** [CONTRIBUTING.md](CONTRIBUTING.md)
- **Reporting and leak response:** [SECURITY.md](SECURITY.md)

## What is inside

| Layer | Where | Who can read it |
|---|---|---|
| 22 lessons in 6 modules (classical → post-quantum) | `curriculum/` | everyone |
| 34 vectors and checks (28 published vectors from RFCs, NIST, reference suites and C2SP CCTV; 3 adapted textbook examples; 1 parameter check; 2 property self-checks), all recomputed in CI | `vectors/` | everyone |
| 19 challenge solutions + 1 demo payload | `payloads/*.age` | enrolled recipients (demo: everyone) |
| Maintainer notes with visible keys and encrypted values | `private-cli/*.sops.yaml` | enrolled recipients, via SOPS |
| Static reader with in-browser age decryption | `docs/` → GitHub Pages | everyone; decryption needs your identity |

Modules: **01** Caesar, Vigenère, Playfair, Enigma M3 · **02** RC4, AES-CBC, AES-256-GCM, ChaCha20-Poly1305 · **03** RSA-OAEP, Diffie-Hellman, X25519, Ed25519 · **04** SHA-256, BLAKE3, HMAC, PBKDF2, Argon2id · **05** Shamir's Secret Sharing, WebAuthn PRF · **06** ML-KEM-768, ML-DSA-65, hybrid X25519 + ML-KEM.

Each challenge is a real attack or exercise: Caesar brute force, Vigenère with Kasiski, RC4 and GCM nonce reuse, CBC IV bit-flipping, Fermat factoring of a badly generated RSA key, a 48-bit discrete log, an X25519 low-order point, offline PIN guessing against PBKDF2 and Argon2id, and ML-KEM + AEAD.

## Quick start

```sh
git clone https://github.com/Szmelc-INC/Password123 && cd Password123
go build -o p123 ./cmd/p123        # Go 1.27+
./p123 lessons list
./p123 lessons show 02-aes-256-gcm
./p123 vectors verify              # recompute every published vector
./p123 decrypt demo-welcome -i demo/demo-identity.txt   # the public demo identity
./p123 tui                         # full-screen browser: ? help, : commands
```

Install with packages:

```sh
nix run github:Szmelc-INC/Password123 -- lessons list    # flake: package, app, devShell
(cd packaging/arch && makepkg -si)                       # Arch Linux (installs completions)
make install PREFIX=~/.local                             # binary + bash/zsh/fish completions
```

## Get access to the encrypted solutions

```sh
umask 077
age-keygen -o ~/.config/age/password123.txt    # keep this file private and backed up
age-keygen -y ~/.config/age/password123.txt    # send ONLY this public recipient to a maintainer
```

After a maintainer enrolls your recipient and rotates the payloads, decrypt locally:

```sh
p123 decrypt 03-rsa-oaep                       # uses ~/.config/age/password123.txt by default
age -d -i ~/.config/age/password123.txt payloads/03-rsa-oaep.age   # same result with the reference CLI
```

Or open the portal, paste or load your identity, choose a payload and press **Decrypt**. Decryption happens in the tab, and **Lock** clears it. For high-assurance use, prefer the CLI on a trusted machine.

## CLI reference

`p123 <command> --help` prints colored help for every command. All commands accept `--json`, `--no-color` (and honour `NO_COLOR`), `--color auto|always|never`, `--repo` and `--config`.

| Command | What it does |
|---|---|
| `lessons list [--module 02] [--status stable]` | Aligned table of lessons |
| `lessons show <id> [--raw]` | Lesson rendered as Markdown |
| `vectors list` / `vectors verify [--lesson id]` | Sources, and recomputation of every vector |
| `tui` | Lesson list, Markdown pane, `:` command palette, `?` help, `L` lock |
| `encrypt <file> --lesson <id>` / `--demo <id>` / `--out <path>` | Encrypt plaintext that lives outside the repo; updates manifest digests |
| `decrypt <lesson-id \| demo-id \| file.age> [-i identity] [-o out]` | Local decryption; refuses to write plaintext into the repo |
| `rotate --all \| --lesson <id>` | Re-encrypt payloads to the current recipients, in memory |
| `recipients list` / `recipients add <age1...>` | Enrolled public recipients |
| `validate [--skip-vectors] [--site dir]` | Every fail-closed repository check |
| `pages build` / `pages serve` / `pages verify --dir <site>` | Build, preview and check the static portal |
| `config path \| show \| init` | Config file at `~/.config/password123/config.yaml` |
| `completion bash \| zsh \| fish` | Shell completions (also shipped in `completions/`) |

Configuration precedence: flags, then environment (`P123_REPO`, `P123_IDENTITY`, `P123_CONFIG`), then the config file, then defaults.

## Verifying the portal and releases

Every Pages build is deterministic and ships a `SHA256SUMS`. Releases (`v*` tags) publish binaries, a site tarball, `SHA256SUMS` and a GitHub build-provenance attestation:

```sh
sha256sum -c SHA256SUMS
gh attestation verify password123-site_v0.1.1.tar.gz --repo Szmelc-INC/Password123
mkdir site && tar xzf password123-site_v0.1.1.tar.gz -C site && p123 pages verify --dir site
```

A checksum stored next to the file it describes only detects accidents. The attestation is the independent trust anchor. See ARCHITECTURE.md sections 9.4 and 12.

## Security model in one paragraph

GitHub and any visitor can see lesson names, file paths, ciphertext sizes, commit times and the public recipients. They cannot decrypt a payload without an enrolled private identity. A compromised Pages host could serve malicious JavaScript, so the portal is a convenience layer; the CLI is the reference path. Removing a recipient protects only future versions. The full threat model, the list of what this does **not** protect, and the operational runbooks are in [ARCHITECTURE.md](ARCHITECTURE.md).

## Development

```sh
make test            # Go tests, including fail-closed validator cases
make validate        # what CI requires before merge
make bundle          # rebuild docs/assets/age.js from the pinned typage release
make portal-test     # Go-made payloads must decrypt with the browser bundle
nix develop          # go, age, sops, gitleaks, node
```

## License

[MIT](LICENSE)
