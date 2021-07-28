<blockquote>
<h3 class="text text_h3" style="margin-top: 0;">
  Лицензионный ключ доступа Deckhouse Platform Enterprise Edition
</h3>
<div style="width: 500px;">
<p class="text">Лицензионный ключ используется компонентами Deckhouse для доступа к геораспределенному container registry, в котором хранятся все используемые Deckhouse образы.</p>

<p class="text">Приведенные на данной странице команды и файлы конфигурации <strong>сгенерированы</strong> с учетом введенного лицензионного ключа.</p>
</div>

<div style="width: 500px;">
{% include request_access_form.html %}
</div>

<h3 class="text text_h3">
  Введите лицензионный ключ
</h3>

<div class="form" style="width: 500px;">
  <div class="form__row">
    <label class="label">
      Лицензионный ключ
    </label>
    <input class="textfield"
      type="text" license-token name="license-token"
      autocomplete="off" />
  </div>
</div>
</blockquote>

<script>
$(document).ready(function() {

    tokenInputElement = $('[license-token]');
    if ($.cookie("demotoken") || $.cookie("license-token")) {
        let token = $.cookie("license-token") ? $.cookie("license-token") : $.cookie("demotoken");
        tokenInputElement.val(token);
    }

    tokenInputElement.change(function () {
        $.cookie('license-token', $(this).val(), {path: '/' });
        location.reload();
    });
})
</script>
