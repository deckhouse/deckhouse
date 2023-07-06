function setupLogoutButton() {
  let flag = false;

  document.querySelectorAll("div").forEach(function (node) {
    if (node.innerHTML.startsWith("Logged in with auth header") === true) {
      let r = new XMLHttpRequest();
      r.open('GET', window.location.origin + "/dex-authenticator/userinfo", false);
      r.onload  = function() {
        let payload = JSON.parse(r.response);

        node.innerHTML = '' +
          '<div>' +
          '<button class="mat-focus-indicator mat-raised-button mat-button-base mat-primary" color="primary">' +
          '<span class="mat-button-wrapper">' +
          '<a href="/logout" style="color:#ffffff;">Click to logout</a>' +
          '</span>' +
          '</button>' +
          '</div>' +
          '<p><hr/></p>';
        node.innerHTML += '<div><h3>Logged in as </h3> Username: <b>' + payload.email + '</b></div>'

        if (payload.groups.length > 0) {
          node.innerHTML += '<br> Groups: <br><ul>'
          for (const group of payload.groups) {
            let fgroup = '<li>' + group  + '</li>';
            node.innerHTML += fgroup
          }
        }

        flag = true;
      };
      r.send(null);
    }
  })

  if (flag !== true) {
    setTimeout(setupLogoutButton, 1000);
  }
}

// Resize dialog window because groups and emails can be long.
function setResizeParent() {
  let parent = document.querySelectorAll('#mat-menu-panel-0').item(0);
  if (parent) {
    parent.style.minWidth = "400px";
  }
  setTimeout(setResizeParent, 500);
}

setupLogoutButton()
setResizeParent()
