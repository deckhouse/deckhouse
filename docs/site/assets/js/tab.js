function openTabAndSaveStatus(evt, linksClass, contentClass, contentId, storeKey = null, storeVal = null) {
    openTab(evt, linksClass, contentClass, contentId);
    if (storeKey && storeVal) {
        sessionStorage.setItem(storeKey, storeVal);
    }
}

// Returns the tabs block of a tab button, or the document for buttons outside a tabs block.
function getTabScope(el) {
  return (el && el.closest && el.closest(".tabs-block")) || document;
}

// Finds a tab content block by its id: in the tabs block of the button first, then in the whole document.
// A page can have several elements with the same id, for example two tab sets with the same name;
// document.getElementById() returns the first of them, which can belong to another tab set.
// The fallback to the document keeps the calls whose content block is outside the tabs block of the button.
function findTabBlock(scope, id) {
  if (scope && scope !== document && scope.querySelector) {
    var selector = '[id="' + String(id).replace(/["\\]/g, "\\$&") + '"]';
    var own = scope.querySelector(":scope > " + selector) || scope.querySelector(selector);
    if (own) return own;
  }
  return document.getElementById(id);
}

function openTab(evt, linksClass, contentClass, contentId) {
  var i, tabcontent, tablinks, block;
  var trigger = evt.currentTarget || evt;
  var scope = getTabScope(trigger);

  tabcontent = scope.getElementsByClassName(contentClass);
  for (i = 0; i < tabcontent.length; i++) {
    tabcontent[i].style.display = "none";
  }

  tablinks = scope.getElementsByClassName(linksClass);
  for (i = 0; i < tablinks.length; i++) {
    tablinks[i].className = tablinks[i].className.replace(" active", "");
  }

  block = findTabBlock(scope, contentId);
  if (block) block.style.display = "block";
  trigger.className += " active";
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

  var scope = getTabScope(btn);
  var tabcontent = scope.getElementsByClassName(contentClass);
  for (var i = 0; i < tabcontent.length; i++) {
    tabcontent[i].style.display = "none";
  }

  var tablinks = scope.getElementsByClassName(linksClass);
  for (var i = 0; i < tablinks.length; i++) {
    tablinks[i].className = tablinks[i].className.replace(" active", "");
  }

  block = findTabBlock(scope, blockId);
  if (block) block.style.display = "block";
  btn.className += " active";
}

// Returns true for a tab content block: "tabs__content" in the getting started pages,
// "tabs__container--descr" in the Jekyll tabs plugin and in the Hugo tabs shortcode.
function isTabPanel(node) {
  return !!(node.id && node.classList &&
    (node.classList.contains("tabs__content") || node.classList.contains("tabs__container--descr")));
}

// Warns in the console about tab content blocks with the same id: their tabs do not switch correctly.
// Checks only the tab content blocks, not all elements, and runs when the browser is idle.
function warnDuplicateTabIds() {
  var panels = document.querySelectorAll(".tabs__content[id], .tabs__container--descr[id]");
  var seen = Object.create(null);
  var duplicates = [];
  for (var i = 0; i < panels.length; i++) {
    var id = panels[i].id;
    if (seen[id] === 1) duplicates.push(id);
    seen[id] = (seen[id] || 0) + 1;
  }
  if (duplicates.length) {
    console.warn("Tabs: duplicate tab content ids, give every tab set on the page a unique name:", duplicates);
  }
}

// Opens the tabs that contain the element of the URL hash anchor, from the outermost to the innermost, and scrolls to it.
// On page load, scrolls to the element in any case.
// On a hash change, does nothing for an element outside tabs: the browser has already scrolled to it.
function openTabsForHash(hash, onlyInTabs) {
  if (!hash || hash.length < 2) return;
  try {
    // The hash is percent-encoded, for example for Cyrillic heading ids.
    var target = document.getElementById(decodeURIComponent(hash.slice(1)));
    if (!target) return;

    // Walk up the DOM, collecting every tab content block that contains the target.
    var panels = [];
    var node = target.parentElement;
    while (node) {
      if (isTabPanel(node)) {
        panels.unshift(node); // prepend so outermost comes first
      }
      node = node.parentElement;
    }
    if (onlyInTabs && !panels.length) return;

    // Activate outermost → innermost so nested tabs open correctly.
    // Look for the button in the tabs block of the panel: another tab set can have a panel with the same id.
    panels.forEach(function (panel) {
      var selector = "a[onclick*=\"'" + panel.id + "'\"], li[onclick*=\"'" + panel.id + "'\"]";
      var scope = getTabScope(panel);
      var btn = (scope !== document && scope.querySelector(selector)) || document.querySelector(selector);
      if (btn) activateTabBtn(btn);
    });

    target.scrollIntoView({ behavior: "smooth", block: "start" });
  } catch (e) {
    // Malformed hash (e.g. invalid percent-encoding) — ignore.
  }
}

document.addEventListener("DOMContentLoaded", function () {
  // 1. Restore tab state from sessionStorage.
  // Process in DOM order so outer tabs restore before inner tabs.
  var buttons = document.querySelectorAll("[data-store-key]");
  buttons.forEach(function (btn) {
    var key = btn.dataset.storeKey;
    var val = btn.dataset.storeVal;
    if (key && val && sessionStorage.getItem(key) === val) {
      activateTabBtn(btn);
    }
  });

  // 2. Activate tabs for URL hash anchor (overrides sessionStorage restore).
  openTabsForHash(window.location.hash, false);

  // 3. Report duplicate tab content ids without delaying the page.
  if (window.requestIdleCallback) {
    window.requestIdleCallback(warnDuplicateTabIds, { timeout: 5000 });
  } else {
    setTimeout(warnDuplicateTabIds, 2000);
  }
});

// Activate tabs for a link to an anchor inside a hidden tab on the same page, for example from the table of contents.
window.addEventListener("hashchange", function () {
  openTabsForHash(window.location.hash, true);
});
