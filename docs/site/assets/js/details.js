$( document ).ready(function() {
  $('.details__summary').on('click tap', function() {
    $(this).closest('.details').toggleClass('active');
  });

  function expandDetailsForHash(hash) {
    if (!hash) return;
    var target = document.getElementById(decodeURIComponent(hash.replace('#', '')));
    if (!target) return;
    var details = $(target).closest('.details');
    if (!details.length) return;
    details.addClass('active');
    setTimeout(function() {
      var stickyHeader = document.querySelector('.header-container');
      var offset = stickyHeader ? stickyHeader.getBoundingClientRect().height : 0;
      var top = details[0].getBoundingClientRect().top;
      if (top < offset) {
        window.scrollBy({ top: top - offset });
      }
    }, 50);
  }

  expandDetailsForHash(window.location.hash);

  $(window).on('hashchange', function() {
    expandDetailsForHash(window.location.hash);
  });
});