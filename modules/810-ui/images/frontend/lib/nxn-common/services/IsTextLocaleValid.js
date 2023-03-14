function isTextLocaleValid(text, languageName) {
  if (!languageName || languageName == 'RU') return true;
  return text.match(/[а-я]/ig) == null;
}

export default isTextLocaleValid;
