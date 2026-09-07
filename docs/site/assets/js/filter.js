document.addEventListener('DOMContentLoaded', () => {
  const articles = document.querySelectorAll('.button-tile');
  const selectedFiltersList = document.querySelector('.selected__filters--list');
  const filterCheckboxesTags = document.querySelector('.filter__checkboxes--tags');
  const applyButton = document.querySelector('.apply-filter');
  const applyButtonBlock = document.querySelector('.filter__block--apply');
  const openMobile =  document.querySelector('.filter__search--filter');
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

  const urlSearch = 'search';
  const urlEdition = 'edition';
  const urlStage = 'stage';
  const urlTag = 'tag';
  const urlCertification = 'certification';
  const urlParamAll = '__all__';

  // The value of the tag indicating the inclusion of the module in the evaluation object (certification).
  const certifiedTag = 'certified';

  const description = {
    ru: {
      search: 'Поиск',
      values: 'знач.',
      allSelected: 'Выбраны все',
      experimental: 'Экспериментальная версия. Функциональность модуля может сильно измениться. Совместимость с будущими версиями не гарантируется.',
      preview: 'Предварительная версия. Функциональность модуля может измениться, но основные возможности сохранятся. Совместимость с будущими версиями обеспечивается, но может потребоваться миграция.',
      deprecated: 'Модуль устарел. Дальнейшее развитие и поддержка модуля прекращены.'
    },
    en: {
      search: "Search",
      values: 'values',
      allSelected: 'All selected',
      experimental: "Experimental version. The module's functionality may change significantly. Compatibility with future versions is not guaranteed.",
      preview: "Preliminary version. The module's functionality may change, but the core features remain. Compatibility with future versions is ensured, but migration may be required.",
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

  const certificationTitles = {
    ru: {
      'certified': 'Сертифицирован',
      'notCertified': 'Не сертифицирован'
    },
    en: {
      'certified': 'Included in the evaluation scope',
      'notCertified': 'Not included in the evaluation scope'
    }
  };

  function getCertificationTitle(value) {
    const localizedTitles = certificationTitles[lang] || certificationTitles.en;
    return localizedTitles[value] || value;
  }

  // Localized tag titles. The key is the tag value in lowercase.
  // If there is no translation, auto-formatting is used via capitalizeWords.
  const tagTitles = {
    ru: {},
    en: {}
  };

  function getTagTitle(tag) {
    const normalized = (tag || '').trim();
    const localizedTitles = tagTitles[lang] || {};
    return localizedTitles[normalized.toLowerCase()] || capitalizeWords(normalized);
  }

  // Localized tooltip texts for tags. The key is the tag value in lowercase.
  // If there is no text, the tooltip is not displayed.
  const tagTooltips = {
    ru: {
      'ssdlc': 'Разработка модуля ведется в соответствии с процессами по разработке безопасного программного обеспечения согласно ГОСТ РФ.'
    },
    en: {
      'ssdlc': 'Module development is carried out in accordance with the processes for developing secure software.'
    }
  };

  function getTagTooltip(tag) {
    const normalized = (tag || '').trim();
    const localizedTooltips = tagTooltips[lang] || {};
    return localizedTooltips[normalized.toLowerCase()] || '';
  }

  // Localized tooltip texts for certification filter items. The key is the checkbox value.
  // If there is no text, the tooltip is not displayed.
  const certificationTooltips = {
    ru: {
      'certified': 'Модуль входит в объект оценки и прошёл сертификацию на соответствие требованиям ФСТЭК.',
      'notCertified': 'Модуль не входит в объект оценки и не проходил сертификацию на соответствие требованиям ФСТЭК, но должен устанавливаться из доверенного источника.'
    },
    en: {}
  };

  function getCertificationTooltip(value) {
    const localizedTooltips = certificationTooltips[lang] || {};
    return localizedTooltips[value] || '';
  }

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

  const uppercaseAcronyms = { 'ml': 'ML', 'ai': 'AI', 'ci': 'CI', 'cd': 'CD', 'api': 'API' , 'ssdlc': 'SSDLC' };

  function capitalizeWords(value) {
    return value.trim().split(/(\s+|(?=[/])|(?<=[/]))/).map(part => {
      const lower = part.toLowerCase();
      if (uppercaseAcronyms[lower]) return uppercaseAcronyms[lower];
      return part ? part.charAt(0).toUpperCase() + part.slice(1) : part;
    }).join('');
  }

  function markEmptyCheckboxes() {
    const availableTags = new Set();
    const availableStages = new Set();
    const availableEditions = new Set();
    const availableCertifications = new Set();

    Array.from(articles).forEach(article => {
      const articleTags = Array.from(
        article.querySelectorAll('.button-tile__tags .sidebar__badge--container .sidebar__badge_v2')
      ).map(tag => tag.textContent);

      articleTags.forEach(tag => availableTags.add(tag));

      availableCertifications.add(articleTags.includes(certifiedTag) ? 'certified' : 'notCertified');

      article.querySelectorAll('[class*="button-tile__stage-"]').forEach(el => {
        el.classList.forEach(cls => {
          if (cls.startsWith('button-tile__stage-')) {
            availableStages.add(cls.replace('button-tile__stage-', ''));
          }
        });
      });

      article.querySelectorAll('.button-tile__stage[data-stage]').forEach(el => {
        const stage = el.dataset.stage;
        if (stage) {
          availableStages.add(stage);
        }
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
      } else if (container?.classList.contains('filter__container--certification')) {
        isAvailable = availableCertifications.has(checkbox.value);
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
        // The certified tag is processed in a separate "certification" filter group.
        if (tag.textContent === certifiedTag) return;
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
    label.textContent = getTagTitle(tag);

    filterCheckboxesTags.appendChild(input);
    filterCheckboxesTags.appendChild(label);

    const tooltip = getTagTooltip(tag);
    if (tooltip) {
      initTooltip(label, getTagTitle(tag), tooltip);
    }
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

  function buildFilterParams() {
    const params = new URLSearchParams();
    const query = filterSearch ? filterSearch.value.trim() : '';
    if (query) {
      params.set(urlSearch, query);
    }

    const appendSectionParams = (containerSelector, paramName) => {
      const container = document.querySelector(containerSelector);
      if (!container) return;

      const selectAllCheckbox = getSectionSelectAllCheckbox(container);
      if (selectAllCheckbox?.checked) {
        params.set(paramName, urlParamAll);
        return;
      }

      const checkedValues = getFilterContainerCheckboxes(container)
        .filter(checkbox => checkbox.checked)
        .map(checkbox => checkbox.value);

      if (checkedValues.length > 0) {
        params.set(paramName, checkedValues.join(','));
      }
    };

    appendSectionParams('.filter__container--editions', urlEdition);
    appendSectionParams('.filter__container--certification', urlCertification);
    appendSectionParams('.filter__container--stages', urlStage);
    appendSectionParams('.filter__container--tags', urlTag);

    return params;
  }

  function syncUrlWithFilters() {
    const params = buildFilterParams();
    const qs = params.toString();
    const nextSearch = qs ? `?${qs}` : '';

    if (window.location.search === nextSearch) {
      return;
    }

    history.replaceState(null, '', `${nextSearch}${window.location.hash}` || window.location.pathname);
  }

  function applyFiltersFromUrl() {
    const params = new URLSearchParams(window.location.search);
    if (!params.has(urlSearch) && !params.has(urlEdition) && !params.has(urlCertification) && !params.has(urlStage) && !params.has(urlTag)) {
      return;
    }

    if (filterSearch && params.has(urlSearch)) {
      filterSearch.value = params.get(urlSearch) || '';
    }

    const parseSectionValues = (paramName, normalizeValue) => {
      const values = params.getAll(paramName)
        .flatMap(value => (value || '').split(','))
        .map(value => normalizeValue(value))
        .filter(value => value !== '');
      return new Set(values);
    };

    const wantedEditions = parseSectionValues(urlEdition, value => value.trim().toLowerCase());
    const wantedCertifications = parseSectionValues(urlCertification, value => value.trim());
    const wantedStages = parseSectionValues(urlStage, value => value.trim());
    const wantedTags = parseSectionValues(urlTag, value => value.trim());

    const applySectionParams = (containerSelector, wantedValues, normalizeValue = value => value) => {
      const container = document.querySelector(containerSelector);
      if (!container) return;

      const selectAllCheckbox = getSectionSelectAllCheckbox(container);
      if (wantedValues.has(urlParamAll)) {
        setSectionCheckboxesState(container, true);
        if (selectAllCheckbox) {
          selectAllCheckbox.checked = true;
          selectAllCheckbox.indeterminate = false;
        }
        return;
      }

      getFilterContainerCheckboxes(container).forEach(checkbox => {
        checkbox.checked = wantedValues.has(normalizeValue(checkbox.value || ''));
      });
    };

    applySectionParams('.filter__container--editions', wantedEditions, value => value.trim().toLowerCase());
    applySectionParams('.filter__container--certification', wantedCertifications, value => value.trim());
    applySectionParams('.filter__container--stages', wantedStages, value => value.trim());
    applySectionParams('.filter__container--tags', wantedTags, value => value.trim());

    document.querySelectorAll('.filter__container').forEach(container => {
      syncSectionSelectAllState(container);
      updateContainerTitleState(container);
    });
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

    const hasCheckedCheckboxes = checkedCheckboxes.length > 0;

    if (applyButton) {
      applyButton.disabled = !hasCheckedCheckboxes;
    }

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
        const isCertificationFilter = filterContainer?.classList.contains('filter__container--certification');
        const isStagesFilter = filterContainer?.classList.contains('filter__container--stages');
        const isTagsFilter = filterContainer?.classList.contains('filter__container--tags');
        const totalSectionCheckboxes = filterContainer ? getFilterContainerCheckboxes(filterContainer).length : 0;
        const selectedCount = entry.values.size;

        let valuesText;
        if (totalSectionCheckboxes > 0 && selectedCount === totalSectionCheckboxes) {
          valuesText = texts.allSelected;
        } else if (selectedCount > 3) {
          valuesText = `${selectedCount} ${texts.values}`;
        } else if (isEditionsFilter) {
          valuesText = Array.from(entry.values).map(code => editionTitles[code] || code).join(', ');
        } else if (isCertificationFilter) {
          valuesText = Array.from(entry.values).map(value => getCertificationTitle(value)).join(', ');
        } else if (isStagesFilter) {
          valuesText = Array.from(entry.values).map(code => stageTitles[code] || code).join(', ');
        } else if (isTagsFilter) {
          valuesText = Array.from(entry.values).map(value => getTagTitle(value)).join(', ');
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

    const checkboxesCertificationChecked = document.querySelectorAll('.filter__container--certification input[type="checkbox"]:checked:not([data-select-all="true"])');
    const selectedCertifications = Array.from(checkboxesCertificationChecked).map(checkbox => checkbox.value);

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

      if(selectedCertifications.length > 0) {
        const isCertified = Array.from(
          article.querySelectorAll('.button-tile__tags .sidebar__badge--container .sidebar__badge_v2')
        ).some(tag => tag.textContent === certifiedTag);
        const articleCertification = isCertified ? 'certified' : 'notCertified';
        if(!selectedCertifications.includes(articleCertification)) {
          return false;
        }
      }

      if(selectedStages.length > 0) {
        const hasAnyStage = selectedStages.some(stage => {
          return article.querySelector(`.button-tile__stage-${stage}`) !== null
            || article.querySelector(`.button-tile__stage[data-stage="${stage}"]`) !== null;
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

    syncUrlWithFilters();
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

  markEmptyCheckboxes();
  applyFiltersFromUrl();

  if (openMobile) {
    const filter = document.querySelector('.filter__block');
    const hamburgerCollapse = document.querySelector('.hamburger--collapse');
    const body = document.body;

    function closeFilterMobilePanel() {
      if (filter) filter.classList.remove('show');
      if (hamburgerCollapse) hamburgerCollapse.classList.remove('show');
      if (body) body.classList.remove('filter-opened');
    }

    openMobile.addEventListener('click', () => {
      if (!filter) return;
      filter.classList.add('show');
      if (hamburgerCollapse) hamburgerCollapse.classList.add('show');
      if (body) body.classList.add('filter-opened');
    });

    if (applyButtonBlock) {
      applyButtonBlock.addEventListener('click', () => {
        if (applyButton && !applyButton.disabled) {
          closeFilterMobilePanel();
        }
      });
    }
  }

  filterArticles();
  window.addEventListener('pageshow', () => {
    filterArticles();
  });

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

  function initTooltip(target, titleText, descriptionText) {
    const elements = typeof target === 'string'
      ? document.querySelectorAll(target)
      : [].concat(target);
    if (elements.length === 0) return;

    elements.forEach(element => {
      const tooltip = tippy(element, {
        allowHTML: true,
        content: () => createTooltipContent(titleText, descriptionText),
        arrow: true,
        appendTo: 'parent',
        hideOnClick: false,
        delay: [300, 50],
        offset: [0, 10],
        duration: [300],
      });

      element.addEventListener('click', () => {
        setTimeout(() => {
          tooltip.hide();
        }, 5000);
      });
    })
  }

  document.querySelectorAll('.button-tile__stage img').forEach(img => {
    img.addEventListener('click', (e) => {
      e.preventDefault();
      e.stopPropagation();
    });
  });

  initTooltip('.filter__container label[for="experimental"] > img, .button-tile__stage-experimental > img', 'Experimental', texts.experimental);
  initTooltip('.filter__container label[for="preview"] > img, .button-tile__stage-preview > img', 'Preview', texts.preview);
  initTooltip('.filter__container label[for="deprecated"] > img, .button-tile__stage-deprecated > img', 'Deprecated', texts.deprecated);

  document.querySelectorAll('.filter__container--certification input[type="checkbox"]').forEach(checkbox => {
    if (isSectionSelectAllCheckbox(checkbox)) return;
    const tooltip = getCertificationTooltip(checkbox.value);
    if (!tooltip) return;
    const label = document.querySelector(`.filter__container--certification label[for="${checkbox.id}"]`);
    if (label) {
      initTooltip(label, getCertificationTitle(checkbox.value), tooltip);
    }
  });
})
