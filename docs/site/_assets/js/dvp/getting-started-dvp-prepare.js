// DVP GS step 3: command snippet placeholders (requires constants + getting-started-dvp.js).

document.addEventListener('DOMContentLoaded', function () {
  function runPrepareInit() {
    if (typeof dvp_refresh_command_placeholders === 'function') {
      dvp_refresh_command_placeholders();
    }
  }

  if (typeof generate_caps_ssh_key === 'function') {
    generate_caps_ssh_key(false).then(runPrepareInit).catch(function (err) {
      console.error('CAPS SSH key generation failed:', err);
      runPrepareInit();
    });
    return;
  }

  runPrepareInit();
});
