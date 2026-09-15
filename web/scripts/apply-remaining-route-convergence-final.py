from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
path = ROOT / 'web/e2e/enterprise-plan-change-real.spec.ts'
text = path.read_text(encoding='utf-8')
old = "    await expect(page.locator('[data-plan-change-receipt]')).toContainText('chg-tenant-preview-001')"
new = "    await expect(page.locator('[data-plan-change-receipt]')).toContainText(status)\n    await expect(page.locator('[data-plan-change-lifecycle]')).toContainText('chg-tenant-preview-001')"
if old not in text:
    raise RuntimeError('missing final plan receipt assertion anchor')
path.write_text(text.replace(old, new, 1), encoding='utf-8')
print('Final plan receipt assertion aligned with canonical receipt layout.')
