$(window).on('load', function() {

  function getLastPathname() {
    const pathname = window.location.pathname;
    const lastSlash = pathname.lastIndexOf('/');

    if(lastSlash === pathname.length - 1) {
      const penultimateSlash = pathname.substring(0, lastSlash).lastIndexOf('/');
      return pathname.substring(penultimateSlash + 1, lastSlash) + '/';
    } else {
      const partsPathname = pathname.split('/');
      return partsPathname.slice(-2).join('/');
    }
  }

  let lastPathname = getLastPathname();

  if(lastPathname.lastIndexOf('v1') !== 1) {
    lastPathname = lastPathname.replace(/v1/g, '.');
  }
  const activeLink = document.querySelector(`ul.sidebar li.sidebar__item.active a[href$="${lastPathname}"]`);
  
  const hash = window.location.hash;
  const activeLinkToc = document.querySelector(`li.toc-sidebar__item a[href="${hash}"]`);

  const sidebars = document.querySelectorAll('.sidebar__wrapper-inner');
  const sidebarLeft = document.querySelector('.sidebar');
  const sidebarToc = document.querySelector('.toc-sidebar');
  
  function getTopActiveLink(element, sidebar) {
    let top = 0;
    
    while (element && element !== sidebar) {
      top += element.offsetTop;
      element = element.offsetParent;
    }
    return top;
  }

  function getTopActiveLinkSidebar(element) {
    const elementTop = element.offsetTop;

    let parentTop = element.offsetParent;
    return elementTop + parentTop.offsetTop;
  }

  const activeLinkTop = getTopActiveLinkSidebar(activeLink);

  if(activeLinkTop < sidebars[0].scrollTop || (activeLinkTop + activeLink.offsetHeight) > (sidebars[0].scrollTop + sidebars[0].scrollHeight)) {
    sidebars[0].scrollIntoView({
      block: 'nearest',
      behavior: 'smooth'
    })
  }

  sidebars[0].scrollTo({
    top: activeLinkTop,
    behavior: 'smooth'
  })

  const tocActiveLinkTop = getTopActiveLink(activeLinkToc, sidebarToc);

  sidebars[1].scrollTo({
    top: tocActiveLinkTop,
    behavior: 'smooth'
  })
});