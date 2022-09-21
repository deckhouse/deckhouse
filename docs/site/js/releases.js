function getPageLocale() {
  return $('.header__logo > a').first().attr('href') === '/ru/' ? 'ru' : 'en';
}

function getHue4Version(version) {
  let result = 0
  for (let i = 0; i < version.length; ++i)
    result = Math.imul(31, result) + version.charCodeAt(i)
  result = Math.abs(result)
  return result
}

function formatDate(date) {
  return new Intl.DateTimeFormat(getPageLocale() === 'ru' ? 'ru-RU' : 'en-US', {
      weekday: 'short',
      day: 'numeric',
      month: 'short'
  }).format(date);
}

async function showReleaseChannelStatus(apiURL) {
  const ghURL = 'https://github.com/deckhouse/deckhouse'
  const channelCodes = {
    "alpha": 'a',
    "beta": 'b',
    "ea": 'ea',
    "stable": 's',
    "rs": 'rs' };
  const editions = ['ee', 'ce'];

  await fetch(apiURL, {
      headers: {
        'Accept': 'application/json'
      },
    })
    .then(respose => respose.json())
    .then(data => {
      for (const edition of editions) {
        for (const channelItem in channelCodes) {
           const itemData = data.releases[channelCodes[channelItem] + '-' + edition];
           const itemElement = $(`.releases-page__table--content td.${channelItem}.${edition}`);
           const date = new Date(Date.parse(itemData['date']))
           itemElement.find('.version span a').html(`${itemData['version'].replace(/^v/,'')}`).attr('href', `${ghURL}/releases/tag/${itemData['version']}/`);
           itemElement.find('.version span').first().css('background-color', `hsl(${getHue4Version(itemData['version'])}, 50%, 85%)`);
           itemElement.find('.date').html(`${formatDate(date)}`);
           itemElement.find('.doc a').attr('href', `../${itemData['version']}/`);
        }
      }
    })
}

document.addEventListener("DOMContentLoaded", function () {
  const apiURL = 'https://flow.deckhouse.io/releases';
  showReleaseChannelStatus(apiURL)
    .then(() =>  {
      $('.releases-page__table--content').addClass('active');
      })
    .catch((reason) => {
      $('.releases-page__loadblock.failed').addClass('active');
      console.log(`Failed to fetch release channel data from ${apiURL}.`, reason)
      })
    .finally( () => {
      $('.releases-page__loadblock.progress').removeClass('active')
      });
});

$(document).ready(function () {
    tippy('[data-tippy-content]', {
        interactive: false,
        interactiveDebounce: 15,
        maxWidth: 600,
        theme: 'custom',
        allowHTML: true,
        arrow: false,
      });
});
