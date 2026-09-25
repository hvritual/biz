import { createI18n } from 'vue-i18n'
import { messages } from './messages'
import { memberDetailMessages } from './member-detail-messages'
import { memberScopeMessages } from './scope-messages'
import { brandingMessages } from './branding-messages'
import { appearanceMessages } from './appearance-messages'
import { backendTermMessages } from './backend-term-messages'
import { dataPermissionMessages } from './data-permission-messages'
import { personalProfileMessages } from './personal-profile-messages'
import { personalSecurityMessages } from './personal-security-messages'
import { notificationPreferenceMessages } from './notification-preference-messages'

export const supportedLocales = ['zh-CN', 'en-US'] as const
export type UiLocale = (typeof supportedLocales)[number]
const storageKey = 'coffeelink.locale'
const localeMessages = {
  'zh-CN': { ...messages['zh-CN'], dataPermissions: dataPermissionMessages['zh-CN'], appearance: appearanceMessages['zh-CN'], backendTerms: backendTermMessages['zh-CN'], navigation: { ...messages['zh-CN'].navigation, enterprise: { ...messages['zh-CN'].navigation.enterprise, branding: brandingMessages['zh-CN'].navigation, personal: personalProfileMessages['zh-CN'].navigation } }, branding: brandingMessages['zh-CN'], personalProfile: personalProfileMessages['zh-CN'], personalSecurity: personalSecurityMessages['zh-CN'], notificationPreferences: notificationPreferenceMessages['zh-CN'], members: { ...messages['zh-CN'].members, scopes: memberScopeMessages['zh-CN'], profile: memberDetailMessages['zh-CN'] } },
  'en-US': { ...messages['en-US'], dataPermissions: dataPermissionMessages['en-US'], appearance: appearanceMessages['en-US'], backendTerms: backendTermMessages['en-US'], navigation: { ...messages['en-US'].navigation, enterprise: { ...messages['en-US'].navigation.enterprise, branding: brandingMessages['en-US'].navigation, personal: personalProfileMessages['en-US'].navigation } }, branding: brandingMessages['en-US'], personalProfile: personalProfileMessages['en-US'], personalSecurity: personalSecurityMessages['en-US'], notificationPreferences: notificationPreferenceMessages['en-US'], members: { ...messages['en-US'].members, scopes: memberScopeMessages['en-US'], profile: memberDetailMessages['en-US'] } },
}
function supported(value: unknown): value is UiLocale { return typeof value === 'string' && supportedLocales.includes(value as UiLocale) }
function initialLocale(): UiLocale { if (typeof window !== 'undefined') { const saved=window.localStorage.getItem(storageKey); if(supported(saved))return saved; if(window.navigator.language?.toLowerCase().startsWith('zh'))return'zh-CN' } return'en-US' }
export const i18n=createI18n({legacy:false,locale:initialLocale(),fallbackLocale:'zh-CN',messages:localeMessages,missingWarn:false,fallbackWarn:false})
export function currentUiLocale():UiLocale{const value=i18n.global.locale.value;return supported(value)?value:'zh-CN'}
export function setUiLocale(locale:UiLocale){if(!supported(locale))throw new Error(`Unsupported UI locale: ${String(locale)}`);i18n.global.locale.value=locale;if(typeof document!=='undefined')document.documentElement.lang=locale;if(typeof window!=='undefined')window.localStorage.setItem(storageKey,locale)}
export function initializeUiLocale(){setUiLocale(currentUiLocale())}
export function t(key:string,params?:Record<string,unknown>):string{return String(params?i18n.global.t(key,params):i18n.global.t(key))}
export function tx(key:string,fallback:string,params?:Record<string,unknown>):string{return i18n.global.te(key)?t(key,params):fallback}
export function formatNumber(value:number|string,options:Intl.NumberFormatOptions={}):string{const numeric=typeof value==='number'?value:Number(value);return Number.isFinite(numeric)?new Intl.NumberFormat(currentUiLocale(),options).format(numeric):String(value)}
export function formatCurrency(value:number,currency:string,options:Omit<Intl.NumberFormatOptions,'style'|'currency'>={}):string{return new Intl.NumberFormat(currentUiLocale(),{...options,style:'currency',currency}).format(value)}
export function formatPercent(value:number,options:Omit<Intl.NumberFormatOptions,'style'>={}):string{return new Intl.NumberFormat(currentUiLocale(),{...options,style:'percent'}).format(value)}
export function formatDateTime(value:string|number|Date,timeZone:string,options:Intl.DateTimeFormatOptions={}):string{const date=value instanceof Date?value:new Date(value);if(Number.isNaN(date.getTime()))return String(value);return new Intl.DateTimeFormat(currentUiLocale(),{year:'numeric',month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit',...options,timeZone}).format(date)}
export function formatDateOnly(value:string,options:Intl.DateTimeFormatOptions={}):string{const match=/^(\d{4})-(\d{2})-(\d{2})$/.exec(value);if(!match)return value;const date=new Date(Date.UTC(Number(match[1]),Number(match[2])-1,Number(match[3]),12));return new Intl.DateTimeFormat(currentUiLocale(),{year:'numeric',month:'2-digit',day:'2-digit',...options,timeZone:'UTC'}).format(date)}
export function formatRelativeTime(value:string|number|Date,now:string|number|Date=Date.now()):string{const date=value instanceof Date?value:new Date(value),base=now instanceof Date?now:new Date(now);if(Number.isNaN(date.getTime())||Number.isNaN(base.getTime()))return String(value);const seconds=(date.getTime()-base.getTime())/1000;const units:Array<[Intl.RelativeTimeFormatUnit,number]>=[['year',31536000],['month',2592000],['day',86400],['hour',3600],['minute',60],['second',1]];const [unit,scale]=units.find(([,amount])=>Math.abs(seconds)>=amount)??['second',1];return new Intl.RelativeTimeFormat(currentUiLocale(),{numeric:'auto'}).format(Math.round(seconds/scale),unit)}
export function formatUnit(value:number,unit:Intl.NumberFormatOptions['unit'],options:Omit<Intl.NumberFormatOptions,'style'|'unit'>={}):string{return new Intl.NumberFormat(currentUiLocale(),{...options,style:'unit',unit}).format(value)}
