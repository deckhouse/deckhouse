import CurrentUser from 'nxn-common/services/CurrentUser.js';

const I18n = require('i18n-js');

I18n.defaultLocale = 'en';
I18n.locale = CurrentUser.allowed_locales ? CurrentUser.allowed_locales[0] : I18n.defaultLocale;

// TODO: allow locale selection?

require('i18n/ru.js');
require('i18n/en.js');

export default I18n;
