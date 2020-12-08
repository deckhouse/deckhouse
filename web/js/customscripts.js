$('#mysidebar').height($(".nav").height());

$( document ).ready(function() {
    $('#search-input').on("keyup", function (e) {
            if (e.target.value.length > 0 ) $(".header-search__results").addClass("active");
            else $(".header-search__results").removeClass("active");
          });
});

document.addEventListener("DOMContentLoaded", function() {
  /**
   * AnchorJS
   */
  anchors.add('h2,h3,h4,h5');

  $.getJSON('/config/data.json', {_: new Date().getTime()}).done(function (resp) {
    let deckhouseVersionInfo = "unknown";
    if (resp && resp['channel']) {
      deckhouseVersionInfo = resp['channel'];
      $(".releases__menu-item.releases__menu--channel--"+resp['channel']).addClass("releases__menu-item-block-active");
      $(".releases__menu-item-title.releases__menu--channel--"+resp['channel']).addClass("releases__menu-item-title-active");
      var update_channels_list = ['alpha','beta','early-access','stable','rock-solid'];
      if (update_channels_list.indexOf(resp['channel']) < 0 ) {
        $("#releases__stale__block").css({ display: "block" });
      } else {
        $("#releases__stale__block").css({display: "none"});
        if ( resp && resp['version'] && (resp['version'] != "dev" && resp['version'] != "dev" ) )  {
          deckhouseVersionInfo = deckhouseVersionInfo + ' (' + resp['version'] + ')';
        };
      };
    } else {
      console.log('UpdateChannel is not defined.');
    };
    $(".updatechannel__content").text(deckhouseVersionInfo);
    $(".updatechannel__content").removeClass("disable");
  });

});

$( document ).ready(function() {
    //this script says, if the height of the viewport is greater than 800px, then insert affix class, which makes the nav bar float in a fixed
    // position as your scroll. if you have a lot of nav items, this height may not work for you.
    var h = $(window).height();
    //console.log (h);
    if (h > 800) {
        // $( "#mysidebar" ).attr("class", "nav affix");
    }
    // activate tooltips. although this is a bootstrap js function, it must be activated this way in your theme.
    $('[data-toggle="tooltip"]').tooltip({
        placement : 'top'
    });

});

// needed for nav tabs on pages. See Formatting > Nav tabs for more details.
// script from http://stackoverflow.com/questions/10523433/how-do-i-keep-the-current-tab-active-with-twitter-bootstrap-after-a-page-reload
$(function() {
    var json, tabsState;
    $('a[data-toggle="pill"], a[data-toggle="tab"]').on('shown.bs.tab', function(e) {
        var href, json, parentId, tabsState;

        tabsState = localStorage.getItem("tabs-state");
        json = JSON.parse(tabsState || "{}");
        parentId = $(e.target).parents("ul.nav.nav-pills, ul.nav.nav-tabs").attr("id");
        href = $(e.target).attr('href');
        json[parentId] = href;

        return localStorage.setItem("tabs-state", JSON.stringify(json));
    });

    tabsState = localStorage.getItem("tabs-state");
    json = JSON.parse(tabsState || "{}");

    $.each(json, function(containerId, href) {
        return $("#" + containerId + " a[href=" + href + "]").tab('show');
    });

    $("ul.nav.nav-pills, ul.nav.nav-tabs").each(function() {
        var $this = $(this);
        if (!json[$this.attr("id")]) {
            return $this.find("a[data-toggle=tab]:first, a[data-toggle=pill]:first").tab("show");
        }
    });
});
