## Geoproxy service

### Brief description

The geoproxy server acts as a caching server for the IngressNginxController. It is specified as an argument of the `--maxmind-mirror=https://geoproxy:4250/download` parameter.

## Airgap load GeoIP DB

Goal: preload the MaxMind databases into geoproxy storage when the cluster has no access to the Internet or to MaxMind service.

- Expected files format and names
  - Place tarballs exactly as downloaded from MaxMind: `<edition>.tar.gz` (not raw `.mmdb`).
  - Examples: `GeoLite2-City.tar.gz`, `GeoLite2-ASN.tar.gz`, etc. Names must match `spec.geoIP2.maxmindEditionIDs` used by controllers (defaults are `GeoLite2-City` and `GeoLite2-ASN`).

  Full list [editions](https://deckhouse.io/modules/ingress-nginx/cr.html#ingressnginxcontroller-v1-spec-geoip2-maxmindeditionids-element).

- Where files should be placed
  - geoproxy serves files from `/data` inside the geoproxy container.
  - With a StorageClass configured (`.Values.ingressNginx.internal.effectiveStorageClass`), `/data` is a PVC (recommended).

> [!WARNING]
>  - Without a StorageClass, `/data` is `emptyDir` (non‑persistent; data **lost when the Pod is recreated or rescheduled**.).

> [!IMPORTANT]
>  You can configure `spec.geoIP2.maxmindMirror.url` without `maxmindLicenseKey`. In this case geoproxy will only download archives from your mirror (or use the preloaded tarballs) and won't attempt to reach MaxMind directly.

### Preload into PVC (recommended)
  -  Ensure geoproxy is deployed and the PVC for the first replica exists (usually `geo-data-geoproxy-0`).
  -  Create a temporary pod that mounts the PVC and sleeps:


1) Scale sts geoproxy to 0.

```bash
kubectl -n d8-ingress-nginx scale sts/geoproxy --replicas=0
```

2) Create preload Pod on the same Node that geoproxy was deployed.

```yaml
     apiVersion: v1
     kind: Pod
     metadata:
       name: geoproxy-preload
       namespace: d8-ingress-nginx
     spec:
       restartPolicy: Never
       nodeName: $NODENAME # geoproxy StatefullSet Node name.
       containers:
       - name: loader
         image: busybox:1.36
         command: ["sh", "-c", "sleep 3600"]
         volumeMounts:
         - name: geo-data
           mountPath: /data
       volumes:
       - name: geo-data
         persistentVolumeClaim:
           claimName: geo-data-geoproxy-0
```

3) Copy tarballs into the mounted path (repeat per edition):

```bash
kubectl -n d8-ingress-nginx cp GeoLite2-City.tar.gz geoproxy-preload:/data/GeoLite2-City.tar.gz
```

4) Remove the temporary pod:

```bash
kubectl -n d8-ingress-nginx delete pod geoproxy-preload --wait=true
```

5) Scale backward.

```bash
kubectl -n d8-ingress-nginx scale sts/geoproxy --replicas=<previous_number>
```

> [!TIP]
> You can also download a tarball directly from MaxMind:
>
> ```bash
> wget -O GeoLite2-Countrys.tar.gz \
>   'https://download.maxmind.com/app/geoip_download?edition_id=GeoLite2-Country&license_key=LICENSE&suffix=tar.gz'
>```
>
> Replace `edition_id` and `LICENSE` with your actual edition and license key.

## Manual check

  1) Get the node of the pod:

```bash
kubectl -n d8-ingress-nginx get pod geoproxy-0 -o jsonpath='{.spec.nodeName}'; echo
```

  2) On that node, get the PID of the `geoproxy` container:

```bash
CID=$(crictl ps --label io.kubernetes.container.name=geoproxy --label io.kubernetes.pod.name=geoproxy-0 -q)
PID=$(crictl inspect $CID | jq -r .info.pid)
```

  3) Use host wget in the pod network namespace:

```bash
 nsenter -t "$PID" -n wget -qO- http://127.0.0.1:8080/download/GeoLite2-Country.tar.gz | gunzip -t
```

```bash
ls -lah "/proc/$PID/root/data/"
```


> [!NOTE]
> - geoproxy does not need Internet access when files are preloaded; controllers will fetch from the mirror.
> - To refresh controllers quickly, ensure files exist, then watch their logs for successful GeoIP load or check metrics.
