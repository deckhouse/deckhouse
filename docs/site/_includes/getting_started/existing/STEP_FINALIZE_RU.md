<script type="text/javascript" src='{{ assets["getting-started.js"].digest_path }}'></script>
<script type="text/javascript" src='{{ assets["getting-started-access.js"].digest_path }}'></script>

Если вы не включали в конфигурации Deckhouse другие модули, то единственным запущенным модулем после установки 
Deckhouse обладающим WEB-интерфейсом будет модуль [внутренней документации](../..
/documentation/v1/modules/810-deckhouse-web/). Если вы не пользуетесь сервисами типа [nip.io](https://nip.io) или аналогами, то чтобы получить доступ к WEB-интерфейсу модуля нужно создать соответствующую DNS-запись.  

Создайте DNS-запись для доступа к WEB-интерфейсу модуля документации:
<ul>
  <li>Выясните публичный IP-адрес узла, на котором работает Ingress-контроллер.</li>
  <li>Если у вас есть возможность добавить DNS-запись используя DNS-сервер:
    <ul>    
      <li>Если ваш шаблон DNS-имен кластера является <a href="https://en.wikipedia.org/wiki/Wildcard_DNS_record">wildcard
        DNS-шаблоном</a> (например - <code>%s.kube.my</code>), то добавьте соответствующую wildcard A-запись со значением публичного IP-адреса, который вы получили выше.
      </li>
      <li>
        Если ваш шаблон DNS-имен кластера <strong>НЕ</strong> является <a
              href="https://en.wikipedia.org/wiki/Wildcard_DNS_record">wildcard DNS-шаблоном</a> (например - <code>%s-kube.company.my</code>),
        то добавьте А или CNAME-записи со значением публичного IP-адреса, который вы
        получили выше, для DNS-имени <code example-hosts>deckhouse.example.com</code>.
      </li>
    </ul>
  </li>

  <li><p>Если вы <strong>не</strong> имеете под управлением DNS-сервер: добавьте статическую запись соответствия 
  имени <code example-hosts>deckhouse.example.com</code> публичному IP-адресу узла, на котором работает Ingress-контроллер.
  </p><p>Например, 
  на персональном Linux-компьютере, с которого необходим доступ к сервисам Deckhouse, выполните следующую команду (укажите ваш публичный IP-адрес в переменной <code>PUBLIC_IP</code>) для добавления записей в файл <code>/etc/hosts</code> (для Windows используйте файл <code>%SystemRoot%\system32\drivers\etc\hosts</code>):</p>
{% snippetcut selector="export-ip" %}
```shell
export PUBLIC_IP="<PUBLIC_IP>"
```
{% endsnippetcut %}

<p>Добавьте необходимую запись в файл <code>/etc/hosts</code>:</p>

{% snippetcut selector="example-hosts" %}
```shell
sudo -E bash -c "cat <<EOF >> /etc/hosts
$PUBLIC_IP deckhouse.example.com
EOF
"
```
{% endsnippetcut %}
</li></ul>
