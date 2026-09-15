/**
 * Documentation contents on a narrow screen.
 *
 * The header used to do this by cloning `.layout-sidebar__sidebar .sidebar`
 * into its own active menu item and re-running navgoco on the copy. The island
 * has no place to put someone else's subtree, and borrowing one is what made
 * the header and the page impossible to change separately.
 *
 * So the sidebar is shown where it already is: it sits in the DOM on every page
 * that has one and is merely hidden below 1024px. What this adds is a button, a
 * dialog role for as long as the panel is open, and the focus handling that
 * turns a panel covering the page into one a keyboard can actually use.
 */
(function () {
  'use strict';

  var NARROW = '(max-width: 1023px)';
  var PANEL_ID = 'docs-toc-panel';

  var FOCUSABLE = [
    'a[href]',
    'button:not([disabled])',
    'input:not([disabled]):not([type="hidden"])',
    'select:not([disabled])',
    'textarea:not([disabled])',
    '[tabindex]:not([tabindex="-1"])'
  ].join(',');

  function init() {
    var panel = document.querySelector('.layout-sidebar__sidebar');
    if (!panel || !panel.querySelector('.sidebar')) return;

    var host = document.querySelector('.layout-sidebar__content');
    if (!host) return;

    var ru = document.documentElement.lang === 'ru';
    var open = false;
    var previousOverflow = null;

    if (!panel.id) panel.id = PANEL_ID;

    var trigger = document.createElement('button');
    trigger.type = 'button';
    trigger.className = 'docs-toc';
    trigger.setAttribute('aria-expanded', 'false');
    trigger.setAttribute('aria-controls', panel.id);
    trigger.innerHTML = '<span class="docs-toc__label"></span><span class="docs-toc__arrow"></span>';
    trigger.querySelector('.docs-toc__label').textContent = ru ? 'Содержание' : 'Contents';

    var close = document.createElement('button');
    close.type = 'button';
    close.className = 'docs-toc-close';
    close.textContent = ru ? 'Закрыть' : 'Close';

    // Inside the panel, first: the close control has to be the first thing a
    // keyboard reaches when the panel opens, not the last thing on the page.
    panel.insertBefore(close, panel.firstChild);

    // After the heading rather than before it: a control ahead of the page's
    // own <h1> changes what the page announces itself as.
    //
    // The heading is not a direct child here - it sits inside `.docs__wrap-title`
    // - so the insertion point is that wrapper, whatever it happens to be. The
    // first version compared parents, found no match, and put the button first
    // on the page: the very thing this comment says to avoid.
    var heading = host.querySelector('h1');
    var afterHeading = heading;
    while (afterHeading && afterHeading.parentNode !== host) {
      afterHeading = afterHeading.parentNode;
    }

    if (afterHeading) {
      host.insertBefore(trigger, afterHeading.nextSibling);
    } else {
      host.insertBefore(trigger, host.firstChild);
    }

    function focusable() {
      return Array.prototype.filter.call(panel.querySelectorAll(FOCUSABLE), function (el) {
        return el === close || el.offsetParent !== null;
      });
    }

    function onKeydown(event) {
      if (event.key === 'Escape') {
        event.preventDefault();
        setOpen(false);
        return;
      }
      if (event.key !== 'Tab') return;

      // Tab stays inside the panel: it covers the page, so what is behind it is
      // gone visually and has to be out of reach too.
      var items = focusable();
      if (items.length === 0) return;

      var first = items[0];
      var last = items[items.length - 1];
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    }

    function setOpen(next) {
      if (next === open) return;
      open = next;

      document.body.classList.toggle('docs-toc-open', open);
      trigger.setAttribute('aria-expanded', String(open));

      if (open) {
        // The role is added only while the panel is a panel. On a wide screen
        // this element is an ordinary column of the layout and must not
        // announce itself as a dialog.
        panel.setAttribute('role', 'dialog');
        panel.setAttribute('aria-modal', 'true');
        panel.setAttribute('aria-label', ru ? 'Содержание' : 'Contents');

        // Remember what the page had rather than assuming it had nothing.
        previousOverflow = document.body.style.overflow;
        document.body.style.overflow = 'hidden';

        document.addEventListener('keydown', onKeydown, true);
        close.focus();
      } else {
        panel.removeAttribute('role');
        panel.removeAttribute('aria-modal');
        panel.removeAttribute('aria-label');

        if (previousOverflow !== null) {
          document.body.style.overflow = previousOverflow;
          previousOverflow = null;
        }

        document.removeEventListener('keydown', onKeydown, true);
        trigger.focus();
      }
    }

    trigger.addEventListener('click', function () { setOpen(!open); });
    close.addEventListener('click', function () { setOpen(false); });

    // Following a link leaves the page anyway; closing first keeps the scroll
    // lock from surviving into a restored session.
    panel.addEventListener('click', function (event) {
      if (event.target.closest && event.target.closest('a')) setOpen(false);
    });

    if (window.matchMedia) {
      var narrow = window.matchMedia(NARROW);
      var onChange = function (event) { if (!event.matches) setOpen(false); };
      if (narrow.addEventListener) narrow.addEventListener('change', onChange);
      else if (narrow.addListener) narrow.addListener(onChange);
    }
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
