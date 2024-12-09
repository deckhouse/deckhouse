{% assign revision=include.revision %}

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
      type="text" license-token name="license-token-{{ revision }}"
      autocomplete="off" />
  </div>
  <a href="#" id="enter-license-key-{{ revision }}" class="button button_alt">Ввести</a>
  <span></span>
</div>
</div>

<script>
$(document).ready(function() {

    tokenInputElement-{{ revision }} = $('[license-token-{{ revision }}]');
    if ($.cookie("demotoken") || $.cookie("license-token")) {
        let token = $.cookie("license-token") ? $.cookie("license-token") : $.cookie("demotoken");
        tokenInputElement-{{ revision }}.val(token);
    }

    $.cookie('lang', '{{ page.lang }}');
      $('#enter-license-key-{{ revision }}').click((e)=> {
        e.preventDefault();
        let licenseToken = $('[license-token-{{ revision }}]').val().trim();
        getLicenseToken(licenseToken, {{ revision }})
      });
    
      $('[license-token-{{ revision }}]').keypress((e) => {
          const keycode = (event.keyCode ? event.keyCode : event.which);
          let licenseToken = $('[license-token-{{ revision }}]').val().trim();
          if (keycode == '13') {
            getLicenseToken(licenseToken)
          }
      });
    
      triggerBlockOnItemContent('[license-token]', '.dimmer-block-content', {% if page.ee_only != true %}true{% endif %});
    
      generate_password(true);
      replace_snippet_password();
      sessionStorage.setItem('dhctl-revision','{% if page.ee_only == true %}ee{% else %}ce{% endif %}');

})
</script>

<div class="license-form-request">
<h3 class="text text_h3">
  Нет ключа?
</h3>
<div class="button-group">
  <a href="" data-open-modal="request_access" class="button button_alt">Запросить бесплатный триал</a>
</div>
</div>
</div>
