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
      dateStyle: 'full',
      timeStyle: 'short'
  }).format(date);
}

document.addEventListener("DOMContentLoaded", function () {
  const url = 'https://flow.deckhouse.io/deployments';

  fetch(url, {
      headers: {
        'Accept': 'application/json'
      },
    })
    .then(respose => respose.json())
    .then(data => {
      for (const item of data.deployments) {
        const trBody = document.createElement('tr');
        const channel = document.createElement('td');
        const version = document.createElement('td');
        const state = document.createElement('td');
        const link = document.createElement('a');
        const date = new Date(Date.parse(item['updated']))


        channel.innerText = item['channel'];
        version.innerText = item['version'];
        link.href = item['log'];
        link.innerText = item['state'];
        link.style.paddingLeft = '5px';
        state.innerText = ` at ${formatDate(date)}`;

        state.prepend(link);
        const icon = icons[item['state']];
        if (icon) {
          state.prepend(icon);
        }

        trBody.append(channel, version, state);
        tbody.append(trBody);
      }
    });

  const root = document.querySelector('.releases-page__table--wrap');
  const table = document.createElement('table');
  const thead = document.createElement('thead');
  const trHead = document.createElement('tr');
  const tbody = document.createElement('tbody');

  let th = document.createElement('th');

  th.innerText = 'Channel';
  trHead.append(th);
  thead.append(trHead);

  th = document.createElement('th');
  th.innerText = 'Version';
  trHead.append(th);

  th = document.createElement('th');
  th.innerText = 'State';
  trHead.append(th);

  table.append(thead);
  table.append(tbody);
  root.append(table);
});
