from pathlib import Path

path = Path('web/src/features/platform/pages/PlatformTenantsView.vue')
source = path.read_text(encoding='utf-8')


def replace_once(old: str, new: str, label: str) -> None:
    global source
    count = source.count(old)
    if count != 1:
        raise SystemExit(f'{label}: expected exactly one match, found {count}')
    source = source.replace(old, new, 1)


replace_once(
    "import AppIcon from '@/ui/common/AppIcon.vue'\n",
    "import AppIcon from '@/ui/common/AppIcon.vue'\nimport AppPagination from '@/ui/common/AppPagination.vue'\n",
    'pagination import',
)
replace_once('const pageSize=10; let epoch=0', 'const pageSize=ref(10); let epoch=0', 'page size ref')
replace_once(
    'const totalPages=computed(()=>Math.max(1,Math.ceil(filtered.value.length/pageSize)))',
    'const totalPages=computed(()=>Math.max(1,Math.ceil(filtered.value.length/pageSize.value)))',
    'total pages',
)
replace_once(
    'const visibleTenants=computed(()=>{const safe=Math.min(page.value,totalPages.value),start=(safe-1)*pageSize;return filtered.value.slice(start,start+pageSize)})',
    'const visibleTenants=computed(()=>{const safe=Math.min(page.value,totalPages.value),start=(safe-1)*pageSize.value;return filtered.value.slice(start,start+pageSize.value)})',
    'visible tenants',
)
replace_once(
    "const firstVisible=computed(()=>filtered.value.length?(Math.min(page.value,totalPages.value)-1)*pageSize+1:0)\nconst lastVisible=computed(()=>filtered.value.length?Math.min(filtered.value.length,firstVisible.value+visibleTenants.value.length-1):0)\n",
    '',
    'legacy pagination summary',
)
replace_once(
    'Math.ceil(filtered.value.length/pageSize)))}return token===epoch',
    'Math.ceil(filtered.value.length/pageSize.value)))}return token===epoch',
    'refresh page clamp',
)
replace_once(
    "        <div class=\"pagination\" data-ui-region=\"pagination\" :aria-label=\"t('tenants.pagination')\"><span>{{ t('tenants.paginationSummary',{first:firstVisible,last:lastVisible,total:filtered.length}) }}</span><div class=\"pagination-actions\"><UiButton class=\"icon-button\" type=\"button\" :aria-label=\"t('common.previousPage')\" :disabled=\"page<=1\" @click=\"page--\"><AppIcon name=\"left\" :size=\"16\"/></UiButton><strong>{{ Math.min(page,totalPages) }} / {{ totalPages }}</strong><UiButton class=\"icon-button\" type=\"button\" :aria-label=\"t('common.nextPage')\" :disabled=\"page>=totalPages\" @click=\"page++\"><AppIcon name=\"right\" :size=\"16\"/></UiButton></div></div>",
    '        <AppPagination v-model:page="page" v-model:page-size="pageSize" data-ui-region="pagination" :total="filtered.length" />',
    'pagination markup',
)
replace_once(
    '.pagination{display:flex;align-items:center;justify-content:space-between;gap:12px;padding-top:14px;color:var(--color-text-muted);font-size:12px}.pagination-actions{display:flex;align-items:center;gap:8px}.pagination-actions strong{min-width:52px;text-align:center;color:var(--color-text-secondary)}',
    '',
    'legacy pagination css',
)
replace_once('.pagination{align-items:flex-start}', '', 'legacy mobile pagination css')

if '<div class="pagination"' in source or 'firstVisible' in source or 'lastVisible' in source:
    raise SystemExit('legacy pagination implementation remains')
if source.count('data-ui-region="pagination"') != 1:
    raise SystemExit('pagination region must remain unique')

path.write_text(source, encoding='utf-8')
print('REGISTRY_FIRST_PATCH=PASS tenant-pagination=AppPagination members-code-change=0')
