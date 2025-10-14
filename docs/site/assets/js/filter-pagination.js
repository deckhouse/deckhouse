// Filter and pagination logic for modules and guides
// When adding the pagination logic again, add the filter-pagination file.js instead of filter.js
// Also add 'show more' buttons

$(document).ready(function () {
  const articles = document.querySelectorAll('.button-tile');
  const moreButton = document.querySelector('.tile__pagination');
  const filterCheckboxes = document.querySelector('.filter__checkboxes');
  const resetButton = document.querySelector('.reset-check');
  const itemsPerPage = 12;
  let filteredArticles = [];
  let count = 0;

  function hideAllItems() {
    articles.forEach(article => article.style.display = 'none');
  }

  function showItems() {
    const end = Math.min(count + itemsPerPage, filteredArticles.length);

    for(let i = count; i < end; i++) {
        filteredArticles[i].style.display = 'flex';
    }

    count = end;

    if(count >= filteredArticles.length) {
      moreButton.style.display = 'none';
    } else {
      moreButton.style.display = 'flex';
    }
  }

  function initializeArticlePagination(articlesToPagination) {
    filteredArticles = articlesToPagination;
    count = 0;
    hideAllItems();

    if(filteredArticles.length <= itemsPerPage) {
      filteredArticles.forEach(list => list.style.display = 'flex');
      moreButton.style.display = 'none';
    } else {
      showItems();
    }
  }

  function filterArticles() {
    const checkboxesChecked = filterCheckboxes.querySelectorAll('input[type="checkbox"]:checked');
    const selectedTags = Array.from(checkboxesChecked).map(checkbox => checkbox.value);

    const filtered = Array.from(articles).filter(article => {
      const tagElement = Array.from(article.querySelectorAll('.button-tile__tags .sidebar__badge--container .sidebar__badge_v2')).map(tag => tag.textContent);
      return selectedTags.length === 0 || selectedTags.every(tag => tagElement.includes(tag));
    })

    initializeArticlePagination(filtered);

    if (checkboxesChecked.length > 0) {
      resetButton.classList.add('active');
    } else {
      resetButton.classList.remove('active');
    }
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

    filterCheckboxes.appendChild(input);
    filterCheckboxes.appendChild(label);
  }

  function createFilters() {
    const tags = getTags();
    tags.forEach(tag => {
      createCheckboxes(tag);
    });

    filterCheckboxes.querySelectorAll('input[type="checkbox"]').forEach(checkbox => checkbox.addEventListener('change', filterArticles));
  }

  createFilters();
  initializeArticlePagination(Array.from(articles));
  moreButton.addEventListener('click', showItems);

  resetButton.addEventListener('click', () => {
    const checkboxes = filterCheckboxes.querySelectorAll('input[type="checkbox"]');
    checkboxes.forEach(checkbox => {
      checkbox.checked = false;
    });
    filterArticles();
  })
})