<script type="text/javascript" src='{% javascript_asset_tag getting-started %}[_assets/js/getting-started.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag getting-started-access %}[_assets/js/getting-started-access.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag bcrypt %}[_assets/js/bcrypt.js]{% endjavascript_asset_tag %}'></script>

Create a user to access the cluster web interfaces:
<ul>
<li><p>Create on the <strong>master node</strong> the <code>user.yml</code> file containing the user account data and access rights:</p>
{% capture includePath %}_includes/getting_started/dvp/{{ page.platform_code }}/partials/user.yml.inc{% endcapture %}
<div markdown="1">
{% include_file "{{ includePath }}" syntax="yaml" %}
</div>
</li>
<li><p>Apply it using the following command on the <strong>master node</strong>:</p>
<div markdown="1">
```shell
sudo -i d8 k create -f $PWD/user.yml
```
</div>
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
