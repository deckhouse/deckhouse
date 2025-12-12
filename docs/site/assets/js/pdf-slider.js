// const responseFromLicense = {{ site.data.license.response | jsonify }};
// const pageLang = {{ page.lang }};

// console.log(pageLang);

import {getDocument, GlobalWorkerOptions} from './pdf.min.mjs';

GlobalWorkerOptions.workerSrc = '/assets/js/pdf.worker.min.mjs';

document.addEventListener('DOMContentLoaded', () => {
  const wrappers = document.querySelectorAll('[data-presentation]');

  class PdfDocClass {
    constructor(container) {
      this.pdfDoc = null;
      this.pageNum = 1;
      this.pageRendering = false;
      this.pageNumPending = null;
      this.container = container;
      this.pdfUrl = container.dataset.presentation;
      this.canvas = this.initializeCanvas();
      this.ctx = this.canvas.getContext('2d');
      this.nav = this.initializeNav();
    }

    initializeCanvas() {
      let canvas;

      canvas = document.createElement('canvas');
      canvas.classList.add('pdf-slider__canvas');

      this.container.appendChild(canvas);

      return canvas;
    }

    initializeNav() {
      let v = this;
      let nav = {};

      nav.container = document.createElement('div')
      nav.container.classList.add('pdf-slider__nav');

      nav.prev = document.createElement('button')
      nav.prev.classList.add('pdf-slider__nav--button', 'pdf-slider__nav--button-prev');
      nav.prev.innerHTML = `<svg><use xlink:href="/images/sprite.svg#chevron-left-icon"></use></svg>`;
      nav.prev.addEventListener('click', function() { v.prevPage() });

      nav.next = document.createElement('button')
      nav.next.classList.add('pdf-slider__nav--button', 'pdf-slider__nav--button-next');
      nav.next.innerHTML = `<svg><use xlink:href="/images/sprite.svg#chevron-left-icon"></use></svg>`;
      nav.next.addEventListener('click', function() { v.nextPage() } );

      nav.num = document.createElement('span')
      nav.num.classList.add('pdf-slider__nav--page-num');

      nav.download = document.createElement('button')
      nav.download.classList.add('pdf-slider__nav--button', 'pdf-slider__nav--button-download');
      const pageLang = document.documentElement.lang || document.querySelector('html')?.lang || '';
      const downloadTitle = pageLang === 'ru' ? 'Скачать PDF' : 'Download PDF';
      nav.download.setAttribute('title', downloadTitle);
      nav.download.innerHTML = `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg"><path d="M12 15V3M12 15L8 11M12 15L16 11M3 17V19C3 20.1046 3.89543 21 5 21H19C20.1046 21 21 20.1046 21 19V17" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>`;
      nav.download.addEventListener('click', function() { v.downloadPdf() });

      nav.container.appendChild(nav.prev);
      nav.container.appendChild(nav.num);
      nav.container.appendChild(nav.next);
      nav.container.appendChild(nav.download);

      this.container.appendChild(nav.container);

      return nav;
    }

    renderPage(num) {
      let v = this;
      const defaultWidth = 900;
      this.pageRendering = true;
      // Using promise to fetch the page
      this.pdfDoc.getPage(num).then(function (page) {
        // Get the available width from the container or its parent .docs element
        let containerElement = v.container.closest('.docs') || v.container.parentElement;
        let availableWidth = defaultWidth; // default fallback

        if (containerElement) {
          // Use clientWidth which accounts for padding but not scrollbar
          availableWidth = containerElement.clientWidth;

          // If still 0, try getting computed style to account for any CSS constraints
          if (availableWidth === 0) {
            const computedStyle = window.getComputedStyle(containerElement);
            const paddingLeft = parseFloat(computedStyle.paddingLeft) || 0;
            const paddingRight = parseFloat(computedStyle.paddingRight) || 0;
            availableWidth = containerElement.offsetWidth - paddingLeft - paddingRight;
          }
        }

        // Final fallback if width is still invalid
        if (!availableWidth || availableWidth <= 0) {
          availableWidth = defaultWidth;
        }

        // Calculate scale based on available width
        var _viewport = page.getViewport({ scale: 1 });
        var scale = availableWidth / _viewport.width;
        var viewport = page.getViewport({ scale: scale });

        // Set canvas internal dimensions to match viewport
        v.canvas.height = viewport.height;
        v.canvas.width = viewport.width;

        // Set canvas display size to match viewport (not multiplied by scale again!)
        v.canvas.style.width = `${viewport.width}px`;
        v.canvas.style.height = `${viewport.height}px`;

        // Render PDF page into canvas context
        let renderContext = {
          canvasContext: v.ctx,
          viewport: viewport
        };

        // Set navigation container width to match canvas width
        v.nav.container.style.width = `${viewport.width + 2}px`;

        let renderTask = page.render(renderContext);

        // Wait for rendering to finish
        renderTask.promise.then(function () {
          v.pageRendering = false;
          if (v.pageNumPending !== null) {
            v.renderPage(v.pageNumPending);
            v.pageNumPending = null;
          }
        });
      });

      this.nav.num.textContent = num;
      this.checkPageLock();
    }

    prevPage() {
      if (this.pageNum <= 1) {
        return;
      }
      this.pageNum--;
      this.queueRenderPage(this.pageNum);
    }

    nextPage() {
      if (this.pageNum >= this.pdfDoc.numPages) {
        return;
      }
      this.pageNum++;
      this.queueRenderPage(this.pageNum);
    }

    checkPageLock() {
      if (this.pageNum == 1) {
        this.nav.prev.classList.add('disabled');
      } else {
        this.nav.prev.classList.remove('disabled');
      }
      if (this.pageNum >= this.pdfDoc.numPages) {
        this.nav.next.classList.add('disabled');
      } else {
        this.nav.next.classList.remove('disabled');
      }
    }

    queueRenderPage(num) {
      if (this.pageRendering) {
        this.pageNumPending = num;
      } else {
        this.renderPage(num);
      }
    }

    downloadPdf() {
      if (!this.pdfUrl) {
        return;
      }
      
      // Create a temporary anchor element to trigger download
      const link = document.createElement('a');
      link.href = this.pdfUrl;
      link.download = this.pdfUrl.split('/').pop();
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
    }
  }

  let pdfFiles = [];

  wrappers.forEach((i, idx)=> {

    pdfFiles.push(new PdfDocClass(i));

    const url = `${pdfFiles[idx].container.dataset.presentation}`
    getDocument(url).promise.then(function (pdfDoc_) {
      pdfFiles[idx].pdfDoc = pdfDoc_;
      pdfFiles[idx].renderPage(1);
    });
  })
})
