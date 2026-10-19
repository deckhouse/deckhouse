# AGENTS.md

## Test and documentation secrets (gitleaks)

Gitleaks scans the whole repository: tests, testdata, golden files, fixtures, documentation and module examples. Secret-looking example values must not introduce new findings, so every such value must be taken from the list below. These values are allowlisted once in `.gitleaks.toml`.

Rules:

- When writing or reviewing tests, documentation or examples, take any password, token, API key, base64 secret payload, htpasswd or bcrypt hash, JWT, private key or cloud credential from this list. Do not invent new ones, do not copy real ones.
- Pick the value by purpose. If several fit, choose the one whose decoded meaning reads best in context.
- If nothing fits, add a new value to this list and to the last `[[allowlists]]` block in `.gitleaks.toml` in the same change.
- Every new entry in `.gitleaks.toml` gets a comment above it with the date, the MR or issue that added it and the reason, for example `# Added on 2026-10-16 in MR !703: ...`. A whole-file exclusion in `paths` is allowed only for third-party files or fixtures parsed by the code under test, by exact path, and never for files with real secrets.
- The pipeline reads `.gitleaks.toml` from `main`, so a new entry works only after it is merged. Add it in a separate MR before the MR that uses it.
- A value that is not in the list is a gitleaks finding. Do not add per-file or per-value ignores, replace it with a value from the list.
- Inline `gitleaks:allow` comments are ignored by CI (`--ignore-gitleaks-allow`). Never add them.
- Use the encoded form where `Secret.data`, docker config `auth` or similar requires base64. The "Meaning" column shows the decoded text.
- When a hook or code under test needs a value of a specific length or format, extend `dh-test-secret-` with a suffix of that length instead of inventing a new secret. Check what the code actually parses before replacing a value: keys and certificates that are parsed, and names derived by the code under test, must stay as they are.
- Obvious placeholders such as `<YOUR-TOKEN>` or `***` are not findings and are always fine in documentation.

| Purpose | Value | Meaning |
|---|---|---|
| Plain password, token or API key | `dh-test-secret-0123456789abcdef` | |
| Plain value of a required length | `dh-test-secret-` plus any `[0-9A-Za-z]` suffix, e.g. `dh-test-secret-012345678` (24 chars), `dh-test-secret-0123456789abcdef0` (32 chars) | |
| Base64 of any `dh-test-secret-*` value | starts with `ZGgtdGVzdC1zZWNyZXQt`, e.g. `ZGgtdGVzdC1zZWNyZXQtMDEyMzQ1Njc4` | `dh-test-secret-012345678` |
| Base64 secret data (`Secret.data`) | `dGVzdC12YWx1ZQ==` | `test-value` |
| Docker config / registry `auth` (base64 `user:password`) | `dXNlcm5hbWU6cGFzc3dvcmQ=` (the value without the trailing `=` is also allowed, for invalid-padding cases) | `username:password` |
| Base64 token | `PFlPVVItR1JBRkFOQUNMT1VELVRPS0VOPg==` | `<YOUR-GRAFANACLOUD-TOKEN>` |
| Base64 htpasswd line | `Zm9vOiRhcHIxJE9GRzNYeWJwJGNrTDBGSERBa29YWUlsSDkuY3lzVDAK` | `foo:$apr1$OFG3Xybp$ckL0FHDAkoXYIlH9.cysT0` |
| Base64 bcrypt hash (`User.spec.password`) | `JDJ5JDEwJGRNWGVGUVBkdUdYYVMyWDFPcGdZdk9HSy81LkdsNm5sdU9mUkhnNWlQdDhuSlh6SzhpeS5H` | `$2y$10$dMXeFQPduGXaS2X1OpgYvOGK/5.Gl6nluOfRHg5iPt8nJXzK8iy.G` |
| Ceph key | `AQDiVXVmBJVRLxAAg65PhODrtwbwSWrjJwssUg==` | |
| GCP service account `private_key_id` | `98sdcj5e8c7asd98j4j3n9csakn` | |
| AWS access key id | `AKIAIOSFODNN7EXAMPLE` | |
| AWS secret access key | `wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY` | |
| JWT | `eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJkZWNraG91c2UtdGVzdCIsIm5hbWUiOiJ0ZXN0In0.dGVzdC1zaWduYXR1cmUtbm90LXJlYWw` | |

### Private keys

In tests use exactly this body between the markers:

```text
-----BEGIN PRIVATE KEY-----
ZGVja2hvdXNlLXRlc3QtcHJpdmF0ZS1rZXktbm90LWEtcmVhbC1zZWNyZXQtMDEyMzQ1Njc4OWFiY2RlZg==
-----END PRIVATE KEY-----
```

The same body is valid inside `RSA PRIVATE KEY` / `EC PRIVATE KEY` markers and in escaped single-line form (`\n`). If a test needs a cryptographically valid key or certificate (parsing, TLS), generate it at test runtime instead of committing it.

In documentation keep the key elided, exactly in one of these forms:

- Yandex Cloud JSON, single line: `-----BEGIN PRIVATE KEY-----\nMIIE....1ZPJeBLt+\n-----END PRIVATE KEY-----\n` (with the `\n` written as the two characters backslash and `n`).
- SSH key block:

  ```text
  -----BEGIN OPENSSH PRIVATE KEY-----
  ...
  -----END OPENSSH PRIVATE KEY-----
  ```

- GCP service account JSON: `"private_key": "-----BEGIN PRIVATE KEY-----",` followed by `"client_id": "342975658279248",`.
