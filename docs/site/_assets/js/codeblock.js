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
          let textToCopy = '';

          function codeChild(node) {

            if(node.nodeType === Node.TEXT_NODE) {
              textToCopy += node.textContent;
            } else if(node.nodeType === Node.ELEMENT_NODE) {
              if(node.hasAttribute('data-copy') || node.getAttribute('data-copy') === 'ignore') {
                return;
              }
            }

            for(let i = 0; i < node.childNodes.length; i++){
              codeChild(node.childNodes[i]);
            }
          }

          for(let i = 0; i < code.childNodes.length; i++) {
            codeChild(code.childNodes[i]);
          }

          navigator.clipboard.writeText(textToCopy).then(r => {
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

document.addEventListener("DOMContentLoaded", function() {
  const preElement = document.querySelectorAll('pre');

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
      wrap: 'Wrap',
      unwrap: 'Unwrap',
    },
    ru: {
      wrap: 'Переносить строки',
      unwrap: 'Не переносить строки',
    }
  };
  const texts = textTooltip[lang];

  preElement.forEach(pre => {

    if(!pre.querySelector('code.language-tree')) {
      pre.classList.add('code__transfer');
      const code = pre.querySelector('code');

      const codeText = code.innerHTML;
      const lines = codeText.split('\n');

      let newHTML = '';

      let lineIndex = 1;
      lines.forEach((line, index, arr) => {
        const trimLine = line.trim();

        if(trimLine.length === 0 || /^(\s*<\/span>\s*){1,3}$/.test(trimLine)) {
          return;
        }
        newHTML += `<span data-copy="ignore" class="line-number">${lineIndex}</span>${line}`;

        if(index < arr.length - 1) {
          newHTML += `\n`;
        }
        
        lineIndex++;
      });

      code.innerHTML = newHTML;

      const button = document.createElement('button');
      button.classList.add('wrap__button');

      const wrapIcon = document.createElement('img');
      wrapIcon.src = '/images/wrap-button.svg';
      wrapIcon.classList.add('wrap__icon');
      wrapIcon.style.display = 'none';

      const unwrapIcon = document.createElement('img');
      unwrapIcon.src = '/images/unwrap-button.svg';
      unwrapIcon.classList.add('wrap__icon');

      button.appendChild(wrapIcon);
      button.appendChild(unwrapIcon);

      code.classList.add('wrap');
      pre.appendChild(button);

      let isWrapper = false;

      button.classList.remove('show');

      button.addEventListener('click', function() {
        isWrapper = !isWrapper;
        code.classList.toggle('wrap');

        if(isWrapper) {
          wrapIcon.style.display = 'inline';
          unwrapIcon.style.display = 'none';
        } else {
          wrapIcon.style.display = 'none';
          unwrapIcon.style.display = 'inline';
        }
      });

      tippy(wrapIcon, {
        placement: 'left',
        arrow: false,
        animation: 'scale',
        theme: 'light',
        content: texts.wrap,
        hideOnClick: false,
        delay: [300, 50],
        offset: [0, 10],
        duration: [300],
      });

      tippy(unwrapIcon, {
        placement: 'left',
        arrow: false,
        animation: 'scale',
        theme: 'light',
        content: texts.unwrap,
        hideOnClick: false,
        delay: [300, 50],
        offset: [0, 10],
        duration: [300],
      });

      pre.addEventListener('mouseenter', () => {
        button.classList.add('show');
      });

      pre.addEventListener('mouseleave', () => {
        button.classList.remove('show');
      });
    }
  })
})
