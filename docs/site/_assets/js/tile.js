document.addEventListener('DOMContentLoaded', function () {
    const blocks = document.querySelectorAll('.button-tile');

    blocks.forEach(block => {
        const header = block.querySelector('.button-tile__title');
        const paragraph = block.querySelector('.button-tile__descr');

        if(header && paragraph) {
            const headerHeight = header.offsetHeight;
            const lineHeight = parseFloat(window.getComputedStyle(paragraph).lineHeight);
            const maxParagraphLines = headerHeight > lineHeight * 1.5 ? 2 : 3;
            const maxHeight = lineHeight * maxParagraphLines;

            paragraph.style.maxHeight = maxHeight + 'px';
            paragraph.style.overflow = 'hidden';
            paragraph.style.textOverflow = 'ellipsis';
            paragraph.style.display = '-webkit-box';
            paragraph.style.webkitLineClamp = maxParagraphLines;
            paragraph.style.webkitBoxOrient = 'vertical';
        }
    });
})

document.addEventListener('DOMContentLoaded', function () {
  const items = document.querySelectorAll('.button-tile');
  const button = document.querySelector('.tile__pagination');
  const itemsPerPage = 12;
  let count = 0;

  function hideAllItems() {
    items.forEach(item => item.style.display = 'none');
  }

  function showItems() {
    const end = Math.min(count + itemsPerPage, items.length);

    for(let i = 0; i < end; i++) {
        items[i].style.display = 'flex';
    }

    count = end;

    if(count >= items.length) {
        button.style.display = 'none';
    }
  }

  if(items.length < itemsPerPage) {
    button.style.display = 'none';
  }

  hideAllItems()
  showItems();

  button.addEventListener('click', () => {
    showItems();
  });
})