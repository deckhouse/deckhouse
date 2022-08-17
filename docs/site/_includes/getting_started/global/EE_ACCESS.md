<div class="license-form__wrap">
<div class="license-form-enter">
<h3 class="text text_h3">
  Enter license key
</h3>

<div class="form form--inline" style="width: 500px;">
  <div class="form__row">
    <label class="label">
      License key
    </label>
    <input id="license-token-input" class="textfield"
      type="text" license-token name="license-token"
      autocomplete="off" />
  </div>
  <a href="#" id="enter-license-key" class="button button_alt">Enter</a>
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
  Or request license key
</h3>
<div class="button-group">
  <a href="javascript:raOpen()" class="button button_alt">Request 30-day free trial access!</a>
</div>
</div>
</div>
