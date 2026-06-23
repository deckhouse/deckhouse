$( document ).ready(function() {
  $('.details__summary').on('click tap', function() {
    var $details = $(this).closest('.details');
    $details.toggleClass('active');
    if ($details.hasClass('active') && window.refreshInstallerCarousels) {
      requestAnimationFrame(function() {
        window.refreshInstallerCarousels();
      });
    }
  });

  function expandDetailsForHash(hash) {
    if (!hash) return;
    var target = document.getElementById(decodeURIComponent(hash.replace('#', '')));
    if (!target) return;
    var details = $(target).closest('.details');
    if (!details.length) return;
    var stickyHeader = document.querySelector('.header-container');
    target.style.scrollMarginTop = (stickyHeader ? stickyHeader.getBoundingClientRect().height : 0) + 'px';
    details.addClass('active');
    target.scrollIntoView();
  }

  expandDetailsForHash(window.location.hash);

  $(window).on('hashchange', function() {
    expandDetailsForHash(window.location.hash);
  });
});