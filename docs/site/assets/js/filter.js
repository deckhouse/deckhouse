$(document).ready(function () {
  const filterCheckboxes = document.querySelector('.filter__checkboxes');
  const articles = document.querySelectorAll('.button-tile');
  const resetButton = document.querySelector('.reset-check');

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

  function filterArticles() {
    const checkboxesChecked = filterCheckboxes.querySelectorAll('input[type="checkbox"]:checked');
    const selectedTags = Array.from(checkboxesChecked).map(checkbox => checkbox.value);
    articles.forEach(article => {
      const tagElement = Array.from(article.querySelectorAll('.button-tile__tags .sidebar__badge--container .sidebar__badge_v2')).map(tag => tag.textContent);

      const shouldShow = selectedTags.length === 0 || selectedTags.every(tag => tagElement.includes(tag));
      article.style.display = shouldShow ? 'flex' : 'none';
    });

    if (checkboxesChecked.length > 0) {
      resetButton.classList.add('active');
    } else {
      resetButton.classList.remove('active');
    }
  }

  function createFilters() {
    const tags = getTags();
    tags.forEach(tag => {
      createCheckboxes(tag);
    });

    filterCheckboxes.querySelectorAll('input[type="checkbox"]').forEach(checkbox => checkbox.addEventListener('change', filterArticles));
  }

  resetButton.addEventListener('click', () => {
    const checkboxes = filterCheckboxes.querySelectorAll('input[type="checkbox"]');
    checkboxes.forEach(checkbox => {
      checkbox.checked = false;
    });
    filterArticles();
  })

  createFilters();
  filterArticles();
})
