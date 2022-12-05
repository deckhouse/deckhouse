import en from "./en"
//import ru from "./ru"

export enum Locales {
  EN = "en",
  //RU = 'ru',
}

export const LOCALES = [
  { value: Locales.EN, caption: "English" },
  //{value: Locales.RU, caption: "Русский"},
]

export type LangPack = typeof en
export type MuteItems = typeof en.mute.items

export const translations = {
  [Locales.EN]: en,
  //[Locales.RU]: ru
}

export const defaultLocale = Locales.EN

export const i18n = (): LangPack => {
  return translations[defaultLocale]
}
