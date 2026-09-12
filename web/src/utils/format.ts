export const formatNumber = (value: number) => new Intl.NumberFormat('zh-CN').format(value)
export const timestamp = () => new Date().toLocaleString('sv-SE', { timeZone: 'Asia/Shanghai' })
export const csvCell = (value: unknown) => {
  const text = String(value ?? '')
  return '"' + (/^[=+@\-\t\r]/.test(text) ? "'" + text : text).replaceAll('"', '""') + '"'
}
export function downloadCsv(name: string, rows: unknown[][]) {
  const blob = new Blob(['\uFEFF' + rows.map((r) => r.map(csvCell).join(',')).join('\r\n')], {
    type: 'text/csv;charset=utf-8',
  })
  const link = document.createElement('a')
  const url = URL.createObjectURL(blob)
  link.href = url
  link.download = name
  link.click()
  setTimeout(() => URL.revokeObjectURL(url), 1000)
}
