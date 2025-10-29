<script>
function toggleDetails(id) {
  const details = document.getElementById(id);
  const button = event.target;
  const summaryRow = button.closest('tr');
  const cells = summaryRow.querySelectorAll('td');

  if (details.style.display === 'none') {
    details.style.display = 'table-row-group';
    button.textContent = '[{{ site.data.i18n.common["hide_details"][page.lang] }}]';
    cells.forEach(cell => cell.style.fontWeight = 'bold');
  } else {
    details.style.display = 'none';
    button.textContent = '[{{ site.data.i18n.common["show_details"][page.lang] }}]';
    cells.forEach(cell => cell.style.fontWeight = 'normal');
  }
}
</script>
