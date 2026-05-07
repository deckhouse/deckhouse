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
});
