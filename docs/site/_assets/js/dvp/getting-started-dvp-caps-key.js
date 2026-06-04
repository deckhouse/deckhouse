// DVP GS: Ed25519 CAPS SSH key pair in browser (requires getting-started-dvp-constants.js).

function dvpConcatBytes(chunks) {
  var total = chunks.reduce(function (sum, chunk) {
    return sum + chunk.length;
  }, 0);
  var out = new Uint8Array(total);
  var offset = 0;
  chunks.forEach(function (chunk) {
    out.set(chunk, offset);
    offset += chunk.length;
  });
  return out;
}

function dvpUint32BE(n) {
  var bytes = new Uint8Array(4);
  new DataView(bytes.buffer).setUint32(0, n >>> 0, false);
  return bytes;
}

function dvpSshString(str) {
  var encoded = new TextEncoder().encode(str);
  return dvpConcatBytes([dvpUint32BE(encoded.length), encoded]);
}

function dvpSshBuffer(buf) {
  return dvpConcatBytes([dvpUint32BE(buf.length), buf]);
}

function dvpBase64UrlToBytes(b64url) {
  var pad = '='.repeat((4 - (b64url.length % 4)) % 4);
  var b64 = b64url.replace(/-/g, '+').replace(/_/g, '/') + pad;
  var binary = atob(b64);
  var bytes = new Uint8Array(binary.length);
  for (var i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i);
  }
  return bytes;
}

function dvpBytesToBase64(bytes) {
  var binary = '';
  for (var i = 0; i < bytes.length; i++) {
    binary += String.fromCharCode(bytes[i]);
  }
  return btoa(binary);
}

function dvpWrapPem(base64Body) {
  var lines = base64Body.match(/.{1,70}/g) || [];
  return '-----BEGIN OPENSSH PRIVATE KEY-----\n' + lines.join('\n') + '\n-----END OPENSSH PRIVATE KEY-----\n';
}

function dvpEncodeOpenSSHEd25519PrivateKey(seed, publicKey, comment) {
  var pubBlob = dvpSshBuffer(dvpConcatBytes([dvpSshString('ssh-ed25519'), dvpSshBuffer(publicKey)]));

  var check = crypto.getRandomValues(new Uint8Array(4));
  var checkInt = new DataView(check.buffer).getUint32(0, false);

  var privateSection = [];
  privateSection.push.apply(privateSection, dvpUint32BE(checkInt));
  privateSection.push.apply(privateSection, dvpUint32BE(checkInt));
  privateSection.push.apply(privateSection, dvpSshString('ssh-ed25519'));
  privateSection.push.apply(privateSection, dvpSshBuffer(publicKey));

  var privateKey64 = new Uint8Array(64);
  privateKey64.set(seed, 0);
  privateKey64.set(publicKey, 32);
  privateSection.push.apply(privateSection, dvpSshBuffer(privateKey64));
  privateSection.push.apply(privateSection, dvpSshString(comment || 'dvp-caps'));

  var padLen = privateSection.length % 8;
  if (padLen !== 0) {
    padLen = 8 - padLen;
  }
  for (var p = 1; p <= padLen; p++) {
    privateSection.push(p);
  }

  var payload = dvpConcatBytes([
    new TextEncoder().encode('openssh-key-v1\u0000'),
    dvpSshString('none'),
    dvpSshString('none'),
    dvpSshString(''),
    dvpUint32BE(1),
    pubBlob,
    dvpSshBuffer(new Uint8Array(privateSection)),
  ]);

  return dvpWrapPem(dvpBytesToBase64(payload));
}

function dvpFormatEd25519PublicKey(publicKey, comment) {
  var blob = dvpConcatBytes([dvpSshString('ssh-ed25519'), dvpSshBuffer(publicKey)]);
  return 'ssh-ed25519 ' + dvpBytesToBase64(blob) + ' ' + (comment || 'dvp-caps');
}

function dvpAsciiToBase64(str) {
  return btoa(str);
}

// Generate Ed25519 CAPS key pair once per GS session (sessionStorage).
function generate_caps_ssh_key(force) {
  if (
    !force &&
    sessionStorage.getItem(STORAGE_KEYS.capsPrivateKeyB64) &&
    sessionStorage.getItem(STORAGE_KEYS.capsPublicKey)
  ) {
    return Promise.resolve();
  }

  if (
    !globalThis.crypto ||
    !globalThis.crypto.subtle ||
    typeof globalThis.crypto.subtle.generateKey !== 'function'
  ) {
    return Promise.reject(new Error('Web Crypto API is not available in this browser'));
  }

  return globalThis.crypto.subtle
    .generateKey({ name: 'Ed25519' }, true, ['sign', 'verify'])
    .then(function (keyPair) {
      return Promise.all([
        globalThis.crypto.subtle.exportKey('jwk', keyPair.privateKey),
        globalThis.crypto.subtle.exportKey('raw', keyPair.publicKey),
      ]);
    })
    .then(function (parts) {
      var jwk = parts[0];
      var publicKeyRaw = new Uint8Array(parts[1]);
      var seed = dvpBase64UrlToBytes(jwk.d);

      var pem = dvpEncodeOpenSSHEd25519PrivateKey(seed, publicKeyRaw, 'dvp-caps');
      var publicKeyLine = dvpFormatEd25519PublicKey(publicKeyRaw, 'dvp-caps');

      sessionStorage.setItem(STORAGE_KEYS.capsPrivateKeyB64, dvpAsciiToBase64(pem));
      sessionStorage.setItem(STORAGE_KEYS.capsPublicKey, publicKeyLine);
    });
}
