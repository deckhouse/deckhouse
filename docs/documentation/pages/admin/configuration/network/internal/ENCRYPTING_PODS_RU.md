---
title: "Шифрование трафика между подами"
permalink: ru/admin/configuration/network/internal/encrypting-pods.html
lang: ru
---

Для шифрования трафика между подами в Deckhouse Kubernetes Platform (DKP) можно использовать mTLS, реализованный средствами Istio (модуль [`istio`](/modules/istio/)).

<!-- Перенесено с минорными изменениями из https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/istio/#mutual-tls -->

mTLS (mutual TLS) обеспечивает взаимную аутентификацию сервисов с использованием TLS-сертификатов: при всех исходящих запросах проверяется серверный сертификат, а при входящих — клиентский.
После успешной проверки sidecar-proxy идентифицирует удаленный узел и может использовать эти данные для авторизации либо в прикладных целях.

Каждый сервис получает собственный идентификатор в формате `<TrustDomain>/ns/<Namespace>/sa/<ServiceAccount>`.

Здесь:

- `TrustDomain` — домен кластера.
- `ServiceAccount` — имя аккаунта (может быть `default` или заданное пользователем).

Этот идентификатор используется в качестве удостоверяемого имени в TLS-сертификатах.

Настройки можно переопределить на уровне пространств имён (namespace).

## Пример настройки mTLS

<!-- взято и немного изменено из ПМИ от архитекторов-->

1. Включите модуль `istio`:

   ```shell
   d8 k create -f -<<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: istio
   spec:
     version: 2
     enabled: true
   EOF
   ```

1. Создайте пространство имен, добавьте лейблы:

   ```bash
   d8 k create namespace test-istio-mtls
   d8 k label namespace test-istio-mtls istio-injection=enabled
   d8 k label namespace test-istio-mtls security.deckhouse.io/pod-policy=privileged
   ```

1. Добавьте политику, определяющую режим работы  mTLS.
   Возможные режимы:

   - `PERMISSIVE` (используется по умолчанию) — шифрованный трафик между подами разрешён, но допускается и незашифрованный.
   - `STRICT` — допускается только зашифрованный трафик.

   Пример применения режима `PERMISSIVE`:

   ```shell
   d8 k -n test-istio-mtls create -f -<<EOF
   apiVersion: security.istio.io/v1beta1
   kind: PeerAuthentication
   metadata:
     name: default
   spec:
     mtls:
       mode: PERMISSIVE
   EOF
   ```

1. Создайте Deployment:

   - Если используется публичный registry:

     ```bash
     d8 k -n test-istio-mtls create deployment webserver --image=docker.io/library/nginx:1.26-alpine --port 80
     ```

   - Если используется all-in-one image (замените адрес на свой):

     ```bash
     d8 k -n test-istio-mtls create deployment webserver --image=registry.company.network/localrepo/all-in-one-image:0.1 --port 80 -- /bin/sh -c 'nginx -g "daemon off;"'
     ```

1. Опубликуйте Deployment:

   ```bash
   d8 k -n test-istio-mtls expose deployment webserver --port=80
   ```

1. Получите IP-адрес пода и имя узла, на котором он работает. Эта информация понадобится для запуска `tcpdump` на соответствующем узле:

   ```bash
   d8 k -n test-istio-mtls get pods -l app=webserver -o wide
   ```

   Пример вывода:

   ```console
   NAME                    READY   STATUS    RESTARTS   AGE   IP             NODE                                        NOMINATED NODE   READINESS GATES
   webserver-76d6c9b8c-9mdtb   2/2     Running   0          48m   10.111.1.122   test-worker-e36e4712-5948b-sp9t8   <none>           <none>
   ```

1. Подключитесь к узлу по SSH от root-пользователя и запустите `tcpdump`:

   ```bash
   tcpdump -A -v -i any host 10.111.1.122 and port 80
   ```

1. Создайте Deployment client в этом же пространстве имён, чтобы он был участником mesh-сети:

   - При использовании образов из публичных registry:

     ```bash
     d8 k -n test-istio-mtls create deployment client --image=docker.io/library/alpine:3.21 -- /bin/sh -c "sleep infinity"
     ```

   - При использовании образа all-in-one-image (замените адрес на свой):

     ```bash
     d8 k -n test-istio-mtls create deployment client --image=registry.company.network/localrepo/all-in-one-image:0.1 -- /bin/sh -c "sleep infinity"
     ```

