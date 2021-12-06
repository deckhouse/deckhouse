<script type="text/javascript" src='{{ assets["getting-started.js"].digest_path }}'></script>
<script type="text/javascript" src='{{ assets["getting-started-access.js"].digest_path }}'></script>

Если вы не включали в конфигурации Deckhouse другие модули, то единственным запущенным модулем после установки
Deckhouse обладающим WEB-интерфейсом будет модуль [внутренней документации](../..
/documentation/v1/modules/810-deckhouse-web/). Если вы не пользуетесь сервисами типа [nip.io](https://nip.io) или аналогами, то чтобы получить доступ к WEB-интерфейсу модуля нужно создать соответствующую DNS-запись.

Create a DNS record to access a WEB interface of the documentation module:
<ul>
<li>Discover public IP address of the node where the Ingress controller is running.</li>
  <li>If you have the DNS server and you can add a DNS records:
  <ul>
    <li>If your cluster DNS name template is a <a href="https://en.wikipedia.org/wiki/Wildcard_DNS_record">wildcard DNS</a> (e.g. - <code>%s.kube.my</code>), then add a corresponding wildcard A record containing the public IP, you've discovered previously.
    </li>
    <li>If your cluster DNS name template is <strong>NOT</strong> a <a
            href="https://en.wikipedia.org/wiki/Wildcard_DNS_record">wildcard DNS</a> (e.g. - <code>%s-kube.company.my</code>), then add А or CNAME record containing the public IP, you've discovered previously, for the <code example-hosts>deckhouse.example.com</code> service DNS name:
      </li>
    </ul>
  </li>
  <li><p>If you <strong>don't have a DNS server</strong>: on your PC add static entries (specify your public IP address in the <code>PUBLIC_IP</code>variable) that match the <code example-hosts>deckhouse.example.com</code> to the public IP to the <code>/etc/hosts</code> file for Linux (<code>%SystemRoot%\system32\drivers\etc\hosts</code> for Windows):</p>
{% snippetcut selector="export-ip" %}
```shell
export PUBLIC_IP="<PUBLIC_IP>"
```
{% endsnippetcut %}

  <p>Add an entry to the <code>/etc/hosts</code> file:</p>
{% snippetcut selector="example-hosts" %}
```shell
sudo -E bash -c "cat <<EOF >> /etc/hosts
$PUBLIC_IP deckhouse.example.com
EOF
"
```
{% endsnippetcut %}
</li></ul>
