$('#mysidebar').height($(".nav").height());

$( document ).ready(function() {
    $('#search-input').on("keyup", function (e) {
            if (e.target.value.length > 0 ) $(".search__results").addClass("active");
            else $(".search__results").removeClass("active");
          });
});

document.addEventListener("DOMContentLoaded", function() {
    /**
    * AnchorJS
    */
    if (window.anchors_disabled != true) {
        anchors.add('h2,h3,h4,h5');
        anchors.add('.anchored');
    }

});

$( document ).ready(function() {
    //this script says, if the height of the viewport is greater than 800px, then insert affix class, which makes the nav bar float in a fixed
    // position as your scroll. if you have a lot of nav items, this height may not work for you.
    var h = $(window).height();
    //console.log (h);
    if (h > 800) {
        // $( "#mysidebar" ).attr("class", "nav affix");
        // $( "#mysidebar" ).attr("class", "nav");
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

$(document).ready(function() {
    var $notice = $('#notice');
    var $notice_collapse = $('#notice-collapse');
    var $notice_expand = $('#notice-expand');
    var notice_state = localStorage.getItem('notice-state') || 'expanded';

    function switchNotice(state) {
        $notice.attr('data-state', state);
        localStorage.setItem('notice-state', state);
    }

    switchNotice(notice_state);

    $notice_collapse.on('click', (e) => {
        e.preventDefault();
        switchNotice('collapsed');
    })
    $notice_expand.on('click', (e) => {
        e.preventDefault();
        switchNotice('expanded')
    })
});

/* features tabs */

$(document).ready(function() {
    $('[data-features-tabs-trigger]').on('click', function() {
        var name = $(this).attr('data-features-tabs-trigger');
        var $parent = $(this).closest('[data-features-tabs]');
        var $triggers = $parent.find('[data-features-tabs-trigger]');
        var $contents = $parent.find('[data-features-tabs-content]');
        var $content = $parent.find('[data-features-tabs-content=' + name + ']');

        $triggers.removeClass('active');
        $contents.removeClass('active');

        $(this).addClass('active');
        $content.addClass('active');
    })
});

// Clipbord copy functionality
var action_toast_timeout;
function showActionToast(text) {
  clearTimeout(action_toast_timeout);
  var action_toast = $('.action-toast');
  action_toast.text(text).fadeIn()
  action_toast_timeout = setTimeout(function(){ action_toast.fadeOut() }, 5000);
}

$(document).ready(function(){
  new ClipboardJS('[data-snippetcut-btn-name-ru]', {
    text: function(trigger) {
      showActionToast('Скопировано в буфер обмена')
      return $(trigger).closest('[data-snippetcut]').find('[data-snippetcut-name]').text();
    }
  });
  new ClipboardJS('[data-snippetcut-btn-name-en]', {
    text: function(trigger) {
      showActionToast('Has been copied to clipboard')
      return $(trigger).closest('[data-snippetcut]').find('[data-snippetcut-name]').text();
    }
  });
  new ClipboardJS('[data-snippetcut-btn-text-en]', {
    text: function(trigger) {
      showActionToast('Has been copied to clipboard')
      return $(trigger).closest('[data-snippetcut]').find('[data-snippetcut-text]').text();
    }
  });
  new ClipboardJS('[data-snippetcut-btn-text-ru]', {
    text: function(trigger) {
      showActionToast('Скопировано в буфер обмена')
      return $(trigger).closest('[data-snippetcut]').find('[data-snippetcut-text]').text();
    }
  });

});

// GDPR

$(document).ready(function(){
    var $gdpr = $('.gdpr');
    var $gdpr_button = $('.gdpr__button');
    var gdpr_status = $.cookie('gdpr-status');

    if (!gdpr_status || gdpr_status != 'accepted') {
        $gdpr.show();
    }

    $gdpr_button.on('click', function() {
        $gdpr.hide();
        $.cookie('gdpr-status', 'accepted', {path: '/' ,  expires: 3650 });
    })
});

$(document).ready(function(){
  const tables = $('table');

  if (tables.length === 0) {
    return;
  }

  tables.each((_, table) => {
    $(table).wrap("<div class='table-wrapper'></div>")
  })
});

const openDiagram = function () {
  const button = $('[data-open-scheme]');
  const wrap = $('.functionality-block__diagram-wrap')
  const wrapHeight = wrap.height()
  const imageHeight = $('.functionality-block__diagram-wrap img').height();

  $(button).click(() => {
    if (wrap.hasClass('open')) {
      wrap.removeClass('open');
      wrap.height(wrapHeight);
      button.attr('data-open-scheme') === 'ru' ? button.text('Подробнее') : button.text('Show');
    } else {
      wrap.addClass('open');
      wrap.height(imageHeight);
      button.attr('data-open-scheme') === 'ru' ? button.text('Скрыть') : button.text('Hide');
    }
  })
}

function changeHandler(e) {
  e.style.color = "#02003E";
  if (e.value === "telegram") {
    $('.nickname.hidden').removeClass('hidden');
    $('.nickname input').attr('required', 'required');
  } else {
    $('.nickname').addClass('hidden');
    $('.nickname.hidden input').removeAttr('required');
  }
}

document.addEventListener('DOMContentLoaded', () => {
  let header = document.querySelector('header');
  let lastScrollTop = 0;
  let topOffsetToTransform = 25;

  const headerTransforms = () => {
    let top = window.scrollY

    changeShadow(top)
    changeOffset(top)

    lastScrollTop = top
  }

  window.onscroll = headerTransforms

  const changeShadow = (top) => {
    if (!header.classList.contains('header_float') && top >=
      topOffsetToTransform) {
      header.classList.add('header_float')
    }
    else if (header.classList.contains('header_float') && top <
      topOffsetToTransform) {
      header.classList.remove('header_float')
    }
  }

  const changeOffset = (top) => {
    const notificationBar = header.querySelector('.notification-bar')

    if (notificationBar === null) {
      return
    }

    if (lastScrollTop < top && !header.classList.contains('header_small') && top >
      topOffsetToTransform) {
      header.classList.add('header_small')
      header.style.transform = `translateY(-${notificationBar.offsetHeight}px)`
    }
    else if (lastScrollTop > top && header.classList.contains('header_small')) {
      header.classList.remove('header_small')
      header.removeAttribute('style')
    }
  }
})

window.addEventListener("load", function() {
  openDiagram()
});
