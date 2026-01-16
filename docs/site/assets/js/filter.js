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
      preview: 'Предварительная версия. Функциональность модуля может измениться, но основные возможности сохранятся. Совместимость с будущими версиями обеспечивается, но может потребоваться миграция.',
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
  const filterSearch = document.getElementById('search-filter');
  let fullResetHandler = null;

  function hideAllItems() {
    articles.forEach(article => article.style.display = 'none');
  }

  function initializeArticleFilter(articlesToFilter) {
    hideAllItems();

    articlesToFilter.forEach(article => {
      article.style.display = 'flex';
    })
  }

  function createSelectedFilterElement(text, onRemove) {
    const selectedElement = document.createElement('div');
    selectedElement.classList.add('selected__filter');
    selectedElement.textContent = text;

    const removeButton = document.createElement('a');
    removeButton.classList.add('remove__filter');
    removeButton.addEventListener('click', onRemove);

    selectedElement.appendChild(removeButton);
    return selectedElement;
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
    if (!filterCheckboxesTags) return;
    
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
    if (!filterCheckboxesTags) return;
    
    const tags = getTags();
    tags.forEach(tag => {
      createCheckboxes(tag);
    });
  }

  if (filterCheckboxesTags) {
    createFilters();
  }
  
  function resetAllFilters() {
    if (filterSearch) {
      filterSearch.value = '';
    }
    
    document.querySelectorAll('.filter input[type="checkbox"]:checked').forEach(checkbox => {
      checkbox.checked = false;
    });

    document.querySelectorAll('.filter__container--title').forEach(title => {
      title.classList.remove('filter-selected');
    });
    
    filterArticles();
  }

  function filterArticles() {
    if (selectedFiltersList) {
      selectedFiltersList.innerHTML = '';
    }
    
    const checkedCheckboxes = document.querySelectorAll('.filter input[type="checkbox"]:checked');
    const query = filterSearch ? filterSearch.value.trim() : '';

    if (selectedFiltersList) {
      checkedCheckboxes.forEach(checkbox => {
        const filterContainer = checkbox.closest('.filter__container');
        const filterName = filterContainer?.querySelector('.filter__container--title.closing-title')?.textContent || '';
        const checkboxValue = checkbox.value;
        const checkboxText = `${filterName}: ${checkboxValue}`;

        const selectedElement = createSelectedFilterElement(checkboxText, () => {
          checkbox.checked = false;
          filterArticles();
        });

        selectedFiltersList.appendChild(selectedElement);
      });

      if(query.length > 0) {
        const searchText = `${texts.search}: ${query}`;
        const selectedElement = createSelectedFilterElement(searchText, () => {
          if (filterSearch) {
            filterSearch.value = '';
          }
          filterArticles();
        });

        selectedFiltersList.appendChild(selectedElement);
      }

      if(checkedCheckboxes.length > 0 || query.length > 0) {
        if (fullResetHandler) {
          fullReset.removeEventListener('click', fullResetHandler);
        }
        fullResetHandler = resetAllFilters;
        fullReset.addEventListener('click', fullResetHandler);
        selectedFiltersList.appendChild(fullReset);
      }
    }

    const checkboxesEditorialChecked = document.querySelectorAll('.filter__container--editorial input[type="checkbox"]:checked');
    const selectedEditorial = Array.from(checkboxesEditorialChecked).map(checkbox => checkbox.value);

    const checkboxesStatusesChecked = document.querySelectorAll('.filter__container--statuses input[type="checkbox"]:checked');
    const selectedStatuses = Array.from(checkboxesStatusesChecked).map(checkbox => checkbox.value);

    const checkboxesTagsChecked = document.querySelectorAll('.filter__container--tags input[type="checkbox"]:checked');
    const selectedTags = Array.from(checkboxesTagsChecked).map(checkbox => checkbox.value);

    const filtered = Array.from(articles).filter(article => {
      const titleElement = article.querySelector('h2');
      if (!titleElement) return false;
      
      const title = titleElement.textContent.toLowerCase();
      if(query && !title.includes(query.toLowerCase())) {
        return false;
      }

      if(selectedEditorial.length > 0) {
        const hasEditorial = selectedEditorial.every(editorial => {
          if(editorial === 'commercialEditions') {
            return article.dataset.commercialEditions === 'true';
          }
          return false;
        });
        if(!hasEditorial) {
          return false;
        }
      }

      if(selectedStatuses.length > 0) {
        const hasAllStatuses = selectedStatuses.every(status => {
          return article.querySelector(`.button-tile__stage-${status}`) !== null;
        });
        if(!hasAllStatuses) {
          return false;
        }
      }

      if(selectedTags.length > 0) {
        const articleTags = Array.from(
          article.querySelectorAll('.button-tile__tags .sidebar__badge--container .sidebar__badge_v2')
        ).map(tag => tag.textContent);
        
        if(!selectedTags.every(tag => articleTags.includes(tag))) {
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

  if (filterSearch) {
    filterSearch.addEventListener('input', filterArticles);
  }

  if(resetButton) {
    resetButton.addEventListener('click', resetAllFilters);
  }

  document.querySelectorAll('.filter__container').forEach(container => {
    const checkboxes = container.querySelectorAll('input[type="checkbox"]');
    const title = container.querySelector('.filter__container--title');
    checkboxes.forEach(checkbox => {
      checkbox.addEventListener('change', function() {
        const check = Array.from(checkboxes).some(checkbox => checkbox.checked);
        title.classList.toggle('filter-selected', check);
      })
    })
  })

  const checkboxes = document.querySelectorAll('.filter__container input[type="checkbox"]');
  checkboxes.forEach(checkbox => {
    checkbox.addEventListener('change', filterArticles);
  });

  initializeArticleFilter(Array.from(articles));

  function createTooltipContent(titleText, descriptionText) {
    const container = document.createElement('div');
    container.classList.add('statuses-tooltip');
    
    const title = document.createElement('p');
    title.classList.add('statuses-tooltip__title');
    title.textContent = titleText;
    
    const description = document.createElement('p');
    description.classList.add('statuses-tooltip__descr');
    description.textContent = descriptionText;
    
    container.appendChild(title);
    container.appendChild(description);
    return container;
  }
  
  function initTooltip(selector, titleText, descriptionText) {
    const elements = document.querySelectorAll(selector);
    if (elements.length === 0) return;

    tippy(elements, {
      allowHTML: true,
      content: () => createTooltipContent(titleText, descriptionText),
      arrow: true,
      appendTo: 'parent',
      hideOnClick: false,
      delay: [300, 50],
      offset: [0, 10],
      duration: [300],
    });
  }

  initTooltip('.filter__container label[for="experimental"] > img, .button-tile__stage-experimental > img', 'Experimental', texts.experimental);
  initTooltip('.filter__container label[for="preview"] > img, .button-tile__stage-preview > img', 'Preview', texts.preview);
  initTooltip('.filter__container label[for="generalAvailability"] > img, .button-tile__stage-generalAvailability > img', 'General Availability (GA)', texts.generalAvailability);
  initTooltip('.filter__container label[for="deprecated"] > img, .button-tile__stage-deprecated > img', 'Deprecated', texts.deprecated);
})