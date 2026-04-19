d8 k create -f - <<EOF
---
apiVersion: deckhouse.io/v1alpha2
kind: Project
metadata:
  name: test-project
spec:
  description: test-project
  projectTemplateName: default
  parameters:
    # [<ru>] Квоты проекта.
    # [<en>] Project quotas.
    resourceQuota:
      requests:
        cpu: 20
        memory: 20Gi
      limits:
        cpu: 20
        memory: 20Gi
    # [<en>] Project administrators.
    # [<ru>] Администраторы проекта.
    administrators:
      - subject: User
        name: test-user
EOF
