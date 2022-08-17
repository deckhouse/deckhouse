const icons = {
  inactive: '😌 ',
  queued: '🚀 ',
  in_progress: '⏳ ',
  pending: '🤔 ',
  success: '✅ ',
  failure: '❌ ',
  error: '🛑 '
};

function formatDate(date) {
  return new Intl.DateTimeFormat('en-US', {
      dateStyle: 'short',
      timeStyle: 'short'
  }).format(date);
}

document.addEventListener("DOMContentLoaded", function () {
  const url = 'https://flow.deckhouse.io/releases';

  const channels = ['a', 'b', 'ea', 's', 'rs'];
  const channelCodes = {
    "Alpha": 'a',
    "Beta": 'b',
    "Early Access": 'ea',
    "Stable": 's',
    "Rock Solid": 'rs' };
  const editions = ['fe', 'ee', 'ce'];

  const root = document.querySelector('.releases-page__table--wrap');
  const table = document.createElement('table');
  const thead = document.createElement('thead');
  const trHead = document.createElement('tr');
  const tbody = document.createElement('tbody');

  fetch(url, {
      headers: {
        'Accept': 'application/json'
      },
    })
    .then(respose => respose.json())
    .then(data => {
      for (const channelItem in channelCodes) {
        const trBody = document.createElement('tr');
        const channel = document.createElement('td');
        channel.innerText = channelItem[0].toUpperCase() + channelItem.slice(1);
        trBody.append(channel)
        for (const edition of editions) {
           const itemData = data.releases[channelCodes[channelItem] + '-' + edition]
           const rawItem = document.createElement('td');
           const link = document.createElement('a');
           const date = new Date(Date.parse(itemData['date']))
           link.href = `../${itemData['version']}/`;
           link.innerText = itemData['version'];
           rawItem.innerText = ` (${formatDate(date)})`;
           rawItem.prepend(link);
           trBody.append(rawItem)
        }
        tbody.append(trBody);
      }

    });

  let th = document.createElement('th');

  th.innerText = 'Channel';
  trHead.append(th);
  thead.append(trHead);

  for (const edition of editions) {
    th = document.createElement('th');
    th.innerText = edition.toUpperCase();
    trHead.append(th);
  }

  table.append(thead);
  table.append(tbody);
  root.append(table);
});
