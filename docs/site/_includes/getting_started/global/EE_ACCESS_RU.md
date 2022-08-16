<div class="license-form__wrap">
<div class="license-form-enter">
<h3 class="text text_h3">
  Введите лицензионный ключ
</h3>

<div class="form form--inline" style="width: 500px;">
  <div class="form__row">
    <label class="label">
      Лицензионный ключ
    </label>
    <input id="license-token-input" class="textfield"
      type="text" license-token name="license-token"
      autocomplete="off" />
  </div>
  <a href="#" id="enter-license-key" class="button button_alt">Ввести</a>
</div>
</div>

<div class="license-form-request">
<h3 class="text text_h3">
  Или запросить лицензионный ключ
</h3>
<div class="button-group">
  <a href="javascript:raOpen()" class="button button_alt">Запросить бесплатный 30-дневный доступ!</a>
</div>
</div>
</div>

<script>
$(document).ready(function() {
    $('#enter-license-key').click((e)=> {
        e.preventDefault();
        const input = $('[license-token]');
        const wrapper = $('.custom-disabled-block');

        if (input.val() !== '') {
          update_license_parameters(input.val());
          wrapper.removeClass('disabled');
        } else {
          wrapper.addClass('disabled');
        }
    });
})
</script>