1. После создания пода сделайте запрос к сервису `webserver`:

   ```bash
   d8 k -n test-istio-mtls exec -ti deployments/client -- wget -S --spider --timeout 1 webserver`
   ```

   В `tcpdump` будет виден только зашифрованный трафик.

   ```shell
   10.111.91.68.http > 10.111.91.167.54204: Flags [P.], cksum 0xd7c7 (incorrect -> 0xe83a), seq 1:3033, ack 518, win 134, options [nop,nop,TS val 3160034944 ecr 2239691194], length 3032: HTTP
   E....]@.@.T.
   o[D
   o[..P..'...;.i8...........
   .ZN..~......z...v...T......d.4p
   ..6j..V..G...0 d..U h...e....W.`.....4....;.......T.......3.$... [..%ex.....i<...*.[..C...jC....p.+..............Nt ................{..P%.+.,L......A........B..7}.v../...rg.o.)5......E.X;K8..%,..yb.^f...^+..Ble.j..w3 S......}.. |3...+.=...;H..#...Y...3h.....:.9.w.A.4X..g...|S;.`<....%.y..4.....D.m...../6.7[.......!+.J........2._r.D.C>`.3A..... ..H.^1...P1N....)..%... r.=..s.FB..i^..^....4{;...ED_.\...g`l,q........U*0.#..(......).h..|sc...
   .c(..kd....5C@..5.Z.L..C2 @.7>..y%....R.5........j9........!........H.a....b..].0..~..0x7..+N...o.6........ (%7:.Xr..N......'.O.a.j&v..Ba.....t..q. =U.t.fU.].......2g....Eat.D.3n........*G.N..!LY.......n)......._jL..9RD..gT.lX..p&..=.d..Tq%....qF`.....'..|..$!g..j.d. N%tb^...@- ..`...S:..D.....y...ckA.;}Y,.....X..6...=[,..|AD..._}......W.. ..u3G...<. ..&.0j.,.'!.# ....w..bx..............}U.;..y....J.K..fQ.#]..3.V..=.d._.'....q.;!.9.N......n.7.Zi.>....@...].u.A}.;.....c..s......d.*=..G..9......Nt@....v..s.>.
   ...h....Cm.Z.......n......L.......-_.......r...%Z.....h...........`..8A....yt.t..2d....oH?.1.O&.J....F..b.OV.............E1H..%~..2.H..{.I...=.I.*..2y1p0h..........P.....@r....vk.!".......{..`.3..<,.r&L.....M...t...;.z...Q...1+.,.......:._L.V.....e.. ..!]\..6*}....vT.A>.....A0.....{.?}AH.+R..g.[=?.X...|94....S+!..e...*..M`f...o..b.K#5.....@...5.......o.(.
   ...
   ```

1. Создайте пространство без лейбла `istio-injection=enabled` и выполните запрос к сервису `webserver.test-istio-mtls` (т.е. выполняется запрос из пода, который не является участником mesh-сети):

   ```bash
   d8 k create namespace test-istio-mtls-without-injection
   ```

1. Добавьте Deployment:

   - При использовании образов из публичных registry:

     ```bash
     d8 k -n test-istio-mtls-without-injection create deployment alpine --image=docker.io/library/alpine:3.21 -- /bin/sh -c "sleep infinity"
     ```

   - При использовании образа all-in-one-image (замените адрес на свой):

     ```bash
     d8 k -n test-istio-mtls create deployment client --image=registry.company.network/localrepo/all-in-one-image:0.1 -- /bin/sh -c "sleep infinity"
     ```

1. После создания пода сделайте запрос к сервису `webserver.test-istio-mtls`:

   ```bash
   d8 k -n test-istio-mtls-without-injection exec -ti deployments/alpine -- wget -S --spider --timeout 1 webserver.test-istio-mtls`.
   ```

   В выводе `tcpdump` будут отображаться незашифрованные (plain text) запросы и ответы:

   ```shell
   10.111.91.211.54424 > 10.111.91.68.http: Flags [P.], cksum 0xcc77 (incorrect -> 0xee57), seq 1:93, ack 1, win 128, options [nop,nop,TS val 305286128 ecr 3834507600], length 92: HTTP, length: 92
   GET / HTTP/1.1
   Host: webserver.test-istio-mtls
   User-Agent: Wget
   Accept: */*
   Connection: close
   
   E...y.@.?...
   o[.
   o[D...P....<Nb......w.....
   .2K....PGET / HTTP/1.1
   Host: webserver.test-istio-mtls
   User-Agent: Wget
   Accept: */*
   Connection: close
   
   09:14:20.960302 lxc4f4a182c887c In  IP (tos 0x0, ttl 64, id 33396, offset 0, flags [DF], proto TCP (6), length 52)
   10.111.91.68.http > 10.111.91.211.54424: Flags [.], cksum 0xcc1b (incorrect -> 0xa850), ack 93, win 128, options [nop,nop,TS val 3834507601 ecr 305286128], length 0
   E..4.t@.@..Z
   o[D
   o[..P..<Nb................
   ...Q.2K.
   09:14:20.964496 lxc4f4a182c887c In  IP (tos 0x0, ttl 64, id 33397, offset 0, flags [DF], proto TCP (6), length 1003)
   10.111.91.68.http > 10.111.91.211.54424: Flags [P.], cksum 0xcfd2 (incorrect -> 0x44eb), seq 1:952, ack 93, win 128, options [nop,nop,TS val 3834507605 ecr 305286128], length 951: HTTP, length: 951
   HTTP/1.1 200 OK
   server: istio-envoy
   date: Fri, 24 Jan 2025 09:14:20 GMT
   content-type: text/html
   content-length: 615
   last-modified: Wed, 14 Aug 2024 05:42:07 GMT
   etag: "66bc43af-267"
   accept-ranges: bytes
   x-envoy-upstream-service-time: 0
   connection: close
   x-envoy-decorator-operation: webserver.test-istio-mtls.svc.cluster.local:80/*
  
   <!DOCTYPE html>
   <html>
   <head>
   <title>Welcome to nginx!</title>
   <style>
   html { color-scheme: light dark; }
   body { width: 35em; margin: 0 auto;
   font-family: Tahoma, Verdana, Arial, sans-serif; }
   </style>
   </head>
   <body>
   <h1>Welcome to nginx!</h1>
   <p>If you see this page, the nginx web server is successfully installed and
   working. Further configuration is required.</p>
   
   <p>For online documentation and support please refer to
   <a href="http://nginx.org/">nginx.org</a>.<br/>
   Commercial support is available at
   <a href="http://nginx.com/">nginx.com</a>.</p>
   
   <p><em>Thank you for using nginx.</em></p>
   </body>
   </html>
   ```
