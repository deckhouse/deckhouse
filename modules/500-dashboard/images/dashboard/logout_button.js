function setupLogoutButton() {
  let flag = false;

  document.querySelectorAll("div").forEach(function (node) {
    if (node.innerHTML.startsWith("Logged in with auth header") === true) {
      node.innerHTML += '<div><br><a class="mat-menu-content" href="/logout">Click to logout</a></div>';
      flag = true;
    }
  })

  if (flag !== true) {
    setTimeout(setupLogoutButton, 1000);
  }
}

setupLogoutButton()
