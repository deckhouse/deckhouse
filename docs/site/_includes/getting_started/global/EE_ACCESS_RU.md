**Введите лицензионный ключ доступа Deckhouse Platform Enterprise Edition**

[Запросить](javascript:raOpen()) бесплатный 30-дневный демо-доступ к Deckhouse Platform Enterprise Edition.

<div class="form" style="width: 500px;">
  <div class="form__row">
    <label class="label" title="DNS template">
      Лицензионный ключ
    </label>
    <input class="textfield"
      type="text" license-token name="license-token"
      autocomplete="off" />
  </div>
</div>

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
