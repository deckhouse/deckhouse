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
      values: 'знач.',
      allSelected: 'Выбраны все',
      experimental: 'Экспериментальная версия. Функциональность модуля может сильно измениться. Совместимость с будущими версиями не гарантируется.',
      preview: 'Предварительная версия. Функциональность модуля может измениться, но основные возможности сохранятся. Совместимость с будущими версиями обеспечивается, но может потребоваться миграция.',
      generalAvailability: 'Общедоступная версия. Модуль готов к использованию в production-средах.',
      deprecated: 'Модуль устарел. Дальнейшее развитие и поддержка модуля прекращены.'
    },
    en: {
      search: "Search",
      values: 'values',
      allSelected: 'All selected',
      experimental: "Experimental version. The module's functionality may change significantly. Compatibility with future versions is not guaranteed.",
      preview: "Preliminary version. The module's functionality may change, but the core features remain. Compatibility with future versions is ensured, but migration may be required.",
      generalAvailability: 'General availability. The module is ready for use in production environments.',
      deprecated: 'The module is deprecated. Further development and support for this module have been discontinued.'
    }
  };

  const texts = description[lang];
  const filterSearch = document.getElementById('search-filter');
  let fullResetHandler = null;

  const editionTitles = {
    'ce': 'Community Edition',
    'be': 'Basic Edition',
    'se': 'Standard Edition',
    'se-plus': 'Standard Edition+',
    'ee': 'Enterprise Edition',
    'cse-lite': 'CSE Lite',
    'cse-pro': 'CSE Pro'
  };

  const stageTitles = {
    'experimental': 'Experimental',
    'preview': 'Preview',
    'generalAvailability': 'General Availability',
    'deprecated': 'Deprecated'
  };

  function isSectionSelectAllCheckbox(checkbox) {
    return checkbox?.dataset?.selectAll === 'true';
  }

  function getFilterContainerCheckboxes(container) {
    return Array.from(container.querySelectorAll('input[type="checkbox"]'))
      .filter(checkbox => !isSectionSelectAllCheckbox(checkbox));
  }

  function getSectionSelectAllCheckbox(container) {
    return container.querySelector('input[type="checkbox"][data-select-all="true"]');
  }

  function syncSectionSelectAllState(container) {
    const selectAllCheckbox = getSectionSelectAllCheckbox(container);
    if (!selectAllCheckbox) return;

    const sectionCheckboxes = getFilterContainerCheckboxes(container);
    const checkedCount = sectionCheckboxes.filter(checkbox => checkbox.checked).length;

    selectAllCheckbox.checked = sectionCheckboxes.length > 0 && checkedCount === sectionCheckboxes.length;
    selectAllCheckbox.indeterminate = checkedCount > 0 && checkedCount < sectionCheckboxes.length;
  }

  function setSectionCheckboxesState(container, checked) {
    const sectionCheckboxes = getFilterContainerCheckboxes(container);
    sectionCheckboxes.forEach(checkbox => {
      checkbox.checked = checked;
    });
  }

  function updateContainerTitleState(container) {
    if (!container) return null;
    const title = container.querySelector('.filter__container--title');
    if (!title) return;

    const checkboxes = getFilterContainerCheckboxes(container);
    const checkedCount = checkboxes.filter(checkbox => checkbox.checked).length;

    title.classList.toggle('filter-selected', checkedCount > 0);
    if (checkedCount > 0) {
      title.dataset.selectedCount = String(checkedCount);
    } else {
      delete title.dataset.selectedCount;
    }
  }

  function markEmptyCheckboxes() {
    const availableTags = new Set();
    const availableStages = new Set();
    const availableEditions = new Set();

    Array.from(articles).forEach(article => {
      article.querySelectorAll('.button-tile__tags .sidebar__badge--container .sidebar__badge_v2').forEach(tag => {
        availableTags.add(tag.textContent);
      });

      article.querySelectorAll('[class*="button-tile__stage-"]').forEach(el => {
        el.classList.forEach(cls => {
          if (cls.startsWith('button-tile__stage-')) {
            availableStages.add(cls.replace('button-tile__stage-', ''));
          }
        });
      });

      const editions = (article.dataset.editions || '').trim().toLowerCase();
      if (editions) {
        editions.split(',').forEach(edition => {
          const trimmedEdition = edition.trim();
          if (trimmedEdition) {
            availableEditions.add(trimmedEdition);
          }
        });
      }
    });

    document.querySelectorAll('.filter__container input[type="checkbox"]').forEach(checkbox => {
      if (isSectionSelectAllCheckbox(checkbox)) return;

      const label = checkbox.nextElementSibling ? checkbox.nextElementSibling : document.querySelector(`label[for="${checkbox.id}"]`);

      const container = checkbox.closest('.filter__container');
      let isAvailable = true;

      if (container?.classList.contains('filter__container--tags')) {
        isAvailable = availableTags.has(checkbox.value);
      } else if (container?.classList.contains('filter__container--editions')) {
        isAvailable = availableEditions.has((checkbox.value || '').trim().toLowerCase());
      } else if (container?.classList.contains('filter__container--stages')) {
        isAvailable = availableStages.has(checkbox.value);
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
    const selectedElementContainer = document.createElement('div');
    selectedElementContainer.classList.add('selected__filter--container');

    const selectedElement = document.createElement('div');
    selectedElement.classList.add('selected__filter');
    selectedElement.textContent = text;

    const removeButton = document.createElement('a');
    removeButton.classList.add('remove__filter');
    removeButton.addEventListener('click', onRemove);

    selectedElementContainer.appendChild(selectedElement);
    selectedElementContainer.appendChild(removeButton);
    return selectedElementContainer;
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

    const checkedCheckboxes = document.querySelectorAll('.filter input[type="checkbox"]:checked:not([data-select-all="true"])');
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
        const filterContainer = entry.checkboxes[0]?.closest('.filter__container');
        const isEditionsFilter = filterContainer?.classList.contains('filter__container--editions');
        const isStagesFilter = filterContainer?.classList.contains('filter__container--stages');
        const totalSectionCheckboxes = filterContainer ? getFilterContainerCheckboxes(filterContainer).length : 0;
        const selectedCount = entry.values.size;

        let valuesText;
        if (totalSectionCheckboxes > 0 && selectedCount === totalSectionCheckboxes) {
          valuesText = texts.allSelected;
        } else if (selectedCount > 3) {
          valuesText = `${selectedCount} ${texts.values}`;
        } else if (isEditionsFilter) {
          valuesText = Array.from(entry.values).map(code => editionTitles[code] || code).join(', ');
        } else if (isStagesFilter) {
          valuesText = Array.from(entry.values).map(code => stageTitles[code] || code).join(', ');
        } else {
          valuesText = Array.from(entry.values).join(', ');
        }

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

    const checkboxesEditionlChecked = document.querySelectorAll('.filter__container--editions input[type="checkbox"]:checked:not([data-select-all="true"])');
    const selectedEditions = Array.from(checkboxesEditionlChecked).map(checkbox => checkbox.value);

    const checkboxesStagesChecked = document.querySelectorAll('.filter__container--stages input[type="checkbox"]:checked:not([data-select-all="true"])');
    const selectedStages = Array.from(checkboxesStagesChecked).map(checkbox => checkbox.value);

    const checkboxesTagsChecked = document.querySelectorAll('.filter__container--tags input[type="checkbox"]:checked:not([data-select-all="true"])');
    const selectedTags = Array.from(checkboxesTagsChecked).map(checkbox => checkbox.value);

    const filtered = Array.from(articles).filter(article => {
      const titleElement = article.querySelector('h2');
      if (!titleElement) return false;

      const title = titleElement.textContent.toLowerCase();
      if(query && !title.includes(query.toLowerCase())) {
        return false;
      }

      if(selectedEditions.length > 0) {
        const articleEditionsStr = (article.dataset.editions || '').trim().toLowerCase();
        const articleEditions = articleEditionsStr
          ? articleEditionsStr.split(',').map(e => e.trim()).filter(e => e)
          : [];
        const matchesEditions = selectedEditions.some(selectedEdition => {
          const normalizedSelected = (selectedEdition || '').trim().toLowerCase();
          return articleEditions.includes(normalizedSelected);
        });
        if(!matchesEditions) {
          return false;
        }
      }

      if(selectedStages.length > 0) {
        const hasAnyStage = selectedStages.some(stage => {
          return article.querySelector(`.button-tile__stage-${stage}`) !== null;
        });
        if(!hasAnyStage) {
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
      syncSectionSelectAllState(container);
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
    const sectionCheckboxes = getFilterContainerCheckboxes(container);
    const selectAllCheckbox = getSectionSelectAllCheckbox(container);

    if (selectAllCheckbox) {
      selectAllCheckbox.addEventListener('change', function() {
        setSectionCheckboxesState(container, selectAllCheckbox.checked);
        syncSectionSelectAllState(container);
        filterArticles();
      });
    }

    sectionCheckboxes.forEach(checkbox => {
      checkbox.addEventListener('change', function() {
        updateContainerTitleState(container);
        syncSectionSelectAllState(container);
      });
    });
  })

  const checkboxes = document.querySelectorAll('.filter__container input[type="checkbox"]:not([data-select-all="true"])');
  checkboxes.forEach(checkbox => {
    checkbox.addEventListener('change', filterArticles);
  });

  initializeArticleFilter(Array.from(articles));
  document.querySelectorAll('.filter__container').forEach(container => {
    updateContainerTitleState(container);
    syncSectionSelectAllState(container);
  });
  markEmptyCheckboxes();

  function createTooltipContent(titleText, descriptionText) {
    const container = document.createElement('div');
    container.classList.add('stages-tooltip');

    const title = document.createElement('p');
    title.classList.add('stages-tooltip__title');
    title.textContent = titleText;

    const description = document.createElement('p');
    description.classList.add('stages-tooltip__descr');
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
