// DVP GS: sessionStorage keys, defaults, and session reads (steps 2–6).

var STORAGE_KEYS = {
  internal: 'dhctl-internal-network-cidrs',
  vm: 'dhctl-virtual-machine-cidrs',
  pod: 'dhctl-pod-subnet-cidr',
  service: 'dhctl-service-subnet-cidr',
  worker: 'dhctl-worker-node-ip',
  master: 'dhctl-master-node-ip',
  nfsShare: 'dhctl-nfs-share',
  nfsHost: 'dhctl-nfs-host',
  domain: 'dhctl-domain',
  password: 'dhctl-user-password',
  passwordHash: 'dhctl-user-password-hash',
  capsPrivateKeyB64: 'dhctl-caps-private-key-base64',
  capsPublicKey: 'dhctl-caps-public-key',
};

var DEFAULTS = {
  pod: '10.115.0.0/16',
  service: '10.225.0.0/16',
  vm: '10.20.0.0/16',
  nfsShare: '/srv/nfs/dvp',
  domain: '%s.domain.my',
  domainSuffix: 'domain.my',
};

// publicDomainTemplate validation (step 2 form).
var PUBLIC_DOMAIN_PATTERN = /^(%s([-a-z0-9]*[a-z0-9])?|[a-z0-9]([-a-z0-9]*)?%s([-a-z0-9]*)?[a-z0-9]|[a-z0-9]([-a-z0-9]*)?%s)(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$/;

// CAPS private key PEM from sessionStorage (base64-encoded PEM).
function dvpCapsPrivateKeyPem() {
  var b64 = sessionStorage.getItem(STORAGE_KEYS.capsPrivateKeyB64) || '';
  if (!b64) {
    return '';
  }
  try {
    return atob(b64);
  } catch (err) {
    return '';
  }
}

// Step 2 form values and generated secrets from sessionStorage.
function dvpSessionValues(extra) {
  extra = extra || {};
  return {
    internal: sessionStorage.getItem(STORAGE_KEYS.internal) || '',
    vm: sessionStorage.getItem(STORAGE_KEYS.vm) || DEFAULTS.vm,
    pod: sessionStorage.getItem(STORAGE_KEYS.pod) || DEFAULTS.pod,
    svc: sessionStorage.getItem(STORAGE_KEYS.service) || DEFAULTS.service,
    nfsH: sessionStorage.getItem(STORAGE_KEYS.nfsHost) || '',
    nfsS: sessionStorage.getItem(STORAGE_KEYS.nfsShare) || DEFAULTS.nfsShare,
    hash: sessionStorage.getItem(STORAGE_KEYS.passwordHash) || '',
    password: sessionStorage.getItem(STORAGE_KEYS.password) || '',
    capsPrivateKeyB64: sessionStorage.getItem(STORAGE_KEYS.capsPrivateKeyB64) || '',
    capsPublicKey: sessionStorage.getItem(STORAGE_KEYS.capsPublicKey) || '',
    master: sessionStorage.getItem(STORAGE_KEYS.master) || '',
    workerIp: sessionStorage.getItem(STORAGE_KEYS.worker) || '',
    pubDomain: (sessionStorage.getItem(STORAGE_KEYS.domain) || '').trim() || DEFAULTS.domain,
    licenseCfg: extra.licenseCfg !== undefined ? extra.licenseCfg : '',
  };
}
