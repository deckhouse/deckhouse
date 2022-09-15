document.addEventListener('DOMContentLoaded', () => {
// If absolute URL from the remote server is provided, configure the CORS
// header on that server.
  const wrappers = document.querySelectorAll('.canvas__wrap');

  // var url = '/images/Deckhouse_Getting_started_Cloud_RU.pdf';

// Loaded via <script> tag, create shortcut to access PDF.js exports.
  var pdfjsLib = window['pdfjs-dist/build/pdf'];

// The workerSrc property shall be specified.
  pdfjsLib.GlobalWorkerOptions.workerSrc = '//mozilla.github.io/pdf.js/build/pdf.worker.js';

  class PdfDocClass {
    constructor(pdfDoc, pageNum, pageRendering, pageNumPending, scale, canvas) {
      this.pdfDoc = pdfDoc;
      this.pageNum = pageNum;
      this.pageRendering = pageRendering;
      this.pageNumPending = pageNumPending;
      this.scale = scale;
      this.canvas = canvas;
      this.ctx = canvas.getContext('2d');
    }

    // renderPage(num) {
    //   this.pageRendering = true;
    //   // Using promise to fetch the page
    //   this.pdfDoc.getPage(num).then(function (page) {
    //     var viewport = page.getViewport({scale: this.scale});
    //     this.canvas.height = viewport.height;
    //     this.canvas.width = viewport.width;
    //
    //     // Render PDF page into canvas context
    //     var renderContext = {
    //       canvasContext: this.ctx,
    //       viewport: viewport
    //     };
    //     var renderTask = page.render(renderContext);
    //
    //     // Wait for rendering to finish
    //     renderTask.promise.then(function () {
    //       this.pageRendering = false;
    //       if (this.pageNumPending !== null) {
    //         // New page rendering is pending
    //         this.renderPage(pageNumPending);
    //         pageNumPending = null;
    //       }
    //     });
    //   });
    //   // document.getElementById('page_num').textContent = num;
    // }
    //
    // /**
    //  * If another page rendering in progress, waits until the rendering is
    //  * finised. Otherwise, executes rendering immediately.
    //  */
    // queueRenderPage(num) {
    //   if (this.pageRendering) {
    //     this.pageNumPending = num;
    //   } else {
    //     this.renderPage(num);
    //   }
    // }
    //
    //
    // onPrevPage() {
    //   if (this.pageNum <= 1) {
    //     return;
    //   }
    //   this.pageNum--;
    //   this.queueRenderPage(this.pageNum);
    // }
    //
    // // document.getElementById('prev').addEventListener('click', onPrevPage);
    //
    // onNextPage() {
    //   if (this.pageNum >= this.pdfDoc.numPages) {
    //     return;
    //   }
    //   this.pageNum++;
    //   this.queueRenderPage(this.pageNum);
    // }

    // document.getElementById('next').addEventListener('click', onNextPage);
  }

  let pdfFiles = [];

  wrappers.forEach((i, idx)=> {
    const canvas = i.querySelector('.the-canvas');
    const navbar = i.querySelector('.slider__nav');

    pdfFiles.push(new PdfDocClass(null, 1, false, null, 1.1, canvas));


    // var pdfDoc = null;
      // pageNum = 1,
      // pageRendering = false,
      // pageNumPending = null,
      // scale = 0.8,
      // canvas = document.getElementById('the-canvas'),
      // ctx = canvas.getContext('2d');

    /**
     * Get page info from document, resize canvas accordingly, and render page.
     * @param num Page number.
     */
    function renderPage(num) {
      pdfFiles[idx].pageRendering = true;
      // Using promise to fetch the page
      pdfFiles[idx].pdfDoc.getPage(num).then(function (page) {
        var viewport = page.getViewport({scale: pdfFiles[idx].scale});
        pdfFiles[idx].canvas.height = viewport.height;
        pdfFiles[idx].canvas.width = viewport.width;

        // Render PDF page into canvas context
        var renderContext = {
          canvasContext: pdfFiles[idx].ctx,
          viewport: viewport
        };
        navbar.style.width = `${viewport.width}px`;

        var renderTask = page.render(renderContext);

        // Wait for rendering to finish
        renderTask.promise.then(function () {
          pdfFiles[idx].pageRendering = false;
          if (pdfFiles[idx].pageNumPending !== null) {
            // New page rendering is pending
            renderPage(pdfFiles[idx].pageNumPending);
            pdfFiles[idx].pageNumPending = null;
          }
        });
      });

      // Update page counters
      i.querySelector('.slider__nav--page-num').textContent = num;
    }

    /**
     * If another page rendering in progress, waits until the rendering is
     * finised. Otherwise, executes rendering immediately.
     */
    function queueRenderPage(num) {
      if (pdfFiles[idx].pageRendering) {
        pdfFiles[idx].pageNumPending = num;
      } else {
        renderPage(num);
      }
    }

    /**
     * Displays previous page.
     */
    function onPrevPage() {
      if (pdfFiles[idx].pageNum <= 1) {
        return;
      }
      pdfFiles[idx].pageNum--;
      queueRenderPage(pdfFiles[idx].pageNum);
    }

    i.querySelector('.slider__nav--button-prev').addEventListener('click', onPrevPage);

    /**
     * Displays next page.
     */
    function onNextPage() {
      if (pdfFiles[idx].pageNum >= pdfFiles[idx].pdfDoc.numPages) {
        return;
      }
      pdfFiles[idx].pageNum++;
      queueRenderPage(pdfFiles[idx].pageNum);
    }

    i.querySelector('.slider__nav--button-next').addEventListener('click', onNextPage);

    /**
     * Asynchronously downloads PDF.
     */
    const url = `/images/${canvas.dataset.presentation}.pdf`
    pdfjsLib.getDocument(url).promise.then(function (pdfDoc_) {
      pdfFiles[idx].pdfDoc = pdfDoc_;
      // document.getElementById('page_count').textContent = pdfDoc.numPages;

      // Initial/first page rendering
      renderPage(1);
    });
  })

  // pdfjsLib.getDocument(url).promise.then(function (pdfDoc_) {
  //   test.pdfDoc = pdfDoc_;
  //   console.log(test.pdfDoc);
  //   console.log(test);
  //   // document.getElementById('page_count').textContent = pdfDoc.numPages;
  //
  //   // Initial/first page rendering
  //   renderPage(1);
  // });
})
