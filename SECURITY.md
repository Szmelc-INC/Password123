# Security policy

## Reporting

Please report vulnerabilities privately through GitHub's **Report a vulnerability** button (Security → Advisories) on this repository. Do not open a public issue for:

- plaintext challenge content, private identities or other secrets found in the repository, its history, Pages output or release assets;
- a way to make the portal leak an identity or decrypted content;
- a flaw in the verifiers that lets a wrong vector pass as `verified`;
- a way around a `p123 validate` check.

Include a short description, the affected path or commit, and steps to reproduce. We aim to acknowledge reports within a few days.

## Scope and threat model

The confidentiality claim is narrow. Enrolled recipients can decrypt `payloads/*.age`; nobody else can, as long as their private identities stay private. File names, sizes, timestamps, lesson metadata and public recipients are **public by design**. The demo identity in `demo/` is public and opens demo payloads only.

The browser portal is a convenience layer. Whoever controls the host can change the JavaScript it serves. For high-assurance decryption use `p123` or `age` on a trusted machine, or verify a release with its attestation and run the portal offline. See [ARCHITECTURE.md](ARCHITECTURE.md) sections 4, 9 and 11.

## Responding to a suspected leak

1. Stop publication if an active deployment is affected (disable the Pages workflow).
2. Identify what leaked: a password, a token, a private identity, a signing key, a plaintext answer, or only a public key.
3. Revoke or rotate the exposed credential at its source. For an identity, remove the recipient and run `p123 rotate --all`. For a leaked answer, regenerate the challenge.
4. Rewrite Git history only if appropriate (`git-filter-repo`). Assume existing clones, forks and caches keep their copies.
5. Review Actions logs, release assets, Pages output and local build artifacts.
6. Record the incident and add a regression test or scanning rule.

The maintainers coordinate any history rewrite. It requires an explicit decision, because it is destructive.
