document.addEventListener('DOMContentLoaded', () => {
  const articles = document.querySelectorAll('.button-tile');
  const selectedFiltersList = document.querySelector('.selected__filters--list');
  const filterCheckboxesTags = document.querySelector('.filter__checkboxes--tags');
  const resetButton = document.querySelector('.reset-check');
  const fullReset = document.createElement('div');
  fullReset.classList.add('full-reset');
  let lang = document.documentElement.lang;

  if (lang.length === 0) {
    if (window.location.href.includes("deckhouse.ru") || window.location.href.includes("ru.localhost")) {
      lang = "ru"
    } else {
      lang = "en"
    }
  }

  const description = {
    ru: {
      search: 'Поиск',
      experimental: 'Экспериментальная версия. Функциональность модуля может сильно измениться. Совместимость с будущими версиями не гарантируется.',
      preview: 'Предварительная версия. вункциональность модуля может изменитсья, но основные возможности сохранятся. Совместимость с будущими версиями обеспечивается, но может потребоваться миграция.',
      generalAvailability: 'Общедоступная версия. Модуль готов к использованию в production-средах.',
      deprecated: 'Модуль устарел. Дальнейшее развитие и поддержка модуля прекращены.'
    },
    en: {
      search: "Search",
      experimental: "Experimental version. The module's functionality may change significantly. Compatibility with future versions is not guaranteed.",
      preview: "Preliminary version. The module's functionality may change, but the core features remain. Compatibility with future versions is ensured, but migration may be required.",
      generalAvailability: 'General availability. The module is ready for use in production environments.',
      deprecated: 'The module is deprecated. Further development and support for this module have been discontinued.'
    }
  };

  const texts = description[lang];

  function hideAllItems() {
    articles.forEach(article => article.style.display = 'none');
  }

  function initializeArticleFilter(articlesToFilter) {
    hideAllItems();

    articlesToFilter.forEach(article => {
      article.style.display = 'flex';
    })
  }

  function getTags() {
    const tags = new Set();
    articles.forEach(article => {
      article.querySelectorAll('.button-tile__tags .sidebar__badge--container .sidebar__badge_v2').forEach(tag => {
        tags.add(tag.textContent);
      });
    });
    return Array.from(tags).sort();
  }

  function createCheckboxes(tag) {
    const input = document.createElement('input');
    input.type = 'checkbox';
    input.id = tag;
    input.value = tag;

    const label = document.createElement('label');
    label.htmlFor = tag;
    label.textContent = tag;
    label.style.textTransform = 'capitalize';

    filterCheckboxesTags.appendChild(input);
    filterCheckboxesTags.appendChild(label);
  }

  function createFilters() {
    const tags = getTags();
    tags.forEach(tag => {
      createCheckboxes(tag);
    });
  }

  createFilters();
  
  function filterArticles() {
    selectedFiltersList.innerHTML = '';
    const checkedCheckboxes = document.querySelectorAll('.filter input[type="checkbox"]:checked');
    let search = document.getElementById('search-filter');
    const query = search.value;

    checkedCheckboxes.forEach(checkbox => {
      const filterName = checkbox.closest('.filter__container').querySelector('.filter__container--title.closing-title').textContent;
      const checkboxValue = checkbox.value;
      const checkboxText = `${filterName}: ${checkboxValue}`;

      const selectedElement = document.createElement('div');
      selectedElement.classList.add('selected__filter');
      selectedElement.textContent = checkboxText;

      const removeButton = document.createElement('a');
      removeButton.classList.add('remove__filter');

      removeButton.addEventListener('click', function() {
        checkbox.checked = false;
        filterArticles();
      });

      selectedElement.appendChild(removeButton);
      selectedFiltersList.appendChild(selectedElement);
    });

    if(query.length > 0) {
      const searchText = `${texts.search}: ${query}`;

      const selectedElement = document.createElement('div');
      selectedElement.classList.add('selected__filter');
      selectedElement.textContent = searchText;

      const removeButton = document.createElement('a');
      removeButton.classList.add('remove__filter');

      removeButton.addEventListener('click', function() {
        search.value = '';
        filterArticles();
      });

      selectedElement.appendChild(removeButton);
      selectedFiltersList.appendChild(selectedElement);
    }

    if(checkedCheckboxes.length > 0 || query.length > 0) {
      selectedFiltersList.appendChild(fullReset);
      fullReset.addEventListener('click', function() {
        search.value = '';
        checkedCheckboxes.forEach(checkbox => {
          checkbox.checked = false;
        })
        filterArticles();
      })
    }

    resetButton.addEventListener('click', () => {
      checkedCheckboxes.forEach(checkbox => {
        checkbox.checked = false;
      })
      filterArticles();
    })

    const checkboxesEditorialChecked = document.querySelectorAll('.filter__container--editorial input[type="checkbox"]:checked');
    const selectedEditorial = Array.from(checkboxesEditorialChecked).map(checkbox => checkbox.value);

    const checkboxesStatusesChecked = document.querySelectorAll('.filter__container--statuses input[type="checkbox"]:checked');
    const selectedStatuses = Array.from(checkboxesStatusesChecked).map(checkbox => checkbox.value);

    const checkboxesTagsChecked = document.querySelectorAll('.filter__container--tags input[type="checkbox"]:checked');
    const selectedTags = Array.from(checkboxesTagsChecked).map(checkbox => checkbox.value);

    const filtered = Array.from(articles).filter(article => {
      const title = article.querySelector('h2').textContent.toLowerCase();
      if(query.toLowerCase() && !title.includes(query.toLowerCase())) {
        return false;
      }

      if(selectedEditorial.length > 0) {
        for(const editorial of selectedEditorial) {
          if(editorial === 'commercialEditions' && article.dataset.commercialEditions !== true) {
              return false;
          }
        }
      }

      if(selectedStatuses.length > 0) {
        let articleFound = false;
        for(const status of selectedStatuses) {
          if(article.querySelector('.icon')) {
            articleFound = true;
            break;
          }
        }
        if(!articleFound) {
          return false;
        }
      }

      if(selectedTags.length > 0) {
        const tagElement = Array.from(article.querySelectorAll('.button-tile__tags .sidebar__badge--container .sidebar__badge_v2')).map(tag => tag.textContent);
        if(!selectedTags.every(tag => tagElement.includes(tag))) {
          return false;
        }
      }

      return true;
    });

    initializeArticleFilter(filtered);
  }

  document.querySelectorAll('.closing-title').forEach(title => {
    title.addEventListener('click', () => {
      title.nextElementSibling.classList.toggle('hidden');
      title.classList.toggle('rotated');
    })
  })

  const filterSearch = document.getElementById('search-filter');
  filterSearch.addEventListener('input', filterArticles);

  const checkboxes = document.querySelectorAll('.filter__container input[type="checkbox"]');
  checkboxes.forEach(checkbox => {
    checkbox.addEventListener('change', filterArticles);
  });

  initializeArticleFilter(Array.from(articles));

  const experimentalIcon = document.querySelector('.filter__container label[for="experimental"] > img');
  const previewIcon = document.querySelector('.filter__container label[for="preview"] > img');
  const generalAvailabilityIcon = document.querySelector('.filter__container label[for="generalAvailability"] > img');
  const deprecatedIcon = document.querySelector('.filter__container label[for="deprecated"] > img');

  tippy(experimentalIcon, {
    allowHTML: true,
    content: () => {
      const container = document.createElement('div');
      container.classList.add('statuses-tooltip');
      const title = document.createElement('p');
      title.classList.add('statuses-tooltip__title');
      title.textContent = 'Experimental';
      const description = document.createElement('p');
      description.classList.add('statuses-tooltip__descr');
      description.textContent = texts.experimental;
      container.appendChild(title);
      container.appendChild(description);

      return container;
    },
    arrow: true,
    appendTo: 'parent',
    hideOnClick: false,
    delay: [300, 50],
    offset: [0, 10],
    duration: [300],
  });

  tippy(previewIcon, {
    allowHTML: true,
    content: () => {
      const container = document.createElement('div');
      container.classList.add('statuses-tooltip');
      const title = document.createElement('p');
      title.classList.add('statuses-tooltip__title');
      title.textContent = 'Preview';
      const description = document.createElement('p');
      description.classList.add('statuses-tooltip__descr');
      description.textContent = texts.preview;
      container.appendChild(title);
      container.appendChild(description);

      return container;
    },
    arrow: true,
    appendTo: 'parent',
    hideOnClick: false,
    delay: [300, 50],
    offset: [0, 10],
    duration: [300],
  });

  tippy(generalAvailabilityIcon, {
    allowHTML: true,
    content: () => {
      const container = document.createElement('div');
      container.classList.add('statuses-tooltip');
      const title = document.createElement('p');
      title.classList.add('statuses-tooltip__title');
      title.textContent = 'General Availability (GA)';
      const description = document.createElement('p');
      description.classList.add('statuses-tooltip__descr');
      description.textContent = texts.generalAvailability;
      container.appendChild(title);
      container.appendChild(description);

      return container;
    },
    arrow: true,
    appendTo: 'parent',
    hideOnClick: false,
    delay: [300, 50],
    offset: [0, 10],
    duration: [300],
  });

  tippy(deprecatedIcon, {
    allowHTML: true,
    content: () => {
      const container = document.createElement('div');
      container.classList.add('statuses-tooltip');
      const title = document.createElement('p');
      title.classList.add('statuses-tooltip__title');
      title.textContent = 'Deprecated';
      const description = document.createElement('p');
      description.classList.add('statuses-tooltip__descr');
      description.textContent = texts.deprecated;
      container.appendChild(title);
      container.appendChild(description);

      return container;
    },
    arrow: true,
    appendTo: 'parent',
    hideOnClick: false,
    delay: [300, 50],
    offset: [0, 10],
    duration: [300],
  });
})