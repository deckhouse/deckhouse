<blockquote>
<h3 class="text text_h3" style="margin-top: 0;">
  Deckhouse Platform Enterprise Edition license key
</h3>
<div style="width: 500px;">
<p class="text">The license key is used by Deckhouse components to access the geo-distributed container registry, where all images used by the Deckhouse are stored.</p>

<p class="text">The commands and configuration files on this page are <strong>generated</strong> using the license key you entered.</p>
</div>

<div style="width: 500px;">
{% include request_access_form.html %}
</div>

<h3 class="text text_h3">
  Enter license key
</h3>

<div class="form" style="width: 500px;">
  <div class="form__row">
    <label class="label">
      License key
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
