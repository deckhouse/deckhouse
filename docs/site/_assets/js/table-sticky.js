$(document).ready(function(){
    const tableContainers = document.querySelectorAll('.table-wrapper > div');

    if (tableContainers.length) {
        tableContainers.forEach(tableContainer => {
            const tableWrapper = tableContainer.parentElement

            new ScrollPosition(tableContainer, tableWrapper);
            new StickyHeaderTable(tableContainer, tableWrapper);
        })
    }

    const versionsTable = document.querySelectorAll('.table-wrapper__versions').length > 0;
    if(versionsTable) {
      const rightSidebar = document.querySelector('.layout-sidebar__sidebar_right');
      rightSidebar.style.minWidth = '0';
    }
})

class ScrollPosition {
  constructor(tableContainer, tableWrapper) {
    this.tableContainer = tableContainer;
    this.tableWrapper = tableWrapper;

    this.init();
  }

  init() {
    if (this.tableContainer.offsetWidth < this.tableContainer.scrollWidth) {
      this.tableWrapper.classList.add('more');
      this.tableWrapper.classList.add('more--on-right');
      this.tableContainer.addEventListener('scroll', () => this.handleScroll());
      this.tableHead = this.tableContainer.querySelector('thead') ?? 0;
      this.tableBody = this.tableContainer.querySelector('tbody') ?? 0;
      this.setShadowsheight();
      window.addEventListener('resize', () => this.setShadowsheight());
    }
  }

  setShadowsheight(){
    this.tableWrapper.style.setProperty('--table-height', this.tableHead.offsetHeight + this.tableBody.offsetHeight + 'px');
  }

  handleScroll() {
    if (this.tableContainer.scrollLeft + this.tableContainer.offsetWidth === this.tableContainer.scrollWidth) {
      this.tableWrapper.classList.remove('more--on-right');
    } else {
      this.tableWrapper.classList.add('more--on-right');
    }

    if (this.tableContainer.scrollLeft > 0) {
      this.tableWrapper.classList.add('more--on-left');
    } else {
      this.tableWrapper.classList.remove('more--on-left');
    }
  }
}

class StickyHeaderTable {
  constructor(tableContainer,tableWrapper) {
    if (!tableContainer) return;

    this.tableContainer = tableContainer;
    this.tableWrapper = tableWrapper;
    this.wpadminbar = document.querySelector("#wpadminbar");
    this.navBar = document.querySelector(".notification-bar")
    this.tableElement = tableContainer.querySelector('table');
    this.pageHeader = document.querySelector('header');
    this.tableHeader = tableContainer.querySelector('thead');
    this.tableBody = tableContainer.querySelector('tbody');
    this.tableHeaderCopy = null;
    this.isSyncing = false;
    this.isSticky = false;
    this.tableOffset = 0;
    this.navBarHeight = 0;
    this.adminBarHeight = 0;
    this.TopOffset = 0;

    if(!this.tableHeader || !this.tableElement.classList.contains('fixed-header-table')) return;

    this.init();
  }

  init() {
    this.tableHeaderCopy = this.tableElement.querySelector('thead');
    this.initStickyThead();

    window.addEventListener('resize', (evt) => {
      this.initStickyThead(evt.type);
    });

    document.addEventListener('header_change_event', (evt) => this.initStickyThead(evt.type));

    this.tableElement.addEventListener('enabledStickiness.stickyThead', () => this.enableStickiness());
    this.tableElement.addEventListener('disabledStickiness.stickyThead', () => this.disableStickiness());

    this.tableContainer.addEventListener('scroll', () => this.syncScroll(this.tableContainer, this.tableHeaderCopy));
  }

  initStickyThead(evt = 'default') {
    
    this.calcOffsets(evt);

    stickyThead.apply([this.tableElement], {
      marginTop: this.TopOffset,
      fixedOffset: this.tableOffset,
      cachedHeaderHeight: true,
    });
    this.syncHeadersWidth();
  }

  syncHeadersWidth(){
    if (this.tableHeaderCopy) {
      this.tableHeaderCopy.style.width = `${this.tableWrapper.offsetWidth}px`;
    }
  }

  calcOffsets(evtType) {
    this.TopOffset = this.pageHeader.clientHeight;

    if (evtType == 'default' || evtType == 'resize') {
      this.updateAdminbarHeight();
      this.updateNavBarHeight();
      this.updateTableOffset();
    }

    if (evtType == 'header_change_event' || evtType == 'resize') {
      this.TopOffset -= this.getNavBarOffset();
    }

    this.TopOffset += this.adminBarHeight;
  }

  getNavBarOffset() {
    if (this.pageHeader.classList.contains('header_small')) {
      return this.navBarHeight;
    }
    else{
      return 0;
    }
  }

  updateTableOffset(){
    this.tableOffset = this.tableHeader.clientHeight + (this.tableWrapper.clientHeight - this.tableBody.clientHeight - this.tableHeader.clientHeight);
  }

  updateAdminbarHeight() {
    if(!this.wpadminbar) return;
    this.adminBarHeight = this.wpadminbar.clientHeight;
  }

  updateNavBarHeight() {
    if(!this.navBar) return;
    const navbarCSSHeight = document.querySelector("html").style.getPropertyValue('--nbar-height');
    this.navBarHeight = Number(navbarCSSHeight.replace(/\D/g, ''));
  }

  syncScroll(source, target) {
    if (!this.isSyncing && this.isSticky) {
      this.isSyncing = true;
      target.scrollLeft = source.scrollLeft;
      this.isSyncing = false;
    }
  }

  isOverflown(element) {
    return element.scrollHeight > element.clientHeight || element.scrollWidth > element.clientWidth;
  }

  waitForHeaderCopyReady(element, callback, interval = 10) {
    const checkOverflow = setInterval(() => {
      if (this.tableContainer.innerWidth === element.innerWidth) {
        clearInterval(checkOverflow);
        callback();
      }
    }, interval);
  }

  enableStickiness() {
    this.isSticky = true;
    if (this.tableHeaderCopy) {
      this.tableHeaderCopy.classList.add('sticky');

      if (this.isOverflown(this.tableContainer)) {
        this.tableHeaderCopy.style.overflow = 'hidden';
        this.tableHeaderCopy.style.visibility = 'hidden';
        this.waitForHeaderCopyReady(this.tableHeaderCopy, () => {
          this.tableHeaderCopy.scrollLeft = this.tableContainer.scrollLeft;
          this.tableHeaderCopy.style.visibility = '';
        });
      }
    }
  }

  disableStickiness() {
    this.isSticky = false;
    if (this.tableHeaderCopy) {
      this.tableHeaderCopy.classList.remove('sticky');
    }
  }
}
