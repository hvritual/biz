import { expect, test, type Page, type Route } from '@playwright/test'
import { mkdirSync } from 'node:fs'
import { selectUiOption } from './ui.helpers'

test.skip(!process.env.ENTERPRISE_DATA_PERMISSION_REAL_E2E, 'runs only against the VITE_DATA_MODE=api build')

type PolicyRef = { policyId: string; policyName: string; policyVersion: number; acceptedVersion: number; effective: boolean; invalidReason: string }
type Policy = { id: string; name: string; status: string; siteIds: string[]; version: number; notBefore: string; expiresAt: string; effective: boolean; invalidReason: string }
type Grant = { permission: string; scope: string }
type Role = { id: string; name: string; description: string; roleCode: string; systemRole: boolean; memberCount: number; status: string; version: number; permissions: Grant[]; dataPolicy?: PolicyRef }
type RoleSummary = { roleId: string; roleName: string; roleStatus: string }
type Member = { userId: string; username: string; email: string; status: string; version: number; name: string; phone: string; employeeId: string; position: string; departmentId: string; roles: RoleSummary[]; derivedDataScope: string }
type Scope = { userId: string; version: number; siteIds: string[]; tenantId: string }
type Write = { path: string; method: string; headers: Record<string,string>; body: unknown }
type Options = { policyConflict?: boolean; policyReadbackFailOnce?: boolean; scopeConflict?: boolean; scopeReadbackFailOnce?: boolean }

const activeRole='TENANT_ROLE_STATUS_ACTIVE'
const activeMember='TENANT_MEMBER_STATUS_ACTIVE'
function json(route:Route,status:number,body:unknown){return route.fulfill({status,contentType:'application/json',body:JSON.stringify(body)})}
function summary(role:Role):RoleSummary{return{roleId:role.id,roleName:role.name,roleStatus:role.status}}

