<script type="text/javascript" src='{% javascript_asset_tag getting-started %}[_assets/js/getting-started.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag getting-started-access %}[_assets/js/getting-started-access.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag bcrypt %}[_assets/js/bcrypt.js]{% endjavascript_asset_tag %}'></script>

Создайте пользователя для доступа в веб-интерфейсы кластера:
<ul>
<li><p>Создайте на <strong>master-узле</strong> файл <code>user.yml</code> содержащий описание учетной записи пользователя и прав доступа:</p>
{% snippetcut name="user.yml" selector="user-yml" %}
{% include_file "_includes/getting_started/dvp/{{ page.platform_code }}/partials/user.yml.inc" syntax="yaml" %}
{% endsnippetcut %}
</li>
<li><p>Примените его, выполнив на <strong>master-узле</strong> следующую команду:</p>
{% snippetcut %}
```shell
sudo -i d8 k create -f user.yml
```
{% endsnippetcut %}
</li>
</ul>


<script type="text/javascript">
$(document).ready(function () {
    generate_password(true);
    update_parameter('dhctl-user-password-hash', 'password', '<GENERATED_PASSWORD_HASH>', null, null);
    update_parameter('dhctl-user-password-hash', null, '<GENERATED_PASSWORD_HASH>', null, '[user-yml]');
    update_parameter('dhctl-user-password', null, '<GENERATED_PASSWORD>', null, '[user-yml]');
    update_parameter('dhctl-user-password', null, '<GENERATED_PASSWORD>', null, 'code span.c1');
    update_domain_parameters();
    config_highlight();
});

</script>
