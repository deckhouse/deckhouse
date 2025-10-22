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
