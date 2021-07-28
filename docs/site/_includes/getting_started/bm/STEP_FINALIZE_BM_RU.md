<script type="text/javascript" src='{{ assets["getting-started.js"].digest_path }}'></script>
<script type="text/javascript" src='{{ assets["getting-started-access.js"].digest_path }}'></script>

На данном этапе вы создали базовый кластер, который состоит из **единственного** мастера и этого достаточно для ознакомительных целей. При этом, Deckhouse Platform расчитан исключительно на реальные условия, где помимо мастера присутствуют дополнительные рабочие узлы.

Для продолжения ознакомления с Deckhouse Platform потребуется выбрать один из подходов:
<ul><li>Если вам достаточно единственного мастера, то явно сообщите об этом платформе с помощью команды:
{% snippetcut %}
```bash
kubectl patch nodegroup master --type json -p '[{"op": "remove", "path": "/spec/nodeTemplate/taints"}]'
```
{% endsnippetcut %}
</li>
<li>Если вам требуются дополнительные узлы, добавьте их в кластер согласно <a href="/ru/documentation/v1/modules/040-node-manager/faq.html#как-автоматически-добавить-статичный-узел-в-кластер">документации</a> модуля управления узлами.</li></ul>

После — останется ещё три действия.
<ul><li><p>Установить Ingress-контроллер:</p>
{% snippetcut name="ingress-nginx-controller.yml" selector="ingress-nginx-controller-yml" %}
{% include_file "_includes/getting_started/{{ page.platform_code }}/partials/ingress-nginx-controller.yml.inc" syntax="yaml" %}
{% endsnippetcut %}
</li>
<li><p>Создать пользователя для доступа в веб-интерфейсы кластера:</p>
{% snippetcut name="user.yml" selector="user-yml" %}
{% include_file "_includes/getting_started/{{ page.platform_code }}/partials/user.yml.inc" syntax="yaml" %}
{% endsnippetcut %}
</li>
<li>Создать DNS-записи для организации доступа в веб-интерфейсы кластера:
  <ul><li>Выясните публичный адрес узла, на котором работает Ingress-контроллер в вашем случае.</li>
  <li>Если у вас есть возможность добавить DNS-запись используя DNS-сервер, то мы рекомендуем добавить wildcard-запись для <code>*.example.com</code> и публичного IP-адреса.</li>
  <li>Если вы хотите протестировать работу кластера, но не имеете под управлением DNS-сервер, добавьте статические записи соответствия имен конкретных сервисов IP-адресу узла, на котором работает Ingress-контроллер в файл <code>/etc/hosts</code> для Linux (<code>%SystemRoot%\system32\drivers\etc\hosts</code> для Windows):
{% snippetcut selector="example-hosts" %}
```bash
export PUBLIC_IP="<PUT_PUBLIC_IP_HERE>"
sudo -E bash -c "cat <<EOF >> /etc/hosts
$PUBLIC_IP dashboard.example.com
$PUBLIC_IP deckhouse.example.com
$PUBLIC_IP kubeconfig.example.com
$PUBLIC_IP grafana.example.com
$PUBLIC_IP dex.example.com
EOF
"
```
{% endsnippetcut %}
</li></ul>
</li>
</ul>


<script type="text/javascript">
$( document ).ready(function() {
   generate_password();
   update_parameter('dhctl-user-password-hash', 'password', '<GENERATED_PASSWORD_HASH>',  null ,null);
   update_parameter('dhctl-user-password-hash', null, '<GENERATED_PASSWORD_HASH>',  null ,'[user-yml]');
   update_parameter('dhctl-user-password', null, '<GENERATED_PASSWORD>',  null ,'[user-yml]');
   update_parameter('dhctl-user-password', null, '<GENERATED_PASSWORD>',  null ,'code span.c1');
   update_parameter((sessionStorage.getItem('dhctl-domain')||'example.com').replace('%s.',''), null, 'example.com',  null ,'[user-yml]');
});

</script>
