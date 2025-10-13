---
title: Интеграция со службами Amazon Web Services
permalink: ru/admin/integrations/public/amazon/services.html
lang: ru
---

## Работа с Amazon Elastic Container Registry (ECR)

Чтобы узлы кластера имели доступ к частным репозиториям Amazon ECR:

1. Определите права на чтение образов в политиках репозитория. Важно, чтобы в `Principal` был указан существующий IAM-Role (IAM-роль), привязанная к узлам Deckhouse Kubernetes Platform (DKP).

    Пример политики:

    ```yaml
    {
      "Version": "2012-10-17",
      "Statement": [
        {
          "Sid": "RepositoryRead",
          "Effect": "Allow",
          "Principal": {
            "AWS": "arn:aws:iam::xxx:role/xxx-node"
          },
          "Action": [
            "ecr:BatchCheckLayerAvailability",
            "ecr:BatchGetImage",
            "ecr:DescribeImages",
            "ecr:DescribeRepositories",
            "ecr:GetAuthorizationToken",
            "ecr:GetDownloadUrlForLayer",
            "ecr:ListImages",
            "ecr:ListTagsForResource"
          ]
        }
      ]
    }
    ```

1. Настройте эту политику в AWS Console: Amazon ECR → Private registry → Repositories → требуемый репозиторий → Permissions.

1. Добавьте `ecr:GetAuthorizationToken` в [`additionalRolePolicies`](/modules/cloud-provider-aws/cluster_configuration.html#awsclusterconfiguration-additionalrolepolicies) в AWSClusterConfiguration, чтобы узлы могли автоматически получать токен доступа к образам:

   ```yaml
   additionalRolePolicies:
     - ecr:GetAuthorizationToken
     - ecr:BatchGetImage
     - ecr:DescribeRepositories
   ```

Параметр [additionalRolePolicies](/modules/cloud-provider-aws/cluster_configuration.html#awsclusterconfiguration-additionalrolepolicies) позволяет расширить набор IAM-действий, назначаемых EC2-инстансам, управляемым DKP. Это особенно полезно, если требуется доступ к:

- Amazon ECR;
- Amazon S3;
- другим сервисам AWS, требующим специфических прав.

Если параметр не задан, IAM-роли будут содержать только базовые действия:

```yaml
ec2:DescribeTags
ec2:DescribeInstances
```
