document.addEventListener('DOMContentLoaded', function () {
  const hamburgerCollapse = document.querySelector('.hamburger--collapse');
  const headerSidebar = document.querySelector('.header__sidebar');
  const content = document.querySelector('.content');
  const overlay = document.createElement('div');
  const body = document.querySelector('body');
  const navList = document.querySelector('div .nav__trigger');
  const activeList = document.querySelector('li.active');
  
  if(activeList) {
    navList.textContent = activeList.textContent;
  }


  hamburgerCollapse.addEventListener('click', function() {
    headerSidebar.classList.toggle('show');

    if(headerSidebar.classList.contains('show')) {
      overlay.classList.add('sidebar-overlay');
      content.appendChild(overlay);
      body.classList.add('sidebar-opened');
    } else {
      if(overlay) {
        content.removeChild(overlay);
        body.classList.remove('sidebar-opened');
      }
    }
  })

  window.addEventListener('click', function(e) {
    if(!e.target.matches('.hamburger--collapse')) {
      if(headerSidebar.classList.contains('show')) {
        headerSidebar.classList.remove('show');
        if(overlay) {
          content.removeChild(overlay);
          body.classList.remove('sidebar-opened');
        }
      }
    }
  })

  navList.addEventListener('click', function() {
    const list = document.querySelector('ul#bottom-header-nav.nav.header__nav');
    list.classList.toggle('active');
  })

  $('#language-switch').each(function() {
    let pageDomain = window.location.hostname;
    if (window.location.pathname.startsWith('/ru/')) {
      $(this).attr('onclick', `javascript:location.href='${window.location.href.replace('/ru/', '/en/')}'`)
      $(this).attr('checked', 'checked');
    } else if (window.location.pathname.startsWith('/en/')) {
      $(this).attr('onclick', `javascript:location.href='${window.location.href.replace('/en/', '/ru/')}'`)
      $(this).removeAttr('checked', 'checked');
    } else {
      switch (pageDomain) {
        case 'deckhouse.io':
          $(this).attr('onclick', `javascript:location.href='${window.location.href.replace('deckhouse.io', 'deckhouse.ru')}'`)
          $(this).removeAttr('checked', 'checked');
          break;
        case 'deckhouse.ru':
          $(this).attr('onclick', `javascript:location.href='${window.location.href.replace('deckhouse.ru', 'deckhouse.io')}'`)
          $(this).attr('checked', 'checked');
          break;
        case 'ru.localhost':
          $(this).attr('onclick', `javascript:location.href='${window.location.href.replace('ru.localhost', 'localhost')}'`)
          $(this).attr('checked', 'checked');
          break;
        case 'localhost':
          $(this).attr('onclick', `javascript:location.href='${window.location.href.replace('localhost', 'ru.localhost')}'`)
          $(this).removeAttr('checked', 'checked');
          break;
        default:
          if (pageDomain.includes('deckhouse.ru.')) {
            $(this).attr('onclick', `javascript:location.href='${ window.location.href.replace('deckhouse.ru.', 'deckhouse.')}'`)
            $(this).attr('checked', 'checked');
          } else if (pageDomain.includes('deckhouse.')) {
            $(this).attr('onclick', `javascript:location.href='${window.location.href.replace('deckhouse.', 'deckhouse.ru.')}'`)
            $(this).removeAttr('checked', 'checked');
          }
      }
    }
  });
})