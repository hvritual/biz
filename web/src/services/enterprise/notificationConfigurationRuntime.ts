import { CommercialApiError, commercialRequestId, mutate, request } from '@/services/commercial/platformCommercial'
import { readSession, sessionContext } from '@/services/runtime/api'

export type MessageLevel = 'urgent' | 'important' | 'general'
export type MessageType = Readonly<{ code: string; name: string; level: MessageLevel }>
export type MessageChannel = Readonly<{ code: string; name: string; configurable: boolean; unavailableReason: string }>
export type MessageDirectoryEntry = Readonly<{ id: string; name: string }>
export type MessageConfiguration = Readonly<{
  id: string; tenantId: string; groupId: string; groupName: string; level: MessageLevel; channels: readonly string[]
  primaryUserId: string; secondaryUserId: string; additionalUserIds: readonly string[]; notes: string
  version: string; updatedAt: string; createdAt: string; deleted: boolean
}>
export type ConfigurationDraft = {
  groupId: string; levels: string[]; channels: string[]; primaryUserId: string; secondaryUserId: string
  additionalUserIds: string[]; notes: string
}
export type MessageConfigurationReceipt = Readonly<{ receiptId: string; tenantId: string; configurations: readonly MessageConfiguration[] }>

function safeCount(value: unknown) { const n=Number(value??0); if(!Number.isSafeInteger(n)||n<0) throw new Error('invalid-response'); return n }
function requireText(value: unknown) { if(typeof value!=='string') throw new Error('invalid-response'); return value }
function config(value: any): MessageConfiguration {
  if(!value || typeof value!=='object') throw new Error('invalid-response')
  const level=requireText(value.level) as MessageLevel
  if(!['urgent','important','general'].includes(level)) throw new Error('invalid-response')
  return Object.freeze({
    id:requireText(value.id), tenantId:requireText(value.tenantId), groupId:requireText(value.groupId), groupName:requireText(value.groupName??''),
    level, channels:Object.freeze(Array.isArray(value.channels)?value.channels.map(requireText):[]), primaryUserId:requireText(value.primaryUserId),
    secondaryUserId:requireText(value.secondaryUserId??''), additionalUserIds:Object.freeze(Array.isArray(value.additionalUserIds)?value.additionalUserIds.map(requireText):[]),
    notes:requireText(value.notes??''), version:requireText(value.version), updatedAt:requireText(value.updatedAt), createdAt:requireText(value.createdAt), deleted:Boolean(value.deleted),
  })
}
async function trustedContext() {
  const session=await readSession()
  if(!session.authenticated||!session.active_tenant_id||!session.context_version) throw new Error('unauthenticated')
  return sessionContext(session)
}
function query(values: Record<string,string|number|undefined>) {
  const q=new URLSearchParams()
  Object.entries(values).forEach(([key,value])=>{ if(value!==undefined&&String(value)!=='') q.set(key,String(value)) })
  return q.size?`?${q.toString()}`:''
}
export async function listMessageTypes(input:{code?:string;name?:string;level?:string;page:number;pageSize:number}) {
  const row=await request<any>(`/v1/tenant/notification/types${query({code:input.code,name:input.name,level:input.level,page:input.page,page_size:input.pageSize})}`)
  return { items:(row.items??[]).map((v:any)=>Object.freeze({code:requireText(v.code),name:requireText(v.name),level:requireText(v.level) as MessageLevel})), total:safeCount(row.total) }
}
export async function listMessageChannels():Promise<MessageChannel[]> {
  const row=await request<any>('/v1/tenant/notification/channels')
  return (row.items??[]).map((v:any)=>Object.freeze({code:requireText(v.code),name:requireText(v.name),configurable:Boolean(v.configurable),unavailableReason:requireText(v.unavailableReason??'')}))
}
export async function listMessageGroups(search:string,page:number,pageSize:number){ return directory('/groups',search,page,pageSize) }
export async function listMessageRecipients(search:string,page:number,pageSize:number){ return directory('/recipients',search,page,pageSize) }
async function directory(path:string,search:string,page:number,pageSize:number){
  const row=await request<any>(`/v1/tenant/notification${path}${query({query:search,page,page_size:pageSize})}`)
  return {items:(row.items??[]).map((v:any)=>Object.freeze({id:requireText(v.id),name:requireText(v.name)})),total:safeCount(row.total)}
}
export async function listMessageConfigurations(input:{groupId?:string;level?:string;recipientId?:string;page:number;pageSize:number}) {
  const row=await request<any>(`/v1/tenant/notification/configurations${query({group_id:input.groupId,level:input.level,recipient_id:input.recipientId,page:input.page,page_size:input.pageSize})}`)
  return {items:(row.items??[]).map(config),total:safeCount(row.total)}
}
function receipt(value:any):MessageConfigurationReceipt {
  if(!value||typeof value!=='object') throw new Error('invalid-response')
  return Object.freeze({receiptId:requireText(value.receiptId),tenantId:requireText(value.tenantId),configurations:Object.freeze((value.configurations??[]).map(config))})
}
async function write(path:string,method:'POST'|'PATCH',body:unknown,prefix:string){
  const context=await trustedContext()
  return receipt(await mutate<any>(path,method,body,{idempotencyKey:commercialRequestId(prefix),sessionContext:context}))
}
export function createMessageConfigurations(draft:ConfigurationDraft){ return write('/v1/tenant/notification/configurations','POST',{groupId:draft.groupId,levels:draft.levels,channels:draft.channels,primaryUserId:draft.primaryUserId,secondaryUserId:draft.secondaryUserId,additionalUserIds:draft.additionalUserIds,notes:draft.notes},'notification-config-create') }
export function updateMessageConfiguration(id:string,version:string,draft:ConfigurationDraft){ return write(`/v1/tenant/notification/configurations/${encodeURIComponent(id)}`,'PATCH',{expectedVersion:version,channels:draft.channels,primaryUserId:draft.primaryUserId,secondaryUserId:draft.secondaryUserId,additionalUserIds:draft.additionalUserIds,notes:draft.notes},'notification-config-update') }
export function deleteMessageConfiguration(id:string,version:string){ return write(`/v1/tenant/notification/configurations/${encodeURIComponent(id)}/delete`,'POST',{expectedVersion:version},'notification-config-delete') }
export function notificationConfigurationErrorKey(error:unknown){
  if(error instanceof CommercialApiError){ if(error.status===401)return 'unauthenticated'; if(error.status===403)return 'forbidden'; if(error.status===404)return 'notFound'; if(error.message==='NOTIFICATION_DUPLICATE')return 'duplicate'; if(error.message==='NOTIFICATION_CHANNEL_UNAVAILABLE'||error.status===412)return 'channelUnavailable'; if(error.status===409)return 'conflict'; if(error.status===400||error.status===422)return 'invalid' }
  return 'unavailable'
}