async function mockPermissionServer(page:Page,options:Options={}){
  const policies:Policy[]=[
    {id:'policy-east',name:'华东运营点位',status:'active',siteIds:['site-a'],version:3,notBefore:'',expiresAt:'',effective:true,invalidReason:''},
    {id:'policy-core',name:'核心经营点位',status:'active',siteIds:['site-a','site-b'],version:5,notBefore:'',expiresAt:'',effective:true,invalidReason:''},
    {id:'policy-retired',name:'已撤销策略',status:'revoked',siteIds:['site-c'],version:7,notBefore:'',expiresAt:'',effective:false,invalidReason:'revoked'},
  ]
  let roles:Role[]=[
    {id:'tenant-001:owner',name:'owner',description:'',roleCode:'tenant_owner',systemRole:true,memberCount:1,status:activeRole,version:1,permissions:[{permission:'tenant.role.manage',scope:'DATA_SCOPE_ALL'},{permission:'tenant.role.read',scope:'DATA_SCOPE_ALL'}]},
    {id:'role-ops',name:'运营负责人',description:'负责日常运营',roleCode:'',systemRole:false,memberCount:1,status:activeRole,version:4,permissions:[{permission:'tenant.member.read',scope:'DATA_SCOPE_SITES'}],dataPolicy:{policyId:'policy-east',policyName:'华东运营点位',policyVersion:3,acceptedVersion:3,effective:true,invalidReason:''}},
  ]
  let members:Member[]=[
    {userId:'user-001',username:'alice.owner',email:'alice@coffeelink.test',status:activeMember,version:4,name:'Alice Chen',phone:'',employeeId:'EMP-001',position:'运营负责人',departmentId:'dept-ops',roles:[summary(roles[1]!)],derivedDataScope:'sites'},
  ]
  let scope:Scope={userId:'user-001',version:4,siteIds:['site-a'],tenantId:'tenant-001'}
  const candidates=[
    {id:'site-a',name:'上海旗舰点位',version:2,assignable:true,unavailableReason:''},
    {id:'site-b',name:'杭州办公点位',version:1,assignable:true,unavailableReason:''},
    {id:'site-retired',name:'已停用点位',version:8,assignable:false,unavailableReason:'点位已不可分配'},
  ]
  const writes:Write[]=[]
  let policyReadbackFailure=Boolean(options.policyReadbackFailOnce)
  let scopeReadbackFailure=Boolean(options.scopeReadbackFailOnce)
  let policyWriteSeen=false,scopeWriteSeen=false
  const record=(route:Route)=>{const request=route.request();writes.push({path:new URL(request.url()).pathname.replace(/^\/api(?=\/)/,''),method:request.method(),headers:request.headers(),body:request.postDataJSON()})}

  await page.route(/\/(?:api\/)?(?:auth|v1)\//,async route=>{throw new Error(`Unhandled permission UI request: ${route.request().method()} ${new URL(route.request().url()).pathname}`)})
  await page.route(/\/(?:api\/)?auth\/login(?:\?.*)?$/,async route=>route.fulfill({status:200,contentType:'text/html',body:'<!doctype html><title>Login</title>'}))
  await page.route(/\/(?:api\/)?auth\/session(?:\?.*)?$/,async route=>json(route,200,{authenticated:true,actor_kind:'tenant',user_id:'user-admin',active_tenant_id:'tenant-001',context_version:11,csrf_token:'csrf-data-permission',tenants:[{id:'tenant-001',name:'CoffeeLink 测试租户'}]}))
  await page.route(/\/(?:api\/)?auth\/authorization(?:\?.*)?$/,async route=>{
    const buttonCodes=['tenant.role.list','tenant.role.get','tenant.role.update','tenant.role.set_permissions','tenant.data_policy.list','tenant.role.set_data_policy','tenant.member.list','tenant.member.get','tenant.member.business_scope.get','tenant.member.business_scope.set','tenant.member.scope_candidates','tenant.department.list']
    return json(route,200,{authenticated:true,actor_kind:'tenant',user_id:'user-admin',tenant_id:'tenant-001',roles:['owner'],grants:[{permission:'tenant.role.manage',role_id:'tenant-001:owner',role_name:'owner',scope:'all'}],data_policies:[],site_ids:[],permission_version:'sha256:data-permission-ui',modules:[{code:'access-management',allowed:true,reason:'allowed',actions:buttonCodes}],actions:buttonCodes.map(code=>({code,permissions:[],permission_mode:'all'})),button_codes:buttonCodes})
  })
  await page.route(/\/(?:api\/)?auth\/action-catalog(?:\?.*)?$/,async route=>json(route,200,{schema_version:'v1',actions:[],permissions:[]}))
  await page.route(/\/(?:api\/)?v1\/tenant\/data-policies(?:\?.*)?$/,async route=>json(route,200,{policies}))
  await page.route(/\/(?:api\/)?v1\/tenant\/departments(?:\?.*)?$/,async route=>json(route,200,{departments:[{departmentId:'dept-ops',name:'租赁运营部',parentId:'',leaderUserId:'user-admin',email:'',phone:'',status:'TENANT_DEPARTMENT_STATUS_ACTIVE',sort:10,version:1}]}))
  await page.route(/\/(?:api\/)?v1\/tenant\/member-scope-candidates(?:\?.*)?$/,async route=>json(route,200,{candidates,total:candidates.length}))
  await page.route(/\/(?:api\/)?v1\/tenant\/members\/([^/]+)\/business-scope(?:\?.*)?$/,async route=>{
    const request=route.request()
    if(request.method()==='GET'){
      if(scopeWriteSeen&&scopeReadbackFailure){scopeReadbackFailure=false;return json(route,500,{message:'scope readback unavailable'})}
      return json(route,200,scope)
    }
    record(route)
    if(options.scopeConflict)return json(route,409,{message:'member scope version conflict'})
    const body=request.postDataJSON() as {userId:string;version:number;siteIds:string[]}
    scope={userId:body.userId,version:body.version+1,siteIds:[...body.siteIds].sort(),tenantId:'tenant-001'}
    members=members.map(member=>member.userId===body.userId?{...member,version:scope.version}:member)
    scopeWriteSeen=true
    return json(route,200,scope)
  })
  await page.route(/\/(?:api\/)?v1\/tenant\/members(?:\?.*)?$/,async route=>json(route,200,{members,total:members.length}))
  await page.route(/\/(?:api\/)?v1\/tenant\/members\/(?!removed(?:[/?]|$))[^/?]+(?:\?.*)?$/,async route=>{
    const id=decodeURIComponent(new URL(route.request().url()).pathname.split('/').filter(Boolean).at(-1)??'')
    const member=members.find(item=>item.userId===id)
    return member?json(route,200,member):json(route,404,{message:'member not found'})
  })
  await page.route(/\/(?:api\/)?v1\/tenant\/roles(?:\?.*)?$/,async route=>json(route,200,{roles}))
  await page.route(/\/(?:api\/)?v1\/tenant\/roles\/([^/]+)(?:\/data-policy)?(?:\?.*)?$/,async route=>{
    const request=route.request(),parts=new URL(request.url()).pathname.split('/').filter(Boolean),roleId=decodeURIComponent(parts[parts.indexOf('roles')+1]??''),action=parts.at(-1)
    const role=roles.find(item=>item.id===roleId)
    if(!role)return json(route,404,{message:'role not found'})
    if(request.method()==='GET'){
      if(policyWriteSeen&&policyReadbackFailure){policyReadbackFailure=false;return json(route,500,{message:'role readback unavailable'})}
      return json(route,200,role)
    }
    if(request.method()==='PUT'&&action==='data-policy'){
      record(route)
      if(options.policyConflict)return json(route,409,{message:'role version conflict'})
      const body=request.postDataJSON() as {policyId:string;policyVersion:number}
      const policy=policies.find(item=>item.id===body.policyId)
      const next:Role={...role,version:role.version+1,dataPolicy:policy?{policyId:policy.id,policyName:policy.name,policyVersion:policy.version,acceptedVersion:body.policyVersion,effective:policy.effective,invalidReason:policy.invalidReason}:undefined}
      roles=roles.map(item=>item.id===roleId?next:item);policyWriteSeen=true;return json(route,200,next)
    }
    return json(route,400,{message:'unsupported role operation'})
  })
  return{getWrites:()=>writes,getScope:()=>scope,getRoles:()=>roles}
}
async function openRoles(page:Page){await page.goto('/#/enterprise/roles');await expect(page.locator('[data-enterprise-page="roles"]')).toBeVisible()}
async function openMemberScope(page:Page){
  await page.goto('/#/enterprise/members');await expect(page.locator('[data-enterprise-page="members"]')).toBeVisible()
  await page.getByRole('button',{name:'查看 Alice Chen',exact:true}).click()
  const drawer=page.getByRole('dialog',{name:'成员详情',exact:true})
  await drawer.getByRole('tab',{name:'数据权限',exact:true}).click()
  await expect(drawer.locator('[data-member-business-scope]')).toBeVisible()
  return drawer
}
function roleRow(page:Page){return page.locator('tbody tr').filter({hasText:'运营负责人'})}

test('role policy and member explicit scope render at all CoffeeLink acceptance viewports',async({page})=>{
  await mockPermissionServer(page);mkdirSync('screenshots/enterprise180-permission-ui',{recursive:true})
  for(const viewport of [{width:1366,height:768},{width:1440,height:900},{width:1536,height:1024},{width:390,height:844}]){
    await page.setViewportSize(viewport);await openRoles(page)
    await expect(roleRow(page)).toContainText('华东运营点位')
    await roleRow(page).getByRole('button',{name:'数据策略 运营负责人'}).click()
    const policyDialog=page.getByRole('dialog',{name:'运营负责人 · 数据策略'})
    await expect(policyDialog).toContainText('当前版本 v3')
    await expect(policyDialog).toContainText('绑定版本 v3')
    expect(await page.evaluate(()=>document.documentElement.scrollWidth)).toBe(viewport.width)
    await page.screenshot({path:`screenshots/enterprise180-permission-ui/role-policy-${viewport.width}.png`})
    await policyDialog.getByRole('button',{name:'关闭',exact:true}).click()
    const drawer=await openMemberScope(page)
    await expect(drawer).toContainText('来源：Access 成员显式范围')
    await expect(drawer).toContainText('已授权 1 个点位')
    expect(await page.evaluate(()=>document.documentElement.scrollWidth)).toBe(viewport.width)
    await page.screenshot({path:`screenshots/enterprise180-permission-ui/member-scope-${viewport.width}.png`})
  }
})

test('role policy binding disables invalid policy and confirms authoritative readback before success',async({page})=>{
  const server=await mockPermissionServer(page);await openRoles(page)
  await roleRow(page).getByRole('button',{name:'数据策略 运营负责人'}).click()
  const dialog=page.getByRole('dialog',{name:'运营负责人 · 数据策略'})
  await expect(dialog.getByRole('option',{name:/已撤销策略/})).toBeDisabled()
  await selectUiOption(dialog.getByLabel('Data Policy 约束'),'policy-core')
  await dialog.getByRole('button',{name:'保存策略引用'}).click()
  await expect(page.getByRole('status')).toContainText('角色数据策略已保存并确认')
  await expect(roleRow(page)).toContainText('核心经营点位')
  const writes=server.getWrites().filter(item=>item.path.endsWith('/role-ops/data-policy'))
  expect(writes).toHaveLength(1)
  expect(writes[0]?.body).toMatchObject({roleId:'role-ops',version:4,policyId:'policy-core',policyVersion:5})
  expect(writes[0]?.headers['idempotency-key']).toMatch(/^enterprise-role-data-policy-/)
})

test('role policy write with failed readback stays unconfirmed and recovery does not write twice',async({page})=>{
  const server=await mockPermissionServer(page,{policyReadbackFailOnce:true});await openRoles(page)
  await roleRow(page).getByRole('button',{name:'数据策略 运营负责人'}).click()
  const dialog=page.getByRole('dialog',{name:'运营负责人 · 数据策略'})
  await selectUiOption(dialog.getByLabel('Data Policy 约束'),'policy-core')
  await dialog.getByRole('button',{name:'保存策略引用'}).click()
  await expect(dialog.getByRole('alert')).toContainText('暂时无法确认最终状态')
  await expect(page.getByText('角色数据策略已保存并确认。',{exact:true})).toHaveCount(0)
  await dialog.getByRole('button',{name:'重新读取服务端状态'}).click()
  await expect(page.getByRole('status')).toContainText('角色数据策略已保存并确认')
  expect(server.getWrites().filter(item=>item.path.endsWith('/role-ops/data-policy'))).toHaveLength(1)
})

test('role policy CAS conflict refreshes authority and keeps editor open without fake success',async({page})=>{
  const server=await mockPermissionServer(page,{policyConflict:true});await openRoles(page)
  await roleRow(page).getByRole('button',{name:'数据策略 运营负责人'}).click()
  const dialog=page.getByRole('dialog',{name:'运营负责人 · 数据策略'})
  await selectUiOption(dialog.getByLabel('Data Policy 约束'),'policy-core')
  await dialog.getByRole('button',{name:'保存策略引用'}).click()
  await expect(dialog.getByRole('alert')).toContainText('已刷新服务端状态')
  await expect(dialog.getByLabel('Data Policy 约束')).toHaveValue('policy-east')
  await expect(page.getByText('角色数据策略已保存并确认。',{exact:true})).toHaveCount(0)
  expect(server.getWrites().filter(item=>item.path.endsWith('/role-ops/data-policy'))).toHaveLength(1)
})

test('member explicit scope uses current candidates, CAS and authoritative readback',async({page})=>{
  const server=await mockPermissionServer(page);const drawer=await openMemberScope(page)
  await drawer.getByRole('button',{name:'调整范围'}).click()
  const dialog=page.getByRole('dialog',{name:'Alice Chen · 数据权限'})
  await expect(dialog.getByText('已停用点位')).toBeVisible()
  await expect(dialog.getByText('点位已不可分配')).toBeVisible()
  const retired=dialog.getByText('已停用点位').locator('..').getByRole('checkbox')
  await expect(retired).toBeDisabled()
  await dialog.getByText('杭州办公点位').locator('..').getByRole('checkbox').check()
  await dialog.getByRole('button',{name:'保存数据权限'}).click()
  await expect(page.getByRole('status')).toContainText('成员数据权限已保存并确认')
  expect(server.getScope().siteIds).toEqual(['site-a','site-b'])
  const writes=server.getWrites().filter(item=>item.path.endsWith('/user-001/business-scope'))
  expect(writes).toHaveLength(1)
  expect(writes[0]?.body).toMatchObject({userId:'user-001',version:4,siteIds:['site-a','site-b']})
  expect(writes[0]?.headers['idempotency-key']).toMatch(/^enterprise-member-scope-/)
})

test('member scope readback recovery and CAS conflict never manufacture confirmed success',async({page})=>{
  let server=await mockPermissionServer(page,{scopeReadbackFailOnce:true});let drawer=await openMemberScope(page)
  await drawer.getByRole('button',{name:'调整范围'}).click()
  let dialog=page.getByRole('dialog',{name:'Alice Chen · 数据权限'})
  await dialog.getByText('杭州办公点位').locator('..').getByRole('checkbox').check()
  await dialog.getByRole('button',{name:'保存数据权限'}).click()
  await expect(dialog.getByRole('alert')).toContainText('暂时无法确认最终状态')
  await dialog.getByRole('button',{name:'重新读取服务端状态'}).click()
  await expect(page.getByRole('status')).toContainText('成员数据权限已保存并确认')
  expect(server.getWrites().filter(item=>item.path.endsWith('/user-001/business-scope'))).toHaveLength(1)

  await page.unrouteAll({behavior:'ignoreErrors'})
  server=await mockPermissionServer(page,{scopeConflict:true});drawer=await openMemberScope(page)
  await drawer.getByRole('button',{name:'调整范围'}).click();dialog=page.getByRole('dialog',{name:'Alice Chen · 数据权限'})
  await dialog.getByText('杭州办公点位').locator('..').getByRole('checkbox').check()
  await dialog.getByRole('button',{name:'保存数据权限'}).click()
  await expect(dialog.getByRole('alert')).toContainText('已刷新权威范围')
  await expect(page.getByText('成员数据权限已保存并确认。',{exact:true})).toHaveCount(0)
  expect(server.getWrites().filter(item=>item.path.endsWith('/user-001/business-scope'))).toHaveLength(1)
})
