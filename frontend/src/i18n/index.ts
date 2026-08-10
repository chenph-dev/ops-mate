import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';
import zhCN from '@/locales/zh-CN';
import enUS from '@/locales/en-US';

const STORAGE_KEY = 'ops-mate-lang';

export type AppLang = 'zh-CN' | 'en-US';

/** 读取上次选择的语言，缺失或非法时回落默认简体中文。 */
function detectLang(): AppLang {
  try {
    const saved = localStorage.getItem(STORAGE_KEY);
    if (saved === 'zh-CN' || saved === 'en-US') return saved;
  } catch {
    // localStorage 不可用时忽略，使用默认语言
  }
  return 'zh-CN';
}

export function changeLanguage(lang: AppLang): void {
  try {
    localStorage.setItem(STORAGE_KEY, lang);
  } catch {
    // 忽略存储失败，仅切换当前会话语言
  }
  void i18n.changeLanguage(lang);
}

i18n.use(initReactI18next).init({
  lng: detectLang(),
  fallbackLng: 'zh-CN',
  defaultNS: 'common',
  resources: {
    'zh-CN': zhCN,
    'en-US': enUS,
  },
  interpolation: {
    escapeValue: false, // React 已做转义，无需 i18next 再转义
  },
  returnEmptyString: false,
});

export default i18n;
