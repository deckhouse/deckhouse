<div class="license-form__wrap">
<div class="license-form-enter">
<h3 class="text text_h3">
  Введите лицензионный ключ
</h3>

<div class="form form--inline">
  <div class="form__row" style="max-width: 383px;">
    <label class="label">
      Лицензионный ключ
    </label>
    <input id="license-token-input" class="textfield"
      type="text" license-token name="license-token"
      autocomplete="off" />
  </div>
  <a href="#" id="enter-license-key" class="button button_alt">Ввести</a>
  <span></span>
</div>
</div>

<script>
$(document).ready(function() {

    tokenInputElement = $('[license-token]');
    if ($.cookie("demotoken") || $.cookie("license-token")) {
        let token = $.cookie("license-token") ? $.cookie("license-token") : $.cookie("demotoken");
        tokenInputElement.val(token);
    }
})
</script>

<div class="license-form-request">
<h3 class="text text_h3">
  Нет ключа?
</h3>
<div class="button-group">
  <a href="javascript:raOpen()" class="button button_alt">Запросить бесплатный 30-дневный доступ!</a>
</div>
</div>
</div>
