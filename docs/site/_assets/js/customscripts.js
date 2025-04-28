$('#mysidebar').height($(".nav").height());

$( document ).ready(function() {
    $('#search-input').on("keyup", function (e) {
            if (e.target.value.length > 0 ) $(".search__results").addClass("active");
            else $(".search__results").removeClass("active");
          });
          $('a.lang-switcher').each(function() {
            let pageDomain = window.location.hostname;
            if (window.location.pathname.startsWith('/ru/')) {
              $(this).attr('href', window.location.href.replace('/ru/', '/en/'))
            } else if (window.location.pathname.startsWith('/en/')) {
              $(this).attr('href', window.location.href.replace('/en/', '/ru/'))
            } else {
              switch (pageDomain) {
                case 'deckhouse.io':
                  $(this).attr('href', window.location.href.replace('deckhouse.io', 'deckhouse.ru'))
                  break;
                case 'deckhouse.ru':
                  $(this).attr('href', window.location.href.replace('deckhouse.ru', 'deckhouse.io'))
                  break;
                case 'ru.localhost':
                  $(this).attr('href', window.location.href.replace('ru.localhost', 'localhost'))
                  break;
                case 'localhost':
                  $(this).attr('href', window.location.href.replace('localhost', 'ru.localhost'))
                  break;
                default:
                  if (pageDomain.includes('deckhouse.ru.')) {
                    $(this).attr('href', window.location.href.replace('deckhouse.ru.', 'deckhouse.'))
                  } else if (pageDomain.includes('deckhouse.')) {
                    $(this).attr('href', window.location.href.replace('deckhouse.', 'deckhouse.ru.'))
                  }
              }
            }
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
    const $gdpr = $('.gdpr');
    const $gdpr_button = $('.gdpr__button');
    const gdpr_status = $.cookie('gdpr-status');
    const cmplz_banner_status = $.cookie('cmplz_banner-status');

    if ((!gdpr_status || gdpr_status !== 'accepted') && cmplz_banner_status !== 'dismissed') {
        $gdpr.css('display', 'flex');
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
  };

  tables.each((_, table) => {
    if($(table).hasClass('table__small')) {
      $(table).wrap("<div class='table-wrapper table-wrapper__small'></div>");
    } else {
      $(table).wrap("<div class='table-wrapper'></div>");
    }
  });
});

$(document).ready(function(){
  const titles = $('.resources__prop_name');
  const links = $('.resources__prop_wrap .anchorjs-link');

  links.each((i, link) => {
    $(link).click((e) => {
      e.stopPropagation();
    })
  })

  titles.each((i, title) => {
    $(title).click(() => {
      const firstList = $(title).parent('.resources__prop_wrap').parent('li').parent('ul');

      if (firstList.hasClass('resources')) return;

      const parentElem = $(title).parent('.resources__prop_wrap').parent('li');

      parentElem.toggleClass('closed');
    })
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
  let top;
  let header = document.querySelector('header');
  let lastScrollTop = 0;
  let topOffsetToTransform = 25;

  const calcScroll = () => {
    top = window.scrollY
    lastScrollTop = top
  }

  window.onscroll = calcScroll
  window.addEventListener('scroll', () => changeOffset(top))

  if (!header.classList.contains('header_float')) {
    window.addEventListener('scroll', () => changeShadow(top))
  }

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

document.addEventListener('DOMContentLoaded', function () {
  const pre = document.querySelectorAll('pre');

  let lang = document.documentElement.lang;

  if (lang.length === 0) {
    if (window.location.href.includes("deckhouse.ru") || window.location.href.includes("ru.localhost")) {
      lang = "ru"
    } else {
      lang = "en"
    }
  }

  if (!($.cookie("lang") === lang)) {
    $.cookie('lang', lang, {path: '/', expires: 365});
  }

  const textTooltip = {
    en: {
      copy: 'Copy',
      copied: 'Copied!',
      error: 'Error!'
    },
    ru: {
      copy: 'Копировать',
      copied: 'Скопировано!',
      error: 'Ошибка!'
    }
  };
  const texts = textTooltip[lang];

  if (pre.length) {
    pre.forEach((el) => {
      el.addEventListener('mouseenter', () => {
        if (el.querySelector('.icon--copy') &&
          !el.querySelector('.icon--copy').classList.contains('show')) {
          el.querySelector('.icon--copy').classList.add('show');
          return;
        };

        const copyBtn = document.createElement('div');
        copyBtn.classList.add('icon--copy');

        el.prepend(copyBtn);

        const copyBtnTippy = tippy(el.querySelector('.icon--copy'), {
          placement: 'left',
          arrow: false,
          animation: 'scale',
          theme: 'light',
          content: texts.copy,
          hideOnClick: false,
          delay: [300, 50],
          offset: [0, 10],
          duration: [300],
        });

        el.querySelector('.icon--copy').addEventListener('click', () => {
          const code = el.querySelector('code');
          if (!code) return;

          navigator.clipboard.writeText(code.textContent).then(r => {
            copyBtnTippy.setContent(texts.copied);

            setTimeout(() => {
              copyBtnTippy.hide()
            }, 1000);
          }, () => {
            copyBtnTippy.setContent(texts.error);

            setTimeout(() => {
              copyBtnTippy.hide()
            }, 1000);
          });
        });

        setTimeout(() => {
          el.querySelector('.icon--copy').classList.add('show')
        }, 0);
      });

      el.addEventListener('mouseleave', (e) => {
        if (!el.querySelector('.icon--copy')) {
          return;
        };

        el.querySelector('.icon--copy').classList.remove('show');

        setTimeout(() => {
          if (!el.querySelector('.icon--copy').classList.contains('show')) {
            el.querySelector('.icon--copy').remove();
          };
        }, 300);
      });
    });
  };
});
