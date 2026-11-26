# Password123 architecture and specification

> This is the design specification the repository implements. The short project overview, quick start and CLI reference live in [README.md](README.md). Where this document says "should", the implementation notes in brackets say how Password123 does it.

> **A practical, zero-knowledge cryptography and encryption lab.** 
>
> Public lessons and specifications. Public, verifiable test vectors. Encrypted challenge payloads. Local CLI access. Optional browser-side decryption through a static GitHub Pages portal.

**Project type:** educational reference + challenge repository  
**Storage model:** public Git repository containing public metadata and encrypted payloads  
**Primary tools:** `age`, Mozilla SOPS, Git, GitHub Actions, GitHub Pages, Gitleaks, and an optional browser-side `age` implementation  
**Default rule:** plaintext is public by design; private challenge content is encrypted before it enters Git.

---

## Table of Contents

- [1. Project Purpose](#1-project-purpose)
- [2. The Core Idea](#2-the-core-idea)
- [3. Goals and Non-Goals](#3-goals-and-non-goals)
- [4. Security Model](#4-security-model)
  - [4.1 Assets](#41-assets)
  - [4.2 Threats](#42-threats)
  - [4.3 What This Design Protects](#43-what-this-design-protects)
  - [4.4 What This Design Does Not Protect](#44-what-this-design-does-not-protect)
- [5. Repository Architecture](#5-repository-architecture)
  - [5.1 Architecture at a Glance](#51-architecture-at-a-glance)
  - [5.2 Canonical Data Model](#52-canonical-data-model)
  - [5.3 Directory Layout](#53-directory-layout)
  - [5.4 Source of Truth and Separation of Duties](#54-source-of-truth-and-separation-of-duties)
- [6. Cryptography and Storage Stack](#6-cryptography-and-storage-stack)
  - [6.1 age](#61-age)
  - [6.2 Mozilla SOPS](#62-mozilla-sops)
  - [6.3 SOPS and age Are Not the Same File Format](#63-sops-and-age-are-not-the-same-file-format)
  - [6.4 Optional Encryption and Storage Tools](#64-optional-encryption-and-storage-tools)
- [7. Key Management](#7-key-management)
  - [7.1 Key Types](#71-key-types)
  - [7.2 Generate and Store a Keypair](#72-generate-and-store-a-keypair)
  - [7.3 Enroll a Recipient](#73-enroll-a-recipient)
  - [7.4 GitHub SSH Key Enrollment](#74-github-ssh-key-enrollment)
  - [7.5 Remove a Recipient and Rotate Keys](#75-remove-a-recipient-and-rotate-keys)
  - [7.6 Backups and Key Loss](#76-backups-and-key-loss)
- [8. CLI Workflows](#8-cli-workflows)
  - [8.1 Encrypt a Payload](#81-encrypt-a-payload)
  - [8.2 Decrypt a Payload](#82-decrypt-a-payload)
  - [8.3 Edit a SOPS File](#83-edit-a-sops-file)
  - [8.4 Validate Before Commit](#84-validate-before-commit)
- [9. GitHub Pages Browser Portal](#9-github-pages-browser-portal)
  - [9.1 What the Portal Does](#91-what-the-portal-does)
  - [9.2 Browser Decryption Flow](#92-browser-decryption-flow)
  - [9.3 CSP and the `connect-src` Rule](#93-csp-and-the-connect-src-rule)
  - [9.4 SRI, Supply Chain, and Trust in the Page](#94-sri-supply-chain-and-trust-in-the-page)
  - [9.5 Private-Key Handling in the Browser](#95-private-key-handling-in-the-browser)
  - [9.6 Demo Keys](#96-demo-keys)
  - [9.7 Password-Protected Pages](#97-password-protected-pages)
- [10. GitHub and Repository Visibility](#10-github-and-repository-visibility)
- [11. Metadata Privacy](#11-metadata-privacy)
- [12. Integrity, Authenticity, and Supply-Chain Safety](#12-integrity-authenticity-and-supply-chain-safety)
- [13. CI, Secret Scanning, and GitHub Actions](#13-ci-secret-scanning-and-github-actions)
- [14. Key Rotation, Revocation, and History Cleanup](#14-key-rotation-revocation-and-history-cleanup)
- [15. Cryptography Curriculum](#15-cryptography-curriculum)
- [16. Test Vectors and Challenge Design](#16-test-vectors-and-challenge-design)
- [17. Repository Conventions](#17-repository-conventions)
- [18. Operational Runbooks](#18-operational-runbooks)
- [19. Architecture Decision Matrix](#19-architecture-decision-matrix)
- [20. Extended Tool and Protocol Catalogue](#20-extended-tool-and-protocol-catalogue)
- [21. Known Limits and Common Mistakes](#21-known-limits-and-common-mistakes)
- [22. Roadmap](#22-roadmap)
- [23. Glossary](#23-glossary)
- [24. References](#24-references)

---

## 1. Project Purpose

Password123 is designed to become a practical, zero-knowledge cryptography reference that can be used in two ways:

1. **From the terminal:** users inspect public lesson files and decrypt enrolled challenge payloads with `age` or SOPS.
2. **From a browser:** users browse a public GitHub Pages site and decrypt selected `.age` payloads locally, using a private identity they provide to the page.

The repository is both a learning resource and a working demonstration of modern secret storage. It explains classical ciphers, symmetric encryption, public-key cryptography, hashes, password-based key derivation, threshold schemes, hardware-backed credentials, and post-quantum algorithms.

Every lesson should have a public description and a clear source or specification. A lesson may also have a private challenge payload containing an answer, solution, secret input, or instructor note. That payload is encrypted locally before it is committed.

The repository does **not** try to hide the existence of its lessons. It deliberately publishes useful metadata. It tries to keep selected payload contents confidential while still allowing authorized readers to decrypt them.

## 2. The Core Idea

GitHub repository visibility applies to the repository as a whole. GitHub does not provide a normal folder-level visibility setting where one directory is public and another directory is private.

The design therefore separates data by **content**, not by GitHub permissions:

| Layer | Stored on GitHub | Who can read it? | Purpose |
|---|---|---|---|
| Public lesson manifest | Plaintext YAML or JSON | Everyone | Names, descriptions, standards, public hints, and payload pointers |
| Public test vectors | Plaintext | Everyone | Reproducible, published examples used for learning and verification |
| Encrypted challenge payloads | Binary or armored `.age` files | Everyone can download; enrolled keyholders can decrypt | Solutions, private challenge inputs, and instructor notes |
| SOPS-managed files | Structured files with selected values encrypted | Everyone can see the public structure; enrolled identities can decrypt selected values | CLI-managed secrets and structured private metadata |
| Browser portal | Static HTML, CSS, and JavaScript | Everyone | Browse lessons and decrypt selected payloads locally |
| Private identities | Stored outside the repository | Key owner only | Decrypt payloads |

The important difference is that **GitHub stores ciphertext, not the original secret**. The private identity stays with the person who owns it.

### Recommended default

- Use a native `age` keypair for each person or device.
- Encrypt challenge payloads into individual `.age` files before committing them.
- Keep lesson metadata and published test vectors in plaintext.
- Use SOPS for structured CLI-managed files that benefit from visible keys and encrypted values.
- Build the Pages portal from public manifests and encrypted `.age` payloads only.
- Never give a public GitHub Actions workflow access to private decryption identities.

This creates one clear path for each job instead of forcing every tool to do everything.

## 3. Goals and Non-Goals

### Goals

- Keep public pointers, lesson names, algorithm descriptions, and standards readable on GitHub.
- Keep selected challenge contents encrypted at rest in the public repository.
- Allow multiple enrolled recipients to decrypt the same payload.
- Support both CLI decryption and an optional static browser decryptor.
- Use public and official standards-based test vectors wherever possible.
- Make the repository reproducible, auditable, and easy to maintain.
- Detect accidental plaintext secret commits before they become part of the repository history.
- Explain security trade-offs in plain language.

### Non-Goals

- Hiding the repository, its commit history, its file names, or all activity metadata from GitHub.
- Making public ciphertext inaccessible to people who can download the repository.
- Providing server-side authentication from a plain public GitHub Pages site.
- Recovering a lost private key without a separate recovery design.
- Revoking copies of ciphertext that an authorized user already downloaded.
- Treating a browser page as a trusted hardware security boundary.
- Replacing a professional password manager, enterprise secrets platform, or audited cryptographic product.

## 4. Security Model

Security claims must always name the threat they cover. “Zero-knowledge” is not a magic label that makes every layer safe.

### 4.1 Assets

The repository protects the confidentiality of selected content, such as:

- challenge answers and flags;
- private test inputs that are not published standards vectors;
- instructor notes and unpublished solutions;
- private sample configuration values;
- sensitive data used by a local demonstration;
- private key material used by maintainers.

Public data includes lesson titles, algorithm names, public references, public test vectors, difficulty levels, public hints, and payload locations.

### 4.2 Threats

The design considers:

- a person downloading the public repository without an enrolled private key;
- GitHub or another storage host reading the repository contents;
- accidental commits of plaintext secrets;
- a contributor losing control of an enrolled key;
- a malicious or compromised dependency in the browser portal;
- a compromised GitHub Actions workflow or third-party action;
- offline password guessing against publicly downloadable ciphertext;
- metadata analysis based on file sizes, names, timestamps, and update patterns;
- malicious changes to public manifests or encrypted payloads.

### 4.3 What This Design Protects

When encryption is correctly implemented and the private identity remains private:

- A storage provider can read the public manifest and download encrypted payloads, but cannot decrypt those payloads using only the ciphertext and recipient public keys.
- A person who is not an enrolled recipient cannot decrypt an `age` payload merely by knowing its path.
- SOPS can encrypt selected values while leaving the surrounding structure visible.
- A browser can decrypt an `age` payload locally without sending the private identity to a server, provided the client code is trustworthy and does not transmit it.
- Authenticated encryption detects changes to the encrypted message instead of returning an accepted, modified plaintext.

### 4.4 What This Design Does Not Protect

Encryption does not hide everything:

- GitHub can see repository names, file paths, public metadata, ciphertext sizes, commit times, branches, and download activity available to the hosting platform.
- Public recipients and public keys are not secrets. Their use may still reveal relationships or membership.
- A person with a valid private identity can copy decrypted content and share it.
- Removing a recipient does not erase plaintext or ciphertext already downloaded by that recipient.
- A compromised computer, browser, browser extension, operating system, or clipboard can expose a key or decrypted content.
- A compromised Pages site can serve malicious JavaScript. That code may read a key entered into the page.
- CSP and SRI reduce attack paths, but they do not prove that the HTML and JavaScript served by the host are honest.
- A public ciphertext can be tested offline if it uses a password or passphrase. Encryption does not stop weak passwords from being guessed.

**Important:** the browser portal is a convenience layer. For high-assurance decryption, use the CLI on a trusted machine, or download the site and verify its release signature and hashes before entering a private identity.

## 5. Repository Architecture

### 5.1 Architecture at a Glance

```text
                    PUBLIC GITHUB REPOSITORY
              ┌─────────────────────────────────┐
              │                                 │
              │  curriculum/                    │
              │    public lesson manifests      │
              │                                 │
              │  vectors/                       │
              │    published test vectors       │
              │                                 │
              │  payloads/                      │
              │    *.age encrypted content      │
              │                                 │
              │  docs/                          │
              │    static browser portal        │
              │                                 │
              │  .sops.yaml                     │
              │  GitHub Actions                 │
              └───────────────┬─────────────────┘
                              │
             ┌────────────────┴─────────────────┐
             │                                  │
             ▼                                  ▼
      CLI / local tools                  GitHub Pages
      age + SOPS                         HTML + CSS + JS
             │                                  │
             │                                  │
      local private identity             user enters or selects
      stays on local machine              local private identity
             │                                  │
             ▼                                  ▼
      decrypt .age or SOPS                decrypt .age in browser
      content locally                     in browser memory
             │                                  │
             ▼                                  ▼
      plaintext shown locally             plaintext rendered locally
```

GitHub Actions validates public data, scans for accidental secrets, builds the static portal, and deploys the static files. It does not decrypt private challenge content.

### 5.2 Canonical Data Model

The public manifest is the index. The encrypted payload is the private content. Each challenge has one stable identifier used across files and tools.

A public lesson entry should contain only safe, intentionally public fields:

```yaml
schema_version: 1
lessons:
  - id: "02-aes-256-gcm"
    title: "AES-256-GCM"
    category: "symmetric-aead"
    difficulty: "intermediate"
    status: "stable"
    summary: "Authenticated encryption with associated data."
    references:
      - name: "NIST SP 800-38D"
        url: "https://csrc.nist.gov/pubs/sp/800/38/d/final"
    public_inputs:
      nonce_hex: "<published nonce>"
      aad_hex: "<published AAD>"
      ciphertext_hex: "<published ciphertext>"
      tag_hex: "<published authentication tag>"
    payload:
      path: "payloads/02-aes-256-gcm.age"
      format: "application/json"
      sha256: "<hash of the encrypted file>"
```

The values in this example are placeholders. Real test vectors must be copied from the cited source and verified by an automated test before they are labelled as verified.

The encrypted payload may contain a private challenge or solution document:

```json
{
  "schema_version": 1,
  "lesson_id": "02-aes-256-gcm",
  "kind": "challenge-solution",
  "solution": "<private solution>",
  "explanation": "<private explanation>",
  "verification": {
    "expected_result": "<expected result>",
    "test_command": "<local verification command>"
  }
}
```

The JSON above is a schema example, not a real secret. It should be created locally, encrypted, and only then copied into the tracked `payloads/` directory.

### 5.3 Directory Layout

The implemented layout follows the recommendation. The reference tooling is one Go binary, `p123`; the shell scripts are thin wrappers kept for the names this specification uses, and Go tests replace the suggested Python tests.

```text
Password123/
├── README.md  ARCHITECTURE.md  CONTRIBUTING.md  SECURITY.md  LICENSE
├── .gitignore  .gitattributes  .sops.yaml  .gitleaks.toml
├── .github/
│   ├── dependabot.yml                 reviewed dependency and action updates
│   └── workflows/
│       ├── ci.yml                     build, tests, validate, vectors, bundle, nix, PKGBUILD
│       ├── secret-scan.yml            pinned, checksum-verified gitleaks over full history
│       ├── deploy-pages.yml           static portal only; no secrets, no decryption
│       └── release.yml                binaries, site tarball, SHA256SUMS, build provenance
├── schemas/                           lesson, vectors, payload and demo JSON Schemas
├── curriculum/0N-<module>/<name>.yaml one public lesson per file (22 lessons, 6 modules)
├── vectors/<group>/<name>.yaml        sourced test vectors, each with a named encoding
├── payloads/*.age                     age ciphertext only (challenge solutions + demo)
├── recipients/maintainers.txt         enrolled PUBLIC recipients
├── demo/                              public demo identity + demo manifest (demo-only)
├── private-cli/maintainer.sops.yaml   SOPS: visible keys, encrypted selected values
├── docs/                              portal source: index.html, app.js, styles.css, assets/age.js
├── web/                               pinned typage bundle build + Node cross-implementation tests
├── cmd/p123/  internal/               CLI, TUI, verifiers, validator, Pages builder
├── scripts/                           enroll-recipient, encrypt-payload, validate-manifest,
│                                      verify-vectors, build-pages, rotate-recipients
├── completions/                       bash, zsh and fish completions (generated, checked in CI)
├── packaging/arch/PKGBUILD            Arch package
└── flake.nix                          Nix package, app and dev shell
```

The separation rules below are enforced by `p123 validate`, which fails closed.

### 5.4 Source of Truth and Separation of Duties

Do not store the same secret in several encrypted formats just because several tools are available.

| Data | Canonical location | Encryption | Browser support |
|---|---|---|---|
| Public lesson metadata | `curriculum/` | None | Directly readable |
| Official published test vectors | `vectors/` | None | Directly readable |
| Private challenge solutions | Temporary local source, then `payloads/*.age` | age | Yes, with a compatible browser implementation |
| Structured CLI-only private settings | `private-cli/*.sops.yaml` | SOPS + age recipients | No direct SOPS browser decryption in the default design |
| Private recipient identities | Outside the repository | OS file permissions and optional encrypted backup | Loaded locally only when required |
| Static application code | `docs/` | Not encrypted | Public; must be verified and trusted |

SOPS and raw age files solve related but different problems. A SOPS file uses its own metadata and data-key wrapping structure. It is not just a raw `.age` file with a YAML extension. The Pages portal should decrypt raw `.age` payloads unless a dedicated, reviewed SOPS-compatible browser implementation is deliberately added.

## 6. Cryptography and Storage Stack

### 6.1 age

[age](https://github.com/FiloSottile/age) is the default file-encryption tool.

It is a small file format and CLI with explicit recipients and identities. It supports multi-recipient encryption: one encrypted file can be opened by any one of the enrolled recipients. Every recipient uses their own private identity to recover the same content.

Main key forms:

| Type | Example prefix | Purpose |
|---|---|---|
| Native public recipient | `age1...` | Encrypt to a recipient |
| Native private identity | `AGE-SECRET-KEY-1...` | Decrypt with a classic age identity |
| Hybrid post-quantum public recipient | `age1pq1...` | Encrypt to an age hybrid post-quantum recipient |
| Hybrid post-quantum identity | `AGE-SECRET-KEY-PQ-1...` | Decrypt with the matching hybrid identity |
| SSH public key | `ssh-ed25519` or `ssh-rsa` | Compatibility path for an existing SSH key |

Prefer native age identities for new deployments. SSH recipients are a compatibility option, not the best default for long-lived secret storage. The age documentation notes that SSH recipient support has different cryptographic and privacy properties; SSH keys may be rotated or removed as authentication keys, and hardware-held SSH keys and `ssh-agent` are not supported by the normal SSH identity path.

`age` v1.3.0 introduced native post-quantum recipients based on a hybrid ML-KEM-768 construction. Older age clients cannot be assumed to understand these recipient types. Pin compatible versions and test all CLI, SOPS, and browser implementations before using post-quantum identities in real data.

### 6.2 Mozilla SOPS

[SOPS](https://github.com/getsops/sops) is used when a structured file needs to remain readable while selected values are encrypted.

It supports YAML, JSON, ENV, INI, and binary files, with backends including age, PGP/GPG, and cloud key-management systems.

A SOPS file can leave field names and public values readable while encrypting selected values. The `encrypted_regex` rule chooses keys whose values should be encrypted.

Example `.sops.yaml`:

```yaml
creation_rules:
  - path_regex: '^private-cli/.*\.sops\.yaml$'
    encrypted_regex: '^(secret_key|answer|private_notes)$'
    age: >-
      age1REPLACE_WITH_MAINTAINER_PUBLIC_KEY,
      age1REPLACE_WITH_REVIEWER_PUBLIC_KEY
```

Replace the example recipients with real public keys. Never put a private identity in `.sops.yaml`.

Example source file before encryption:

```yaml
name: "Maintainer settings"
public_description: "Visible description"
secret_key: "this value is encrypted by SOPS"
answer: "this value is encrypted by SOPS"
private_notes: "this value is encrypted by SOPS"
```

After `sops --encrypt --in-place`, the selected values become SOPS ciphertext. The field names and unselected fields remain visible.

The example is intentionally a CLI-managed file. The browser portal does not parse it as an `age` payload.

### 6.3 SOPS and age Are Not the Same File Format

Use these formats for different jobs:

- **Raw `.age` payload:** a complete file encrypted directly with age. This is the default format for challenge payloads and the format the browser portal reads.
- **SOPS YAML/JSON/ENV file:** a structured document with SOPS metadata and selected encrypted fields. This is for local CLI workflows and maintainer configuration.

Do not take a SOPS `ENC[...]` value and pass it to the age browser decryptor. The browser decryptor expects a valid age file, not a SOPS document.

SOPS 3.12.0 added support for hybrid post-quantum age identities. Use SOPS 3.12 or later for that feature, and use a compatible age version. For ordinary classic age identities, still pin and test a known version instead of relying on whatever happens to be installed on a machine.

### 6.4 Optional Encryption and Storage Tools

The repository should have one default path and document alternatives instead of maintaining many competing implementations.

| Tool or format | What it does | Recommended place in this project |
|---|---|---|
| GPG / OpenPGP | Public-key encryption and signing | Alternative recipient system; useful for existing GPG users and git-crypt access |
| Cloud KMS | Stores or unwraps keys through AWS, Google Cloud, or Azure services | Optional team key backend; not required for the local-first design |
| git-crypt | Encrypts selected tracked file contents through Git clean/smudge filters | Alternative for teams that want transparent whole-file encryption in a working tree |
| Transcrypt | Git clean/smudge filters using symmetric encryption | Lightweight alternative for selected paths; manage the shared key carefully |
| git-remote-gcrypt | Encrypts Git repository objects for an encrypted remote | Alternative when repository trees, paths, and history need broader protection |
| Pass / Gopass | Directory-based password stores using GPG or age | Good local companion for managing secrets; not a replacement for the lesson manifest |
| OpenPGP.js | OpenPGP in JavaScript | Browser option only if OpenPGP payloads are required and the implementation is reviewed |
| StatiCrypt / PageCrypt | Wraps static HTML in a browser password prompt | Optional shared-passphrase pages; not the default multi-recipient model |
| WebCrypto API | Browser APIs for supported cryptographic operations | Useful for AES-GCM and KDF demonstrations; does not implement every format by itself |
| Crypt4GH | Encrypted container format with recipient handling and random-access use cases | Specialist option for large or seekable files, not ordinary lesson payloads |
| Bitnami Sealed Secrets / External Secrets Operator | Kubernetes-oriented secret delivery and controller workflows | Relevant to deployment examples, not to a static Pages-only vault |
| HashiCorp Vault / OpenBao | Central secrets systems and transit/key-management services | Optional external service; adds a new trusted component |

## 7. Key Management

Key management is part of the design, not an afterthought. Encryption is only as useful as the private identities that unlock it.

### 7.1 Key Types

- **Recipient/public key:** safe to share with the person who encrypts data for you.
- **Identity/private key:** must remain private and should be stored outside Git.
- **Recovery copy:** a separately protected backup for a key owner or approved recovery process.
- **Demo identity:** intentionally public and only allowed to decrypt demo-only payloads.

Use a separate key per person, and preferably a separate key per device when practical. That makes one device easier to revoke without changing every other device's identity.

### 7.2 Generate and Store a Keypair

Classic age identity:

```bash
umask 077
mkdir -p "$HOME/.config/age"
age-keygen -o "$HOME/.config/age/password123.txt"
```

Post-quantum hybrid identity, with a compatible age version:

```bash
umask 077
age-keygen -pq -o "$HOME/.config/age/password123-pq.txt"
```

The command prints the public recipient. Share only the public recipient. Keep the private identity file out of the repository and protect it with operating-system permissions and an appropriate backup method.

For SOPS, the default age key location is commonly under the user's SOPS configuration directory. Set the location explicitly when needed:

```bash
export SOPS_AGE_KEY_FILE="$HOME/.config/sops/age/keys.txt"
```

The environment variable should point to a private key file outside the repository.

### 7.3 Enroll a Recipient

1. Receive the new public recipient through a trusted channel.
2. Confirm its fingerprint or full key string with the owner through a second channel.
3. Add the recipient to the appropriate public recipient configuration.
4. Re-encrypt affected payloads for the new recipient.
5. Test decryption with the new recipient's private identity before publishing.
6. Record the enrollment date and key status in the maintainer's access register. Do not put private identities in that register.

For SOPS-managed files, update the recipient configuration and run:

```bash
sops updatekeys private-cli/maintainer.sops.yaml
```

`updatekeys` updates the recipient information in the encrypted file. It does not, by itself, create a new data-encryption key. Rotation is a separate operation.

### 7.4 GitHub SSH Key Enrollment

`age` can use supported SSH public keys as recipients. This makes it possible to encrypt a payload for public SSH keys listed on a GitHub profile:

```bash
GITHUB_USER="example-user"
curl -fsSL "https://github.com/${GITHUB_USER}.keys" \
  | age -R - -o "payloads/example-user-vault.age" ./private-payload.bin
```

This is a convenience path. It is not the preferred long-term key-management model.

Important limits:

- The user must retain the matching SSH private key. Removing the public key from GitHub does not make an already downloaded ciphertext disappear.
- SSH keys used only for login may be rotated or deleted more often than a dedicated encryption identity.
- The age SSH path does not use `ssh-agent` as a decryption identity source.
- Hardware-backed SSH keys such as some YubiKey configurations cannot be used through the normal age SSH private-key-file path.
- SSH recipient support includes a key identifier in the encrypted file, so it should not be treated as anonymous recipient encryption.
- Use native age keys for new recipients, and test SOPS compatibility if SSH recipients are used in SOPS-managed files.

### 7.5 Remove a Recipient and Rotate Keys

For a SOPS-managed file:

1. Remove the old public recipient from `.sops.yaml`.
2. Update the file's recipient metadata with `sops updatekeys`.
3. Rotate the data key with `sops rotate -i <file>`.
4. Commit the new encrypted file and updated recipient configuration.
5. Review the resulting file and test it with a remaining authorized identity.

For a raw `.age` payload:

1. Decrypt the payload in a trusted local environment using an authorized identity.
2. Encrypt the plaintext again to the remaining recipients.
3. Replace the old `.age` file and update its public SHA-256 digest in the manifest.
4. Test the new payload with each remaining recipient.
5. Remove any temporary plaintext file securely according to the operating system and storage medium in use.

Rotation protects future versions. It cannot revoke old plaintext, old keys, or ciphertext copies that a former recipient already has. Rewriting Git history also cannot erase copies held by forks, clones, caches, or backups.

### 7.6 Backups and Key Loss

If every copy of a private identity is lost, the matching ciphertext may be permanently unrecoverable. There is no magic administrator override in age.

Use a written recovery policy:

- keep encrypted offline backups of important identities;
- store recovery copies separately from the GitHub account;
- use a second trusted recipient for critical maintainer payloads where appropriate;
- test recovery on a clean machine;
- document who is allowed to hold recovery keys;
- do not place all recovery shares or identities in the same public repository.

## 8. CLI Workflows

The CLI is the reference implementation and the preferred high-assurance path.

### 8.1 Encrypt a Payload

Create the plaintext outside the tracked repository. Set a restrictive umask, encrypt it, then move only the ciphertext into `payloads/`.

```bash
umask 077
age \
  -r age1RECIPIENT_ONE \
  -r age1RECIPIENT_TWO \
  -o payloads/02-aes-256-gcm.age \
  /path/outside/repository/02-aes-256-gcm.json
```

For a binary file, the command is the same. For a text-friendly armored output, use age's armor option and adopt one consistent extension such as `.age.txt`.

Update the manifest's SHA-256 digest after encryption. The digest is for the encrypted file, not for the plaintext.

Do not create the plaintext inside the repository and rely only on encrypting it later. It may be staged, backed up, indexed, or committed by mistake.

### 8.2 Decrypt a Payload

```bash
age -d \
  -i "$HOME/.config/age/password123.txt" \
  payloads/02-aes-256-gcm.age \
  > /tmp/02-aes-256-gcm.json
```

Open the output locally and delete it when finished. Do not print sensitive data into shared CI logs, terminal recordings, shell transcripts, or support tickets.

### 8.3 Edit a SOPS File

SOPS can open supported structured files in an editor and re-encrypt them when saved:

```bash
SOPS_AGE_KEY_FILE="$HOME/.config/sops/age/keys.txt" \
  sops private-cli/maintainer.sops.yaml
```

To decrypt to standard output for a one-time local review:

```bash
SOPS_AGE_KEY_FILE="$HOME/.config/sops/age/keys.txt" \
  sops --decrypt private-cli/maintainer.sops.yaml
```

Do not redirect decrypted output into a tracked path.

### 8.4 Validate Before Commit

A local validation step should check:

- every manifest ID is unique;
- every referenced public file exists;
- every payload path points to an encrypted `.age` file;
- every encrypted payload hash matches the manifest;
- public metadata contains no private identity format;
- published test vectors match their named source;
- the browser bundle contains no private maintainer identity;
- no plaintext challenge source directory is tracked;
- all JSON/YAML files pass schema and syntax validation.

The validation scripts should fail closed. A missing payload, mismatched digest, invalid schema, or unknown lesson status should stop the build.

## 9. GitHub Pages Browser Portal

GitHub Pages can host the public browser interface because it serves static HTML, CSS, and JavaScript. The portal does not need a server-side database or a server-side decryption endpoint.

### 9.1 What the Portal Does

The portal should:

- list public lessons and their references;
- show public examples and published test vectors;
- show whether a lesson has an encrypted challenge payload;
- let a user provide a local age identity;
- decrypt a selected `.age` payload in the browser;
- render the plaintext locally;
- clear the key input and decrypted output when the user locks the page.

The portal should be a **reader**, not the maintainer's production encryption console. Key generation and payload creation belong in local CLI tooling or a separate offline maintainer tool.

### 9.2 Browser Decryption Flow

A suitable implementation path is [Typage](https://github.com/FiloSottile/typage), the TypeScript age implementation documented for browsers and other JavaScript runtimes.

The browser build should:

1. Bundle a pinned, reviewed age implementation into the repository.
2. Load public manifest data from the site itself.
3. Load the encrypted payload from the same site or from a generated local payload bundle.
4. Accept a private identity through a file picker or password-style text input.
5. Keep the identity in application memory only by default.
6. Decrypt the selected file locally.
7. Display plaintext using safe text rendering such as `textContent`, not unsanitized HTML insertion.
8. Clear visible plaintext and drop references to the identity when the user locks the page.

Do not assume that every age implementation supports every recipient type. If the project uses hybrid post-quantum identities, test the exact pinned browser library against the exact CLI version and official test vectors. A working CLI implementation does not prove browser support.

### 9.3 CSP and the `connect-src` Rule

The earlier draft proposed `connect-src 'none'` while also loading separate encrypted blobs with `fetch()`. Those two choices conflict.

The Content Security Policy `connect-src` directive controls APIs such as `fetch()`, `XMLHttpRequest`, WebSocket, EventSource, and `sendBeacon`. With `connect-src 'none'`, a page cannot use `fetch()` to read a local `.age` file from the same origin either.

Choose one of these two designs:

**Profile A: strict no-connection mode**

- Generate `docs/generated/payloads.js` during the local or CI build.
- Embed the encrypted payload bytes as static data in that local JavaScript bundle.
- Load only same-origin scripts, styles, and assets.
- Use `connect-src 'none'`.
- Do not use `fetch()` for payloads.

**Profile B: separate encrypted payload files**

- Keep each `.age` payload as a separate static file.
- Fetch only same-origin payload paths.
- Use `connect-src 'self'`.
- Do not allow remote API endpoints or external CDNs.

Profile B is easier to maintain as the number of challenges grows. Profile A has a stricter connection policy but creates a generated payload bundle that may become large.

A starting policy for a self-hosted static app can look like this:

```html
<meta http-equiv="Content-Security-Policy"
      content="default-src 'none';
               script-src 'self';
               style-src 'self';
               img-src 'self' data:;
               font-src 'self';
               connect-src 'self';
               object-src 'none';
               base-uri 'none';
               form-action 'none'">
```

Use `connect-src 'none'` for Profile A. Use `connect-src 'self'` for Profile B. Keep scripts and styles external to avoid allowing inline code. Do not load scripts, fonts, analytics, or crypto packages from a CDN.

This policy is a starting point, not a complete proof of isolation. CSP directives have different support rules, and a policy delivered through a `<meta>` element cannot express every directive that an HTTP response header can. For example, `frame-ancestors` and `sandbox` are not supported in a meta-delivered policy. GitHub Pages does not give a project full control over arbitrary response headers in the same way as a server you operate.

Also, `connect-src 'none'` only blocks the connection APIs covered by that directive. It does not prove that all possible forms of browser communication are impossible, or that the JavaScript itself is trustworthy.

### 9.4 SRI, Supply Chain, and Trust in the Page

Subresource Integrity (SRI) lets a page declare a cryptographic hash for a referenced script or style file. The browser rejects a resource whose contents do not match the declared hash.

Use SRI for resources where it is supported and useful, but understand its boundary:

- SRI can protect a referenced asset from unexpected changes when the HTML containing the expected hash is trusted.
- SRI does not protect the root HTML document if the hosting origin can replace both the HTML and its script references.
- SRI does not prove that the pinned code is safe; it only proves that the loaded bytes match the pinned hash.
- A hash stored beside the file it describes is not an independent trust anchor.

For stronger verification:

- publish signed release checksums for the portal bundle;
- sign release artifacts with a separately controlled signing identity;
- document how to verify the signature and digest offline;
- pin third-party build actions to full commit SHAs;
- keep all browser dependencies local and version-locked;
- review dependency updates before rebuilding the public portal;
- consider reproducible builds so another machine can compare the output.

A visitor who needs strong assurance should download the portal, verify the signature or digest through a trusted channel, inspect or build it locally, and then use it offline. The host that serves the page is still part of the code-delivery trust boundary.

### 9.5 Private-Key Handling in the Browser

Default to memory-only key handling.

- Do not put private identities in `localStorage`.
- Do not put private identities in URL query strings or URL fragments.
- Do not log private identities, decrypted content, or raw exception objects that may contain sensitive values.
- Do not send private identities to analytics, crash reporting, remote APIs, or telemetry.
- Do not persist a key to `sessionStorage` by default. It survives page reloads in the tab and is readable by same-origin scripts.
- Provide a clear lock action that clears the input and hides decrypted content.
- Avoid retaining decrypted plaintext in the DOM when it is no longer needed.
- Warn that JavaScript cannot guarantee secure zeroization of every memory copy because strings and buffers may be copied or retained by the runtime.

A browser extension, malicious same-origin script, compromised browser, or malware on the machine can still access data while it is being used. For very sensitive content, use the CLI.

### 9.6 Demo Keys

A demo keypair is useful for teaching the flow, but it must be completely separate from real challenge access.

The demo private identity may be published only if:

- it decrypts one or more clearly labelled demo payloads;
- those payloads contain no real private challenge answers or personal data;
- the page explains that anyone can use the demo identity;
- no real maintainer payload is encrypted to the demo recipient.

Never embed a private identity in JavaScript if that identity can decrypt real content. Anyone can inspect the page source.

### 9.7 Password-Protected Pages

Tools such as [StatiCrypt](https://github.com/robinmoisson/staticrypt) and [PageCrypt](https://github.com/Greenheart/pagecrypt) can wrap static HTML in a browser-side password prompt. Browser WebCrypto can perform the decryption, and PBKDF2 can slow password guessing.

This is a different model from per-user public-key enrollment:

| Model | Main benefit | Main limitation |
|---|---|---|
| Shared passphrase | Simple user experience | One shared secret; offline guessing is possible against public ciphertext |
| Individual age identities | Separate recipients and easier per-recipient enrollment | Users must manage private identity files |
| WebAuthn PRF | Can derive application key material from a credential-bound PRF | Browser/authenticator support and recovery design must be tested |

A public encrypted page can be downloaded and attacked offline. Use a long, unique, high-entropy passphrase. PBKDF2 is supported by WebCrypto, but its settings must be chosen carefully. A browser implementation of Argon2id requires a suitable additional implementation, usually WebAssembly; it is not a native WebCrypto algorithm.

A password prompt is not server-side access control. It only protects the encrypted content from readers who do not know the passphrase.

## 10. GitHub and Repository Visibility

A GitHub repository is public, private, or internal where supported by the account type. Visibility is not normally assigned to individual directories.

Common options:

| Requirement | Approach | Notes |
|---|---|---|
| Public source and encrypted private contents | Public repo + age/SOPS | Recommended for this project |
| Separate permissions for source and private files | Public repo + private repo submodule | Public users see the submodule pointer but cannot read the private repository |
| Keep all source private | Private repository | Access applies to the repository, not individual folders |
| Hide only local credentials | `.gitignore` + environment variables | Works only before files are tracked or committed |
| Publish sanitized configuration | `.env.example` | Contains names and placeholders, never real secrets |
| Publish public binary releases from private source | Separate public release repository or public package channel | A release attached to a private repository is not automatically public |
| Private GitHub Pages site for an organization | GitHub Enterprise Cloud private Pages | Site access is limited to people with repository read access; it does not make the HTML cryptographically opaque to GitHub |

GitHub Pages is available for public repositories on GitHub Free. Pages from private repositories are available on supported paid plans, but private publication and access control for a Pages site require GitHub Enterprise Cloud. That feature is useful for internal documentation, but it is not the same as zero-knowledge encryption.

For this project, the public static site should contain only public manifests, public test vectors, encrypted payloads, and public JavaScript. No real plaintext solution should be included in the Pages build output.

## 11. Metadata Privacy

Encryption hides payload contents. It does not automatically hide information around the payload.

A public Git repository may reveal:

- the number of encrypted payloads;
- file names and directory structure;
- approximate ciphertext sizes;
- commit and release timestamps;
- how often a lesson changes;
- recipient public keys if they are committed in configuration;
- the names and relationships of public lesson entries;
- which files a visitor downloads, depending on the host's logs and other infrastructure.

Possible mitigations, ordered from simple to complex:

1. Use opaque payload IDs instead of meaningful secret names.
2. Keep sensitive context out of file names, commit messages, and public metadata.
3. Batch updates where practical instead of publishing every private change immediately.
4. Pad payloads to fixed size bands and use fixed-size chunks if payload length itself is sensitive.
5. Add decoy entries only when the added complexity is justified and the decoys do not mislead maintainers.
6. Store encrypted blobs outside the main Git history when deletion or replacement is important, while remembering that public copies can still be retained.
7. Consider IPFS or another content-addressed store for immutable encrypted blobs. A content identifier is a hash of the content, not an access-control mechanism.
8. Consider Oblivious HTTP (OHTTP) or Private Information Retrieval (PIR) only if hiding which pointer a user requests is a real requirement and an appropriate relay or server design is available.
9. Consider Tor onion mirroring for users who need a separate access path with less direct IP exposure. It does not hide the content of public ciphertext or fix a compromised client.

Do not add padding, decoys, OHTTP, PIR, Tor, or decentralized storage merely to make the architecture look advanced. Add them only when the threat model needs them.

## 12. Integrity, Authenticity, and Supply-Chain Safety

Confidentiality and integrity are different properties.

- Authenticated encryption, such as AES-GCM or ChaCha20-Poly1305, detects changes to ciphertext and associated data.
- `age` protects the encrypted file format and detects malformed or modified encrypted data.
- A SHA-256 digest in the public manifest detects accidental file mismatch when compared against a trusted manifest.
- A digital signature lets a user verify that a trusted publisher signed a manifest or release.
- TUF provides a framework for signed metadata, versioning, expiry, and delegated trust.
- Sigstore can provide signing and identity-related verification for build and release artifacts.

A checksum stored next to a file in the same public repository is useful for accidental corruption, but it is not enough to defeat a malicious party who can change both the file and the checksum. For that, the verification key or signature trust root must be obtained independently.

For release metadata, consider a signed manifest containing:

- manifest version;
- release version;
- payload paths;
- payload SHA-256 digests;
- browser bundle digest;
- build/source revision;
- signature and signing identity information.

Keep signing keys separate from age decryption identities. Encryption keys answer “who can read this?” Signing keys answer “who published this?”

## 13. CI, Secret Scanning, and GitHub Actions

GitHub Actions should validate the public repository, not decrypt its private content.

Recommended workflows:

| Workflow | Trigger | Work |
|---|---|---|
| `ci.yml` | Pull request and push | Validate YAML/JSON, schemas, manifest references, hashes, public vectors, and portal build |
| `secret-scan.yml` | Pull request and push | Run Gitleaks over the relevant Git history and working tree |
| `deploy-pages.yml` | Approved push to the deployment branch | Build and deploy only the static public site |

### CI rules

- Do not expose age private identities to pull-request workflows.
- Do not store maintainer private identities as GitHub Actions secrets for ordinary Pages deployment.
- Do not decrypt real challenge payloads inside a public build job.
- Use the minimum required `GITHUB_TOKEN` permissions for each job.
- Pin third-party GitHub Actions to full commit SHAs and review updates deliberately.
- Keep deployment permissions separate from validation permissions.
- Do not run untrusted pull-request code in a job that has access to secrets.
- Keep build logs free of plaintext challenge content.
- Run dependency and action updates through review rather than silently following mutable tags.

### Gitleaks and TruffleHog

Use [Gitleaks](https://github.com/gitleaks/gitleaks) locally and in CI. [TruffleHog](https://github.com/trufflesecurity/trufflehog) is another option for secret detection and verification.

Secret scanners help catch mistakes. They do not prove that a repository contains no secrets. They can miss unknown formats, encoded values, low-entropy passwords, and application-specific secret types. Review false-positive allowlists carefully; do not exclude entire directories just because scanning them is inconvenient.

Use a current Gitleaks GitHub Action release compatible with the GitHub-hosted runner runtime. The older v2 workflow example in the source notes should not be copied unchanged: the Gitleaks Action v3 line uses the newer Node runtime and its current examples use a newer checkout action. Pin the actual action commits to reviewed full-length SHAs in the working repository.

### `.gitignore` baseline

```gitignore
# Local private identities and key material
keys.txt
*.agekey
*.private-key
*.identity

# Local plaintext and temporary outputs
*.decrypted.*
*.plaintext.*
*.tmp
.tmp/
plaintext-source/
private-work/

# Environment files
.env
.env.*
!.env.example

# Editor and operating-system files
*.swp
*.swo
*~
.DS_Store
Thumbs.db
```

Do **not** ignore `payloads/*.age`; encrypted payloads are intended to be committed. `.gitignore` does not remove a file that Git already tracks. If a secret was committed, treat it as leaked and follow the incident process.

## 14. Key Rotation, Revocation, and History Cleanup

There are three different operations:

1. **Recipient-list update:** changes which identities can decrypt the current encrypted object.
2. **Data-key rotation:** re-encrypts content using a new internal data key or a newly produced encrypted file.
3. **History cleanup:** rewrites Git history to remove old committed objects from the repository's current history graph.

These are not interchangeable.

### SOPS rotation

- `sops updatekeys` synchronizes recipient metadata with `.sops.yaml`.
- `sops rotate -i <file>` creates a new data encryption key and re-encrypts values.
- After removing a recipient, do both as required by the file's current state, then test with a remaining recipient.

### age payload rotation

Decrypt the original payload locally and encrypt it again to the remaining recipient set. Do not try to “edit” an age file to remove a recipient.

### Git history cleanup

If plaintext was ever committed:

1. Assume the secret has been exposed.
2. Revoke or rotate the underlying credential immediately.
3. Use `git-filter-repo` or BFG Repo-Cleaner to rewrite affected history if appropriate.
4. Force-push the rewritten history and coordinate with collaborators.
5. Check branches, tags, forks, caches, release assets, and local clones.
6. Remember that history rewriting cannot delete copies held by other people or systems.

Git history cleanup is damage control. It is not a substitute for rotating a leaked password, API token, signing key, or decryption identity.

## 15. Cryptography Curriculum

The repository is organized into six learning modules. The order moves from historical ciphers to modern primitives and then to multi-party and post-quantum systems.

### Module 01 — Classical Ciphers

| Subject | Main lesson | Status |
|---|---|---|
| Caesar | Fixed alphabet shift; tiny key space | Broken; educational only |
| Vigenère | Repeating-key polyalphabetic substitution | Broken; key length and language patterns leak information |
| Playfair | Digraph substitution | Historical; not secure today |
| Enigma M3 | Rotor-based substitution and operational key settings | Historical machine cipher; study its design and operational weaknesses |
| Transposition | Rearranging character positions | Does not provide modern confidentiality |

Teach substitution, transposition, key spaces, known-plaintext reasoning, frequency analysis, and the difference between historical cleverness and modern security.

### Module 02 — Stream and Block Encryption

| Subject | Main lesson | Status |
|---|---|---|
| RC4 | Stream cipher and keystream bias | Broken; do not use |
| AES-CBC | Block mode, IV rules, padding, and padding-oracle risks | Legacy mode; requires a separate authentication design |
| AES-256-GCM | Authenticated encryption with associated data | Modern AEAD mode when nonce rules are followed |
| ChaCha20-Poly1305 | Stream encryption with authentication | Modern AEAD mode, useful in software without AES acceleration |

Teach block and stream encryption, nonce/IV handling, authentication tags, associated data, padding oracles, and why modern designs normally use AEAD instead of encryption without authentication.

### Module 03 — Asymmetric Cryptography and Key Exchange

| Subject | Main lesson | Important distinction |
|---|---|---|
| RSA-OAEP | Asymmetric encryption with randomized padding | Encryption primitive; use safe parameters and library implementations |
| Diffie-Hellman | Establishing a shared secret over a public channel | Key exchange, not direct message encryption |
| X25519 | Elliptic-curve Diffie-Hellman key agreement | Key agreement, not a signature scheme |
| Ed25519 | Deterministic digital signatures | Signature scheme, not encryption |

Teach trapdoor functions, public/private key roles, key agreement, signatures, key confirmation, and why encryption, key exchange, and signing are separate operations.

### Module 04 — Hashes, MACs, and KDFs

| Subject | Main lesson | Important distinction |
|---|---|---|
| SHA-256 | Cryptographic hash and preimage resistance | Hashing, not encryption |
| BLAKE3 | Tree-based cryptographic hash | Hashing, not encryption |
| HMAC | Keyed message authentication | Integrity/authentication, not confidentiality |
| PBKDF2 | Password-based key derivation | Slows password guessing when configured appropriately |
| Argon2id | Memory-hard password-based key derivation | Strong modern password-stretching option when parameters are chosen for the deployment |

Teach collision resistance, preimage resistance, length-extension issues where relevant, message authentication, salt handling, iteration cost, memory hardness, and offline password attacks.

### Module 05 — Multi-Party and Hardware-Bound Access

| Subject | Main lesson | Important distinction |
|---|---|---|
| Shamir's Secret Sharing | Split one secret into shares with an M-of-N threshold | Shares must be protected and distributed separately |
| WebAuthn PRF | Derive a 32-byte output from a credential-associated PRF | The application receives the output; the authenticator's underlying secret is not exposed |
| Hardware security keys | Bind an operation to a physical authenticator | Requires recovery and replacement planning |
| HSM / PKCS#11 | Use hardware-backed key operations | Hardware custody; not automatically zero-knowledge for every caller or service |

Teach threshold rules, share handling, recovery, hardware-bound credentials, relying-party origins, credential lifecycle, and what happens when a device is lost.

### Module 06 — Post-Quantum Cryptography

| Subject | Main lesson | Standard or role |
|---|---|---|
| ML-KEM-768 | Key encapsulation based on module lattices | NIST FIPS 203; KEM, not bulk data encryption |
| ML-DSA-65 | Digital signatures based on module lattices | NIST FIPS 204; signatures, not encryption |
| Hybrid X25519 + ML-KEM | Combine classical and post-quantum key establishment | Migration and defence-in-depth topic |
| age hybrid recipients | Practical file-encryption integration | Native hybrid support begins with age v1.3.0; client compatibility must be tested |

Teach KEMs, lattice assumptions, hybrid key establishment, larger key and ciphertext sizes, interoperability, migration, and “harvest now, decrypt later” risk.

Do not label ML-KEM as a bulk cipher. Do not label ML-DSA as an encryption algorithm. Use NIST names in current lessons and include older names such as Kyber and Dilithium only as historical or transition context.

## 16. Test Vectors and Challenge Design

A cryptography learning repository must not invent test vectors or call placeholders “verified”.

### Public vectors

- Store published standard vectors in `vectors/`.
- Record the source specification, section, test-case ID, and source URL.
- Keep the input, output, parameters, and encoding exactly as required by the source.
- Add an automated verification test using a maintained cryptographic library or reference implementation.
- Mark a vector `verified` only after the automated test passes.
- If a vector is modified for teaching, label it as an adapted example, not an official vector.

### Private challenges

A private challenge can wrap a custom task around public concepts. Its answer or hidden inputs should be generated separately and encrypted into the `.age` payload.

Do not encrypt public NIST or RFC test vectors just to make them look secret. Their purpose is to be public, reproducible, and independently checkable. Encrypt the challenge answer, not the public standard itself.

### Vector quality rules

- Every value must have a named encoding: UTF-8, hex, base64, binary, or another explicit format.
- Every key length, nonce length, tag length, salt length, iteration count, and memory parameter must be stated.
- Distinguish a secret input from a public parameter.
- Distinguish a test vector from a challenge placeholder.
- Never use strings such as `PLACEHOLDER_KEY` as if they were cryptographic values.
- Keep test fixtures deterministic where the standard requires deterministic vectors.
- Test tampering cases for AEAD algorithms, not just successful decryption.
- For password KDF examples, state the exact password bytes and the KDF parameters.
- For signatures, state the message, public key, signature, and verification result.

## 17. Repository Conventions

Consistency reduces mistakes. Every contributor should follow the same conventions.

### Naming

- Use lowercase kebab-case for files and directories.
- Prefix curriculum modules with two digits: `01-`, `02-`, and so on.
- Use stable lesson IDs such as `02-aes-256-gcm`.
- Keep the lesson ID identical across the manifest, payload metadata, tests, and documentation.
- Use `.yaml` or `.json` for public manifests, `.age` for encrypted binary payloads, and `.sops.yaml` for SOPS-managed structured files.
- Do not put names, passwords, answers, or other secret context in encrypted file names.

### Public fields

A public lesson should normally define:

- `id`
- `title`
- `category`
- `difficulty`
- `status`
- `summary`
- `references`
- `public_inputs`
- `payload.path`
- `payload.sha256`

Never place private answers in a field that is not selected for encryption. When in doubt, move private content into an age payload.

### Status values

Use a small fixed set:

- `draft` — incomplete; not included in a stable release;
- `experimental` — usable for testing, but not yet fully reviewed;
- `stable` — reviewed, tested, and documented;
- `deprecated` — retained for historical learning, not recommended for new use.

### Security labels

Use clear labels such as:

- `public` — intended to be visible to everyone;
- `encrypted` — ciphertext stored in the repository;
- `private-cli` — decrypted only in a local CLI workflow;
- `browser-decryptable` — raw age payload supported by the portal;
- `demo-only` — safe sample material that is intentionally public.

These labels describe project handling. They do not replace encryption or file permissions.

### Documentation style

Every lesson should explain:

1. What the primitive does.
2. What problem it solves.
3. What its inputs and outputs are.
4. How its keys are created and managed.
5. What common mistakes look like.
6. Whether it is appropriate for new systems.
7. Which published standard or test vector is used.
8. How to reproduce the example locally.

Use simple language first, then add exact formulas, standards, parameters, and code for readers who want more detail.

## 18. Operational Runbooks

### Adding a new lesson

1. Pick the correct curriculum module and a stable lesson ID.
2. Add a public manifest with safe metadata and official references.
3. Add public test vectors, if applicable, and record their source.
4. Write challenge content outside the repository.
5. Encrypt the challenge using the approved recipient set.
6. Add only the encrypted payload to `payloads/`.
7. Calculate the ciphertext digest and update the manifest.
8. Run schema, reference, vector, and secret-scanning checks.
9. Test decryption with at least one enrolled identity and, where possible, a second authorized identity.
10. Confirm that the Pages build contains ciphertext only.
11. Submit the change for review.

### Publishing a release

1. Run the full test suite locally.
2. Confirm that all published vectors pass.
3. Confirm that no real private identity or plaintext solution is staged.
4. Build the browser assets from locked dependencies.
5. Generate hashes and sign the release manifest or artifact set.
6. Review the public files that will be deployed.
7. Deploy with a restricted GitHub Actions token.
8. Download the deployed assets and compare them against the signed release output.

### Responding to a suspected leak

1. Stop further publication if an active deployment is affected.
2. Identify whether the exposed value was a password, API token, private identity, signing key, plaintext answer, or public key.
3. Revoke or rotate the exposed credential at its source.
4. Re-encrypt payloads for the new recipient set if an identity was exposed.
5. Rewrite Git history if appropriate, while assuming existing copies remain.
6. Review Actions logs, release assets, Pages output, forks, and local build artifacts.
7. Record the incident and add a regression test or scanning rule.

## 19. Architecture Decision Matrix

| Requirement | Recommended approach | Reason |
|---|---|---|
| Readable public metadata with private values | Public manifest + age payload | Clean separation and browser compatibility |
| Selected encrypted values in a YAML/JSON file | SOPS + age | Preserves document shape and readable keys |
| Whole-file encryption in ordinary Git workflows | git-crypt or Transcrypt | Transparent working-tree experience |
| Encrypt an entire Git remote | git-remote-gcrypt | Hides more repository structure from the remote |
| Different access rights for different folders | Separate repositories or submodules | GitHub does not provide private folders inside one repository |
| Browser decryption for enrolled identities | Static portal + bundled Typage/age implementation | Decryption occurs locally |
| Shared-password static page | StatiCrypt or PageCrypt | Simple, but public ciphertext can be attacked offline |
| Hardware-backed browser key derivation | WebAuthn PRF | Credential-bound derivation where supported |
| Threshold access | Shamir's Secret Sharing or a reviewed threshold service | Requires M-of-N participation |
| Signed public metadata | TUF or a signed manifest with Sigstore/Cosign | Adds integrity and publisher verification |
| Large seekable encrypted files | Crypt4GH or another suitable container | Better fit for specialised large-file access |
| Hide file sizes and update patterns | Padding, chunking, batching | Reduces some metadata leakage at a storage cost |
| Hide which object a user requests | OHTTP or PIR | Advanced access-pattern privacy; adds infrastructure |
| Internal Pages documentation | GitHub Enterprise Cloud private Pages | Organization access control, not zero-knowledge storage |

## 20. Extended Tool and Protocol Catalogue

This section preserves the additional pathways collected during the design discussion. They are grouped by job so that related tools are not repeated as if they were separate architectures.

### A. Git and repository layout

| Item | Use |
|---|---|
| Git repository visibility | Access applies to the repository, not individual folders. |
| Git submodules | Link a public repository to a separate private repository. A submodule is a pointer, not a private folder. |
| `.gitignore` | Prevent selected untracked local files from being added by mistake. It cannot untrack a file already committed. |
| `.env.example` | Publish sanitized configuration names and placeholders. Never copy real credentials into it. |
| GitHub Releases | Publish versioned artifacts. Release visibility follows repository access; use a separate public distribution channel for public downloads from private source. |
| Git history rewriting | `git-filter-repo` and BFG can rewrite current history to remove leaked content. They cannot erase other people's copies. |
| Git clean/smudge filters | Automate encryption and decryption for selected paths during Git operations. |
| git-crypt | GPG-based or symmetric file-content encryption for selected tracked files. |
| Transcrypt | Symmetric encryption through Git filters for selected files. |
| git-remote-gcrypt | Encrypt repository objects before pushing to a remote. |
| Pass / Gopass | Local directory-based secret stores that can use GPG or age and may be versioned with Git. |

### B. Encryption, formats, and browser cryptography

| Item | Use |
|---|---|
| SOPS | Encrypt selected values in structured YAML, JSON, ENV, INI, or binary files. |
| age | Multi-recipient file encryption with explicit public recipients and private identities. |
| GPG / OpenPGP | Established encryption and signing ecosystem; useful for existing key infrastructure. |
| Cloud KMS | Managed key wrapping and cryptographic operations through a cloud provider. This adds a provider trust boundary. |
| AES-256-GCM | Authenticated symmetric encryption; nonce reuse with the same key is unsafe. |
| WebCrypto | Browser cryptographic API for supported algorithms, including AES-GCM and PBKDF2. |
| PBKDF2 | Password-based key derivation; slows offline guessing but does not make a weak password strong. |
| Argon2id | Memory-hard password KDF; requires an additional browser implementation when used in a web page. |
| OpenPGP.js | JavaScript OpenPGP implementation for browser or Node use. |
| Typage / age in WebAssembly or JavaScript | Browser-side age implementation; bundle locally and verify version and format compatibility. |
| StatiCrypt | Static HTML encryption with a shared passphrase. |
| PageCrypt | Another static HTML password-protection option. |
| Hat.sh | Browser-based file encryption/decryption architecture using client-side cryptography. Treat it as a reference project, not as a hidden backend. |
| Crypt4GH | Specialist encrypted container with multi-recipient and random-access use cases. |

### C. Identity, enrollment, and key custody

| Item | Use |
|---|---|
| Native age keypairs | Preferred default for new recipients. |
| age SSH recipients | Compatibility path for supported `ssh-ed25519` and `ssh-rsa` public keys. Do not treat them as anonymous or hardware-agent-compatible. |
| GitHub `.keys` endpoint | Provides a user's published SSH public keys at `https://github.com/<username>.keys`. Validate the result and confirm enrollment with the owner. |
| GPG identities | Alternative user identity and signing system. |
| WebAuthn PRF | Credential-associated 32-byte PRF output that can be used as key material. Support and recovery depend on authenticators and browsers. |
| Shamir's Secret Sharing | Splits a secret into shares that require an M-of-N threshold to reconstruct. |
| Hardware Security Modules | Hardware-backed key generation, signing, or unwrapping operations. |
| PKCS#11 | Standard interface used by applications to access cryptographic tokens and HSMs. |
| age YubiKey plugin | Hardware-backed age identity option where compatible hardware and client setup are available. |
| HashiCorp Vault / OpenBao | External secret and cryptographic service. A service that sees plaintext while performing an operation is part of the trust boundary. |
| Bitnami Sealed Secrets | Kubernetes-focused encrypted secret manifests that a cluster controller can decrypt. |
| External Secrets Operator | Kubernetes integration for retrieving secrets from external secret stores. |
| Lit Protocol | Decentralized threshold/key-access network. Treat it as an advanced external trust dependency, not a simple GitHub login replacement. |
| Key rotation and re-keying | Change recipient access and, where required, re-encrypt with new data keys. |
| Forward-secrecy ratcheting | Key evolution for systems designed around changing sessions or epochs. It is not automatically provided by a static age file. |

### D. Browser delivery, access control, and deployment

| Item | Use |
|---|---|
| GitHub Pages | Static hosting for the public portal. |
| GitHub Enterprise Cloud private Pages | Restricts the site to repository readers for supported organizations. GitHub still hosts the site content. |
| Client-side decryption | Browser receives ciphertext and decrypts locally. Its security depends on the code delivered to the browser. |
| Session storage | Can persist a key for a tab session, but same-origin scripts can read it. Memory-only handling is safer as the default. |
| URL hash fragments | Browsers do not send fragments in the HTTP request, but page scripts can read them and users may leak them through history, logs, or sharing. Avoid placing keys in URLs. |
| SRI | Checks that referenced static assets match expected hashes. It does not independently authenticate the root HTML. |
| CSP | Restricts script, style, image, connection, and other browser resource paths. It is defence-in-depth, not an attestation that code is honest. |
| Cloudflare Access / Zero Trust | Identity-aware front-door access for a web application. Ensure the origin cannot be reached through an unprotected alternate URL. |
| GitHub Actions OIDC | Short-lived identity tokens for a workflow to access an external service. It does not make a public build safe to receive arbitrary private keys. |
| GitHub Environments | Deployment gates, required reviewers, and environment-scoped secrets. Use only where a protected workflow truly needs them. |
| `.env` and environment variables | Local configuration mechanism; keep actual values out of Git. |

### E. Privacy, verification, and advanced delivery

| Item | Use |
|---|---|
| Ciphertext padding and fixed-size chunking | Hide some payload-length information at the cost of storage and processing overhead. |
| Decoy entries | Add noise to public object counts. This is not a substitute for hiding sensitive names or access patterns. |
| IPFS | Content-addressed hosting for immutable encrypted blobs. It does not provide confidentiality or guaranteed deletion. |
| Unlisted GitHub Gists | Conveniently obscure links, not private access control. Anyone with the URL can share it. |
| Mutable release assets | Can separate payload distribution from Git commits, but downloaded or mirrored copies remain. |
| TUF | Signed metadata and delegated trust for update systems. |
| Sigstore / Cosign | Signing and verification of release artifacts and build outputs. |
| Oblivious HTTP (OHTTP) | Uses a relay to reduce what a server learns about a client's requested resource. |
| Private Information Retrieval (PIR) | Allows queries where the data server learns less about which item was requested; more complex than ordinary static hosting. |
| Tor onion service mirroring | Alternative routing and access path; does not encrypt application content by itself. |
| Offline brute-force resistance | Use public-key recipients or strong passphrases with an appropriate KDF. Public ciphertext can always be copied and tested offline. |

These options are not all part of the default stack. The default remains public manifests + age payloads + SOPS for selected structured CLI files + Gitleaks + a static browser reader.

## 21. Known Limits and Common Mistakes

| Mistake | Why it is a problem | Correct approach |
|---|---|---|
| Commit plaintext first, encrypt later | Old commits still contain the secret | Encrypt before the first `git add` |
| Put an age private identity in the public repository | Anyone can decrypt every payload for that identity | Store identities outside Git |
| Assume `.gitignore` removes tracked files | It only affects untracked files | Untrack files and treat old commits as exposed |
| Use `connect-src 'none'` and fetch local blobs | The browser blocks `fetch()` | Embed payloads in a local bundle or use `connect-src 'self'` |
| Load age libraries from a public CDN | The page has a remote code dependency and needs network permission | Bundle and pin the dependency locally |
| Store a real private key in the demo page | Everyone can inspect the page and copy it | Use a demo-only identity and demo-only ciphertext |
| Persist visitor keys in `localStorage` | Long-lived same-origin access increases risk | Keep keys in memory; make persistence an explicit opt-in if ever added |
| Claim CSP proves no exfiltration | CSP has limits and cannot certify the code itself | Verify code, restrict sinks, and use independent signatures or offline operation |
| Treat SOPS YAML as a raw age blob | SOPS has its own metadata and encryption structure | Use a raw `.age` file for browser payloads |
| Assume removing a recipient erases old access | Former recipients may hold old files or plaintext | Rotate future files and plan for copies already made |
| Treat public SSH keys as private or anonymous | They are public and may be linkable to an account | Prefer dedicated native age keys |
| Treat a hash stored beside a file as publisher proof | Both can be changed by the same attacker | Verify a signed manifest with an independent trust root |
| Call every test vector “real” without verification | Incorrect examples teach incorrect cryptography | Cite the source and test every vector |
| Treat ML-KEM as a symmetric cipher | It establishes shared key material; it does not encrypt bulk files by itself | Use a KEM plus an authenticated symmetric cipher |
| Treat Ed25519 or ML-DSA as encryption | They are signature algorithms | Teach signing and encryption as separate functions |
| Use a short password for public ciphertext | Attackers can guess it offline | Use a high-entropy passphrase and a suitable KDF |
| Assume GitHub Releases on a private repo are public | Release access follows repository permissions | Use a separate public distribution channel |

## 22. Roadmap

Status as of the first release. Checked items are implemented and covered by CI.

### Phase 1 — Safe repository foundation

- [x] Create the repository structure.
- [x] Add schemas for public manifests and encrypted payload content.
- [x] Add `.gitignore`, `.gitattributes`, `.sops.yaml`, and Gitleaks configuration.
- [x] Generate maintainer age identities outside the repository.
- [x] Add a small set of public, sourced test vectors (34 vectors, all recomputed in CI).
- [x] Add CLI encrypt/decrypt instructions.
- [x] Add CI schema, hash, vector, and secret-scanning checks.

### Phase 2 — Curriculum and challenge payloads

- [x] Add all six curriculum modules.
- [x] Separate public test vectors from private challenge answers.
- [x] Encrypt challenge payloads to enrolled recipients.
- [x] Add recipient enrollment and rotation runbooks.
- [ ] Verify every payload with more than one authorized identity where practical (only one maintainer is enrolled so far).

### Phase 3 — Static browser portal

- [x] Build the public lesson browser.
- [x] Bundle a pinned age implementation locally.
- [x] Add browser decryption for supported age payloads.
- [x] Add memory-only identity handling and a visible lock action.
- [x] Add CSP and local-only dependencies.
- [x] Add a demo identity that unlocks demo content only.
- [x] Compare browser output against CLI output and known test vectors.

### Phase 4 — Release integrity

- [x] Generate release checksums.
- [x] Sign release metadata or artifacts (GitHub build-provenance attestations, Sigstore-backed).
- [x] Document offline verification.
- [x] Pin GitHub Actions to reviewed full commit SHAs.
- [x] Add dependency update review and reproducible-build checks.

### Phase 5 — Optional advanced features (not started)

- [ ] WebAuthn PRF prototype with documented browser and authenticator support.
- [ ] Hardware-backed age identity support where suitable.
- [ ] M-of-N recovery using Shamir's Secret Sharing.
- [ ] Fixed-size encrypted payload packaging if metadata protection is needed.
- [ ] TUF-style metadata and delegated signing if the project needs a more formal update chain.
- [ ] Optional IPFS or Tor mirrors for distinct distribution needs.
- [ ] OHTTP or PIR research only if access-pattern privacy becomes a real requirement.

## 23. Glossary

| Term | Meaning |
|---|---|
| AEAD | Authenticated Encryption with Associated Data. Encrypts content and checks integrity/authenticity of the ciphertext and associated data. |
| age | A simple file-encryption format and CLI based on explicit recipients and identities. |
| Ciphertext | Encrypted data. It can be public without revealing plaintext when encryption is correctly used. |
| KDF | Key Derivation Function. Derives key material from another secret, often a password. |
| KEM | Key Encapsulation Mechanism. Establishes shared key material for later symmetric encryption. |
| Manifest | Public metadata that describes lessons and points to payloads. |
| Recipient | A public key that can be used to encrypt data for a key owner. |
| Identity | The private key material used to decrypt data for a recipient. |
| SOPS | Mozilla's Secrets OPerationS tool for editing encrypted structured files. |
| SRI | Subresource Integrity. Browser check that a referenced resource matches an expected hash. |
| CSP | Content Security Policy. Browser rules that limit permitted resource and connection sources. |
| WebAuthn PRF | WebAuthn extension that lets a relying party request a credential-associated pseudo-random output. |
| TUF | The Update Framework. A framework for secure update metadata and delegated trust. |
| OHTTP | Oblivious HTTP. A relay-based protocol that limits what a server learns about a client's request. |
| PIR | Private Information Retrieval. A family of methods for querying data while revealing less about the requested item. |

## 24. References

The following primary project and standards pages should be checked when versions or requirements change. The README's version floors are compatibility guidance, not a promise that future releases will remain identical.

### Core tools

- [age — project, CLI, and file format](https://github.com/FiloSottile/age)
- [age — releases and post-quantum support](https://github.com/FiloSottile/age/releases)
- [Typage — TypeScript age implementation](https://github.com/FiloSottile/typage)
- [Mozilla SOPS — project and releases](https://github.com/getsops/sops)
- [SOPS documentation](https://sops.pages.dev/)
- [Gitleaks](https://github.com/gitleaks/gitleaks)
- [Gitleaks GitHub Action](https://github.com/gitleaks/gitleaks-action)
- [TruffleHog](https://github.com/trufflesecurity/trufflehog)

### GitHub and browser security

- [GitHub Pages — getting started](https://docs.github.com/en/pages/getting-started-with-github-pages)
- [GitHub Pages — private site visibility](https://docs.github.com/en/enterprise-cloud@latest/pages/getting-started-with-github-pages/changing-the-visibility-of-your-github-pages-site)
- [GitHub Pages — privately published sites](https://docs.github.com/en/enterprise-cloud@latest/pages/getting-started-with-github-pages/about-privately-published-github-pages-sites)
- [GitHub Actions — secure use](https://docs.github.com/en/actions/security-for-github-actions/security-guides/security-hardening-for-github-actions)
- [MDN — Content Security Policy](https://developer.mozilla.org/en-US/docs/Web/HTTP/Guides/CSP)
- [MDN — `connect-src`](https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Content-Security-Policy/connect-src)
- [MDN — CSP `frame-ancestors` and meta-element limits](https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Content-Security-Policy/frame-ancestors)

### Standards

- [NIST FIPS 203 — ML-KEM](https://csrc.nist.gov/pubs/fips/203/final)
- [NIST FIPS 204 — ML-DSA](https://csrc.nist.gov/pubs/fips/204/final)
- [W3C Web Authentication Level 3 — PRF extension](https://www.w3.org/TR/webauthn-3/)
- [RFC 7748 — X25519 and X448](https://www.rfc-editor.org/rfc/rfc7748)
- [RFC 8032 — EdDSA](https://www.rfc-editor.org/rfc/rfc8032)
- [RFC 8439 — ChaCha20 and Poly1305](https://www.rfc-editor.org/rfc/rfc8439)
- [RFC 9106 — Argon2](https://www.rfc-editor.org/rfc/rfc9106)
- [NIST SP 800-38D — GCM](https://csrc.nist.gov/pubs/sp/800/38/d/final)

### Additional projects and protocols

- [git-crypt](https://github.com/AGWA/git-crypt)
- [Transcrypt](https://github.com/elasticdog/transcrypt)
- [git-remote-gcrypt](https://github.com/spwhitton/git-remote-gcrypt)
- [Pass](https://www.passwordstore.org/)
- [Gopass](https://www.gopass.pw/)
- [OpenPGP.js](https://github.com/openpgpjs/openpgpjs)
- [StatiCrypt](https://github.com/robinmoisson/staticrypt)
- [Crypt4GH](https://github.com/EGA-archive/crypt4gh)
- [The Update Framework](https://theupdateframework.io/)
- [Sigstore](https://www.sigstore.dev/)
- [OpenBao](https://openbao.org/)
- [HashiCorp Vault](https://www.hashicorp.com/products/vault)
- [External Secrets Operator](https://external-secrets.io/)
- [Sealed Secrets](https://github.com/bitnami-labs/sealed-secrets)
- [IPFS](https://ipfs.tech/)
- [Oblivious HTTP](https://www.rfc-editor.org/rfc/rfc9458)

---

## Final Design Rule

**Publish the explanation. Encrypt the answer. Keep the identity. Verify the code. Rotate when access changes.**

That is the whole architecture in one line. Everything else in this repository exists to make those five rules easy to follow and hard to accidentally break.
