<ol>
  <li>
<p>На главной странице нажмите кнопку «Установить Deckhouse Kubernetes Platform».</p>
<div style="width: 70%; margin: 16px auto;">
<img src="/images/gs/installer/install-button.png" alt="Установить Deckhouse Kubernetes Platform" style="width: 100%;">
</div>
  </li>
  <li>
<p>Выберите базовые параметры установки:</p>
<ul>
  <li>Редакцию платформы. По умолчанию выбрана Community Edition, выбрать подходящую редакцию можно в выпадающем списке «Добавить источник» в центре.<br>
  При выборе редакции, отличной от Community Edition, введите в открывшемся окне название (любое) и полученный ключ лицензии.
<div style="width: 70%; margin: 16px auto;">
<img src="/images/gs/installer/enter-license-key.png" alt="Базовые параметры установки" style="width: 100%;">
</div>
  </li>
  <li><a href="../../../documentation/v1/reference/release-channels.html">Канал обновления Deckhouse Kubernetes Platform</a>. По умолчанию выбран Stable.</li>
  <li>Версию Kubernetes. По умолчанию выбран режим «Авто», в котором выбирается <a href="../../../documentation/v1/reference/supported_versions.html#kubernetes">текущая актуальная версия</a>.</li>
</ul>
<div style="width: 70%; margin: 16px auto;">
<img src="/images/gs/installer/main-parameters.png" alt="Базовые параметры установки" style="width: 100%;">
</div>
<p>Затем нажмите кнопку «Инфраструктура» для перехода к следующему шагу.</p>
  </li>
  <li>
    <p>В выпадающем списке «Добавить инфраструктуру» в верхней части страницы выберите необходимую облачную платформу.<br>
    В списке доступные облачные провайдеры активны в зависимости от выбранной редакции DKP.</p>
<div style="width: 70%; margin: 16px auto;">
<img src="/images/gs/installer/select-cloud-provider.png" alt="Как выглядит выпадающий список" style="width: 100%;">
</div>
  </li>
  <li>
    <p>В открывшемся справа окне введите необходимые параметры подключения к облачному провайдеру.</p>
<div style="width: 70%; margin: 16px auto;">
{%- if page.platform_code == 'yandex' %}
<img src="/images/gs/installer/ya-cloud-example.png" alt="Окно настроек Yandex Cloud" style="width: 100%;">
{%- endif %}
{%- if page.platform_code == 'dvp-provider' %}
<img src="/images/gs/installer/dvp-cloud-example.png" alt="Окно настроек DVP" style="width: 100%;">
{%- endif %}
{%- if page.platform_code == 'openstack' %}
<img src="/images/gs/installer/openstack-cloud-example.png" alt="Окно настроек OpenStack" style="width: 100%;">
{%- endif %}
{%- if page.platform_code == 'openstack_selectel' %}
<img src="/images/gs/installer/selectel-cloud-example.png" alt="Окно настроек Selectel" style="width: 100%;">
{%- endif %}
{%- if page.platform_code == 'openstack_vk' %}
<img src="/images/gs/installer/vk-cloud-example.png" alt="Окно настроек VK Cloud" style="width: 100%;">
{%- endif %}
{%- if page.platform_code == 'vsphere' %}
<img src="/images/gs/installer/vsphere-cloud-example.png" alt="Окно настроек VMware vSphere" style="width: 100%;">
{%- endif %}
{%- if page.platform_code == 'vcd' %}
<img src="/images/gs/installer/vcd-cloud-example.png" alt="Окно настроек VMware Cloud Director" style="width: 100%;">
{%- endif %}
{%- if page.platform_code == 'zvirt' %}
<img src="/images/gs/installer/zvirt-cloud-example.png" alt="Окно настроек zVirt" style="width: 100%;">
{%- endif %}
</div>
    <p>Нажмите кнопку «Сохранить».</p>
  </li>
  <li>Созданная инфраструктура отобразится в списке на главном экране. Выберите нужный пункт (если создано несколько вариантов) и нажмите кнопку «Параметры кластера».
<div style="width: 70%; margin: 16px auto;">
{%- if page.platform_code == 'yandex' %}
<img src="/images/gs/installer/ya-cloud-infrastructure.png" alt="Как выглядит окно выбора инфраструктуры" style="width: 100%;">
{%- endif %}
{%- if page.platform_code == 'dvp-provider' %}
<img src="/images/gs/installer/dvp-cloud-infrastructure.png" alt="Как выглядит окно выбора инфраструктуры" style="width: 100%;">
{%- endif %}
{%- if page.platform_code == 'openstack' %}
<img src="/images/gs/installer/openstack-cloud-infrastructure.png" alt="Как выглядит окно выбора инфраструктуры" style="width: 100%;">
{%- endif %}
{%- if page.platform_code == 'openstack_selectel' %}
<img src="/images/gs/installer/selectel-cloud-infrastructure.png" alt="Как выглядит окно выбора инфраструктуры" style="width: 100%;">
{%- endif %}
{%- if page.platform_code == 'openstack_vk' %}
<img src="/images/gs/installer/vk-cloud-infrastructure.png" alt="Как выглядит окно выбора инфраструктуры" style="width: 100%;">
{%- endif %}
{%- if page.platform_code == 'vsphere' %}
<img src="/images/gs/installer/vsphere-cloud-infrastructure.png" alt="Как выглядит окно выбора инфраструктуры" style="width: 100%;">
{%- endif %}
{%- if page.platform_code == 'vcd' %}
<img src="/images/gs/installer/vcd-cloud-infrastructure.png" alt="Как выглядит окно выбора инфраструктуры" style="width: 100%;">
{%- endif %}
{%- if page.platform_code == 'zvirt' %}
<img src="/images/gs/installer/zvirt-cloud-infrastructure.png" alt="Как выглядит окно выбора инфраструктуры" style="width: 100%;">
{%- endif %}
</div>
</li>
</ol>
