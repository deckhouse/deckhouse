$( document ).ready(function() {
  $('.details__summary').on('click tap', function() {
    var $details = $(this).closest('.details');
    $details.toggleClass('active');
    if ($details.hasClass('active')) {
      requestAnimationFrame(function() {
        $details.find('.slick-initialized').each(function() {
          $(this).slick('setPosition');
        });
        if (window.refreshInstallerCarousels) {
          window.refreshInstallerCarousels();
        }
      });
    }
  });
});
