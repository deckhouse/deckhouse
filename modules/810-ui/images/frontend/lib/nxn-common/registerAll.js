import asWorkTime from './filters/asWorkTime.js';
import dateFilter from './filters/date.js';
import translateFilter from './filters/translate.js';
import truncateFilter from './filters/truncate.js';

import NxnFlash from './components/NxnFlash.vue';
import NxnDate from './components/NxnDate.vue';

export default function registerAll(app) {
  app.config.globalProperties.$filters.asWorkTime = asWorkTime;
  app.config.globalProperties.$filters.date = dateFilter;
  app.config.globalProperties.$filters.truncate = truncateFilter;

  app.config.globalProperties.translate = translateFilter;

  app.component('nxn-flash', NxnFlash);
  app.component('nxn-date', NxnDate);
}