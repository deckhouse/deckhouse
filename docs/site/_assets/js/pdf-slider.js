// const responseFromLicense = {{ site.data.license.response | jsonify }};
// const pageLang = {{ page.lang }};

// console.log(pageLang);

document.addEventListener('DOMContentLoaded', () => {
  const wrappers = document.querySelectorAll('[data-presentation]');
  const pdfjsLib = window['pdfjs-dist/build/pdf'];

  pdfjsLib.GlobalWorkerOptions.workerSrc = '/assets/js/pdf.worker.min.js';

  class PdfDocClass {
    constructor(container) {
      this.pdfDoc = null;
      this.pageNum = 1;
      this.pageRendering = false;
      this.pageNumPending = null;
      this.container = container;
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

      nav.container.appendChild(nav.prev);
      nav.container.appendChild(nav.num);
      nav.container.appendChild(nav.next);

      this.container.appendChild(nav.container);

      return nav;
    }

    renderPage(num) {
      let scales = { 1: 3.2, 2: 4 },
        defaultScale = 3,
        scale = scales[window.devicePixelRatio] || defaultScale;

      let displaySize = 1;

      let v = this;
      this.pageRendering = true;
      // Using promise to fetch the page
      this.pdfDoc.getPage(num).then(function (page) {
        let viewport = page.getViewport({scale: scale});
        v.canvas.height = viewport.height;
        v.canvas.width = viewport.width;

        v.canvas.style.minWidth = `${viewport.width * displaySize / scale}px`;
        v.canvas.style.minHeight = `${viewport.height * displaySize / scale}px`;
        v.canvas.style.width = '100%';
        v.canvas.style.height = '100%';

        // Render PDF page into canvas context
        let renderContext = {
          canvasContext: v.ctx,
          viewport: viewport
        };

        v.nav.container.style.minWidth = `${(viewport.width * displaySize / scale) + 2}px`;
        v.nav.container.style.width = 'calc(100% + 2px)';

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
  }

  let pdfFiles = [];

  wrappers.forEach((i, idx)=> {

    pdfFiles.push(new PdfDocClass(i));

    /**
     * Asynchronously downloads PDF.
     */
    const url = `${pdfFiles[idx].container.dataset.presentation}`
    pdfjsLib.getDocument(url).promise.then(function (pdfDoc_) {
      pdfFiles[idx].pdfDoc = pdfDoc_;
      pdfFiles[idx].renderPage(1);
    });
  })
})
