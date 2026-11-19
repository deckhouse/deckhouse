// The Russian header keeps an empty place under the current item of the product row
// (data-ik-slot="row2-current"), shown in the collapsed card only. This puts the page's
// own section tree there, the way the old header did:
// a clone of .layout-sidebar__sidebar .sidebar, folded by navgoco.
//
// The island never writes into the place and has no children there, so its own
// re-renders leave the tree alone. It can still be created again from scratch (the
// row is rebuilt when the layout flips), which is why a MutationObserver refills an
// empty one.
(function () {
    var SLOT = '[data-ik-slot="row2-current"]';
    var SOURCE = '.layout-sidebar__sidebar .sidebar';

    function fill(slot) {
        var source = document.querySelector(SOURCE);
        if (!source || slot.firstElementChild) return;

        var clone = source.cloneNode(true);
        clone.removeAttribute('id');
        clone.classList.add('header__sidebar-nav');
        clone.addEventListener('click', function (event) {
            event.stopPropagation();
        });
        slot.appendChild(clone);

        if (window.$ && window.$.fn && window.$.fn.navgoco) {
            window.$(clone).navgoco({
                caretHtml: '',
                accordion: false,
                openClass: 'active',
                save: false,
                slide: { duration: 400, easing: 'swing' }
            });
        }
    }

    function fillAll() {
        document.querySelectorAll(SLOT).forEach(fill);
    }

    var queued = false;
    function schedule() {
        if (queued) return;
        queued = true;
        window.requestAnimationFrame(function () {
            queued = false;
            fillAll();
        });
    }

    function start() {
        fillAll();
        new MutationObserver(schedule).observe(
            document.querySelector('[data-ik-island="site-header"]') || document.body,
            { childList: true, subtree: true }
        );
    }

    if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', start);
    else start();
})();
