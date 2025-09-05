```bash
# ... Cert gen as in https://gist.github.com/tirumaraiselvan/b7eb1831d25dd9d59a785c11bd46c84b ...
# certs result to shell-test/deployment.yaml secret fields
minikube start
docker build . -f shell-test/Dockerfile -t riptide01/shell-operator:hooks5
docker push riptide01/shell-operator:hooks5
# change image in shell-test/deployment.yaml
kubectl apply -f shell-test/deployment.yaml
kaf svc.yaml
```

<!-- 
kdels test
kdel validatingwebhookconfigurations.admissionregistration.k8s.io test-hooks
kl deployments/shell-operator
 -->