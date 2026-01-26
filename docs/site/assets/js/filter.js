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

  function updateContainerTitleState(container) {
    if (!container) return null;
    const title = container.querySelector('.filter__container--title');
    if (!title) return;

    const checkboxes = container.querySelectorAll('input[type="checkbox"]');
    const checkedCount = Array.from(checkboxes).filter(checkbox => checkbox.checked).length;

    title.classList.toggle('filter-selected', checkedCount > 0);
    if (checkedCount > 0) {
      title.dataset.selectedCount = String(checkedCount);
    } else {
      delete title.dataset.selectedCount;
    }
  }

  function markEmptyCheckboxes() {
    const availableTags = new Set();
    const availableStatuses = new Set();
    const availableEditorial = new Set();

    Array.from(articles).forEach(article => {
      article.querySelectorAll('.button-tile__tags .sidebar__badge--container .sidebar__badge_v2').forEach(tag => {
        availableTags.add(tag.textContent);
      });

      article.querySelectorAll('[class*="button-tile__stage-"]').forEach(el => {
        el.classList.forEach(cls => {
          if (cls.startsWith('button-tile__stage-')) {
            availableStatuses.add(cls.replace('button-tile__stage-', ''));
          }
        });
      });

      const editorial = (article.dataset.editorial || '').trim().toLowerCase();
      if (editorial) {
        availableEditorial.add(editorial);
      }
    });

    document.querySelectorAll('.filter__container input[type="checkbox"]').forEach(checkbox => {

      const label = checkbox.nextElementSibling ? checkbox.nextElementSibling : document.querySelector(`label[for="${checkbox.id}"]`);

      const container = checkbox.closest('.filter__container');
      let isAvailable = true;

      if (container?.classList.contains('filter__container--tags')) {
        isAvailable = availableTags.has(checkbox.value);
      } else if (container?.classList.contains('filter__container--editorial')) {
        isAvailable = availableEditorial.has((checkbox.value || '').trim().toLowerCase());
      } else if (container?.classList.contains('filter__container--statuses')) {
        isAvailable = availableStatuses.has(checkbox.value);
      }

      if (!isAvailable) {
        checkbox.disabled = true;
        checkbox.classList.add('checkbox-disabled');
        if (label) label.classList.add('checkbox-disabled');
      }
    });
  }

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
      delete title.dataset.selectedCount;
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
      const groupedFilters = new Map();

      Array.from(checkedCheckboxes).forEach(checkbox => {
        const filterContainer = checkbox.closest('.filter__container');
        const filterName = filterContainer?.querySelector('.filter__container--title').textContent?.trim() || '';
        if (!filterName) return;

        const entry = groupedFilters.get(filterName) || { checkboxes: [], values: new Set() };
        entry.checkboxes.push(checkbox);
        entry.values.add(checkbox.value);
        groupedFilters.set(filterName, entry);
      });

      groupedFilters.forEach((entry, filterName) => {
        const valuesText = Array.from(entry.values).join(', ');
        const checkboxText = `${filterName}: ${valuesText}`;

        const selectedElement = createSelectedFilterElement(checkboxText, () => {
          entry.checkboxes.forEach(checkbox => {
            checkbox.checked = false;
          });
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
        const articleEditorial = (article.dataset.editorial || '').trim().toLowerCase();
        const matchesEditorial = selectedEditorial.some(editorial => {
          return (editorial || '').trim().toLowerCase() === articleEditorial;
        });
        if(!matchesEditorial) {
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
    document.querySelectorAll('.filter__container').forEach(container => {
      updateContainerTitleState(container);
    });
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
        const checkedCount = Array.from(checkboxes).filter(checkbox => checkbox.checked).length;
        title.classList.toggle('filter-selected', checkedCount > 0);
        if (checkedCount > 0) {
          title.dataset.selectedCount = String(checkedCount);
        } else {
          delete title.dataset.selectedCount;
        }
      })
    })

  })

  const checkboxes = document.querySelectorAll('.filter__container input[type="checkbox"]');
  checkboxes.forEach(checkbox => {
    checkbox.addEventListener('change', filterArticles);
  });

  initializeArticleFilter(Array.from(articles));
  document.querySelectorAll('.filter__container').forEach(container => {
    updateContainerTitleState(container);
  });
  markEmptyCheckboxes();

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

    elements.forEach(element => {
      tippy(element, {
        allowHTML: true,
        content: () => createTooltipContent(titleText, descriptionText),
        arrow: true,
        appendTo: 'parent',
        hideOnClick: false,
        delay: [300, 50],
        offset: [0, 10],
        duration: [300],
      });
    })
  }

  initTooltip('.filter__container label[for="experimental"] > img, .button-tile__stage-experimental > img', 'Experimental', texts.experimental);
  initTooltip('.filter__container label[for="preview"] > img, .button-tile__stage-preview > img', 'Preview', texts.preview);
  initTooltip('.filter__container label[for="generalAvailability"] > img, .button-tile__stage-generalAvailability > img', 'General Availability (GA)', texts.generalAvailability);
  initTooltip('.filter__container label[for="deprecated"] > img, .button-tile__stage-deprecated > img', 'Deprecated', texts.deprecated);
})