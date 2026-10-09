import { expect, type Page, type TestInfo } from '@playwright/test'

type MemberDTO = {
  userId: string
  username: string
  name: string
  status: string
  version: string | number
  roles: Array<{ roleId: string; roleName: string }>
}
type CreationReceipt = {
  member: MemberDTO
  activationMode: string
  notificationEventId: string
  deliveryState: string
  maskedDestination: string
}

// Reuse the existing, already entitled Access sample and its trusted admin.
// All three mutations are performed by the existing product UI. No API mock,
// database write, new identity fixture, activation bypass or credential output.
export async function verifyMemberManagementUI(
  page: Page,
  baseURL: string,
  initialRoleName: string,
  testInfo: TestInfo,
) {
  const suffix = Date.now().toString(36)
  const username = `m360.${suffix}`
  const name = `基础成员 Alice-${suffix}`
  const editedName = `基础成员 Bob-${suffix}`
  const viewerName = `只读成员 Viewer-${suffix}`
  await page.goto(`${baseURL}/#/enterprise/roles`)
  // The preceding Access sample changes the same session's tenant via raw
  // API calls to test stale-context rejection. Start this independent UI
  // task with a fresh document, retaining the real authenticated cookie.
  // Do not reuse the stale SPA store or override its authorization projection.
  await page.reload()
  await page.getByRole('button', { name: '新建角色', exact: true }).click()
  const roleDialog = page.getByRole('dialog', { name: '新建角色' })
  await roleDialog.getByLabel('角色名称').fill(viewerName)
  await roleDialog.locator('[data-role-permission-leaf="tenant.member.read"]').getByRole('checkbox').check()
  await roleDialog.getByLabel('数据范围').click()
  await page.locator('[data-slot="select-item"][data-ui-option-value="all"]').click()
  await roleDialog.getByRole('button', { name: '保存角色', exact: true }).click()
  await expect(roleDialog).not.toBeVisible()
  await expect(page.getByText(viewerName, { exact: true })).toBeVisible()

  await page.goto(`${baseURL}/#/enterprise/members`)
  await expect(page.locator('[data-enterprise-page="members"]')).toBeVisible()
  await page.getByRole('button', { name: '添加成员', exact: true }).click()
  const create = page.getByRole('dialog', { name: '新增成员', exact: true })
  await create.getByLabel('登录账号').fill(username)
  await create.getByLabel('姓名', { exact: true }).fill(name)
  await create.getByLabel('邮箱', { exact: true }).fill(`${username}@example.invalid`)
  await create.getByLabel('激活方式').click()
  await page.locator('[data-slot="select-item"][data-ui-option-value="activation_link"]').click()
  for (const checkbox of await create.getByRole('checkbox').all()) await checkbox.uncheck()
  await create.getByLabel(initialRoleName, { exact: true }).check()
  const createdResponse = page.waitForResponse((response) =>
    new URL(response.url()).pathname === '/api/v1/tenant/members/create' && response.request().method() === 'POST')
  await create.getByRole('button', { name: '保存变更', exact: true }).click()
  const response = await createdResponse
  expect(response.status(), await response.text()).toBe(200)
  const receipt = await response.json() as CreationReceipt
  expect(receipt.member).toMatchObject({ username, name, status: 'TENANT_MEMBER_STATUS_INVITED' })
  expect(receipt.notificationEventId).toBeTruthy()
  // Accepted activation delivery is not activation success or delivered mail.
  expect(receipt).not.toHaveProperty('password')
  expect(receipt).not.toHaveProperty('activationToken')
  const memberID = receipt.member.userId
  expect(memberID).toBeTruthy()
  await expect(create).not.toBeVisible()
  const row = page.locator(`[data-member-id="${memberID}"]`)
  await expect(row).toContainText(name)
  await expect(row).toContainText('待激活')
  await expect(row).toContainText(initialRoleName)

  await row.getByRole('button', { name: `编辑 ${name}`, exact: true }).click()
  const edit = page.getByRole('dialog', { name: '修改成员信息', exact: true })
  await expect(edit.getByLabel('登录账号')).toHaveValue(username)
  await expect(edit.getByLabel('登录账号')).toHaveAttribute('readonly')
  await edit.getByLabel('姓名', { exact: true }).fill(editedName)
  const updatedResponse = page.waitForResponse((res) =>
    new URL(res.url()).pathname === `/api/v1/tenant/members/${memberID}` && res.request().method() === 'PATCH')
  await edit.getByRole('button', { name: '保存变更', exact: true }).click()
  const updated = await updatedResponse
  expect(updated.status(), await updated.text()).toBe(200)
  expect(await updated.json()).toMatchObject({ userId: memberID, username, name: editedName })
  await expect(edit).not.toBeVisible()
  await expect(row).toContainText(editedName)

  await row.getByRole('button', { name: `${editedName} 更多操作`, exact: true }).click()
  await page.getByRole('button', { name: '角色与数据权限变更', exact: true }).click()
  const grant = page.getByRole('dialog', { name: '角色变更与权限调整', exact: true })
  await grant.getByLabel(viewerName, { exact: true }).check()
  await grant.getByLabel(initialRoleName, { exact: true }).uncheck()
  const grantedResponse = page.waitForResponse((res) =>
    new URL(res.url()).pathname === `/api/v1/tenant/members/${memberID}` && res.request().method() === 'PATCH')
  await grant.getByRole('button', { name: '保存变更', exact: true }).click()
  const granted = await grantedResponse
  expect(granted.status(), await granted.text()).toBe(200)
  const member = await granted.json() as MemberDTO
  expect(member.roles.map((role) => role.roleName)).toEqual([viewerName])
  expect(member.status).toBe('TENANT_MEMBER_STATUS_INVITED')
  await expect(grant).not.toBeVisible()
  await page.reload()
  await expect(row).toContainText(editedName)
  await expect(row).toContainText(viewerName)
  await expect(row).not.toContainText(initialRoleName)
  await expect(row).toContainText('待激活')
  // Actual viewport captures are review material, not independent Human UX approval.
  for (const [width, height] of [[1366, 768], [1440, 900], [1536, 1024], [390, 844]] as const) {
    await page.setViewportSize({ width, height })
    await expect(row).toBeVisible()
    await page.screenshot({ path: testInfo.outputPath(`foundation-member-${width}x${height}.png`), fullPage: true })
  }
}
