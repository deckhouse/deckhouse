## How to build static assets for hubble-ui-frontend

```bash
git clone --depth 1 --branch v{{ $hubbleUIVersion }} {{ $.SOURCE_REPO }}/cilium/hubble-ui.git /src
export "TARGETOS=linux"
export "TARGETARCH=amd64"
cd /src
npm --target_arch=${TARGETARCH} install
export "NODE_ENV=production"
npm run build
chown -R 64535:64535 /src/server/public
```
