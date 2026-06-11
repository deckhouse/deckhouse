// DVP GS step 6: password and placeholders (requires getting-started.js + constants + getting-started-dvp.js).

document.addEventListener('DOMContentLoaded', function () {
  if (typeof generate_password === 'function') {
    generate_password();
  }
  if (typeof dvp_refresh_command_placeholders === 'function') {
    dvp_refresh_command_placeholders();
  }
  if (typeof gsReplaceDnsHostnamesInLinks === 'function') {
    gsReplaceDnsHostnamesInLinks(document.querySelector('.post-content') || document);
  }
});
