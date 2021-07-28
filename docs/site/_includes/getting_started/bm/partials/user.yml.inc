# [<en>] section containing the parameters of the static user
# [<ru>] секция, описывающая параметры статического пользователя
# [<en>] version of the Deckhouse API
# [<ru>] используемая версия API Deckhouse
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: admin
spec:
  # [<en>] user e-mail
  # [<ru>] e-mail пользователя
  email: admin@example.com
  # [<en>] this is a hash for generated password: <GENERATED_PASSWORD>
  # [<ru>] это хэш сгенерированного пароля: <GENERATED_PASSWORD>
  # [<en>] you might consider changing this
  # [<ru>] возможно, захотите изменить
  password: <GENERATED_PASSWORD_HASH>
