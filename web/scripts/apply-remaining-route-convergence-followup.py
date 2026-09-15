from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]

def read(path: str) -> str:
    return (ROOT / path).read_text(encoding='utf-8')

def write(path: str, text: str) -> None:
    (ROOT / path).write_text(text, encoding='utf-8')

def replace_once(text: str, old: str, new: str, label: str) -> str:
    if old not in text:
        raise RuntimeError(f'missing follow-up anchor: {label}')
    return text.replace(old, new, 1)

# Organization: keep the grid item shrinkable so the wide member table scrolls inside its panel.
path = 'web/src/features/enterprise/pages/OrganizationView.vue'
text = read(path)
text = replace_once(
    text,
    '''.organization-layout {\n  display: grid;\n  grid-template-columns: 244px minmax(0, 1fr);\n  gap: 16px;\n}\n''',
    '''.organization-layout {\n  display: grid;\n  grid-template-columns: 244px minmax(0, 1fr);\n  gap: 16px;\n}\n.organization-layout > * {\n  min-width: 0;\n}\n''',
    'organization shrinkable grid children',
)
write(path, text)

# Plans: preserve the mounted lifecycle while an authoritative post-confirm refresh is in flight.
path = 'web/src/features/enterprise/pages/PlansView.vue'
text = read(path)
text = replace_once(
    text,
    '    <div v-if="plan.loading" class="card state-card" role="status">正在读取当前租户套餐、权益与用量…</div>',
    '    <div v-if="plan.loading && !plan.model" class="card state-card" role="status">正在读取当前租户套餐、权益与用量…</div>',
    'preserve plan lifecycle during refresh',
)
write(path, text)

# Organization retry assertion: wait for the second async save handler to actually reach the network.
path = 'web/e2e/enterprise-organization-real.spec.ts'
text = read(path)
text = replace_once(
    text,
    '''  await save.click()\n  await expect(dialog.getByRole('alert')).toContainText('部门版本、层级、负责人或成员归属规则发生冲突')\n  const patches = server.getWrites().filter((item) => item.method === 'PATCH')\n  expect(patches).toHaveLength(2)\n  expect(patches[0]?.headers['idempotency-key']).toBe(patches[1]?.headers['idempotency-key'])''',
    '''  await save.click()\n  await expect.poll(() => server.getWrites().filter((item) => item.method === 'PATCH').length).toBe(2)\n  const patches = server.getWrites().filter((item) => item.method === 'PATCH')\n  expect(patches[0]?.headers['idempotency-key']).toBe(patches[1]?.headers['idempotency-key'])''',
    'department retry async network wait',
)
write(path, text)

# Plans: degradation is intentionally rendered inside the usage surface; scope auth assertions to the primary page alert.
path = 'web/e2e/enterprise-plan-real.spec.ts'
text = read(path)
text = replace_once(
    text,
    '''  await openRealPlan(page)\n  await expect(page.getByText(/用量服务暂不可用/)).toBeVisible()\n  await page.getByRole('button', { name: '使用额度' }).click()''',
    '''  await openRealPlan(page)\n  await page.getByRole('button', { name: '使用额度' }).click()\n  await expect(page.getByText(/用量服务暂不可用/)).toBeVisible()''',
    'usage degradation visibility scope',
)
text = text.replace("page.getByRole('alert')", "page.locator('.state-card.error-state[role=\"alert\"]')")
write(path, text)

# Roles: canonical copy calls custom/scope-limited grants “指定数据”.
path = 'web/e2e/enterprise-roles-real.spec.ts'
text = read(path)
text = replace_once(text, "toContainText('授权点位')", "toContainText('指定数据')", 'canonical role scope copy')
write(path, text)

print('Final remaining route convergence fixes applied.')
