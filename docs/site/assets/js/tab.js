function openTabAndSaveStatus(evt, linksClass, contentClass, contentId, storeKey = null, storeVal = null) {
    openTab(evt, linksClass, contentClass, contentId);
    if (storeKey && storeVal) {
        sessionStorage.setItem(storeKey, storeVal);
    }
}

function openTab(evt, linksClass, contentClass, contentId) {
  var i, tabcontent, tablinks;

  tabcontent = document.getElementsByClassName(contentClass);
  for (i = 0; i < tabcontent.length; i++) {
    tabcontent[i].style.display = "none";
  }

  tablinks = document.getElementsByClassName(linksClass);
  for (i = 0; i < tablinks.length; i++) {
    tablinks[i].className = tablinks[i].className.replace(" active", "");
  }

  document.getElementById(contentId).style.display = "block";
  evt.currentTarget.className += " active";
}

// Activate a tab button without a real click event and without writing to sessionStorage.
function activateTabBtn(btn) {
  var linksClass, contentClass, block, onclickStr, match;

  // Derive classes and target block id from the onclick attribute.
  onclickStr = btn.getAttribute("onclick") || "";
  match = onclickStr.match(/openTab\w*\(event,\s*'([^']+)',\s*'([^']+)',\s*'([^']+)'/);
  if (!match) return;

  linksClass  = match[1];
  contentClass = match[2];
  var blockId = match[3];

  var tabcontent = document.getElementsByClassName(contentClass);
  for (var i = 0; i < tabcontent.length; i++) {
    tabcontent[i].style.display = "none";
  }

  var tablinks = document.getElementsByClassName(linksClass);
  for (var i = 0; i < tablinks.length; i++) {
    tablinks[i].className = tablinks[i].className.replace(" active", "");
  }

  block = document.getElementById(blockId);
  if (block) block.style.display = "block";
  btn.className += " active";
}

document.addEventListener("DOMContentLoaded", function () {
  // 1. Restore tab state from sessionStorage.
  // Process in DOM order so outer tabs restore before inner tabs.
  var buttons = document.querySelectorAll("a[data-store-key]");
  buttons.forEach(function (btn) {
    var key = btn.dataset.storeKey;
    var val = btn.dataset.storeVal;
    if (key && val && sessionStorage.getItem(key) === val) {
      activateTabBtn(btn);
    }
  });

  // 2. Activate tabs for URL hash anchor (overrides sessionStorage restore).
  var hash = window.location.hash;
  if (hash) {
    try {
      var target = document.querySelector(hash);
      if (target) {
        // Walk up the DOM, collecting every .tabs__content ancestor.
        var panels = [];
        var node = target.parentElement;
        while (node) {
          if (node.classList && node.classList.contains("tabs__content")) {
            panels.unshift(node); // prepend so outermost comes first
          }
          node = node.parentElement;
        }

        // Activate outermost → innermost so nested tabs open correctly.
        panels.forEach(function (panel) {
          var panelId = panel.id;
          if (!panelId) return;
          var btn = document.querySelector("a[onclick*=\"'" + panelId + "'\"]");
          if (btn) activateTabBtn(btn);
        });

        target.scrollIntoView({ behavior: "smooth", block: "start" });
      }
    } catch (e) {
      // Invalid selector (e.g. hash with special chars) — ignore.
    }
  }
});
