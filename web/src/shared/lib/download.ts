export function downloadText(
  name: string,
  content: string,
  type = "text/plain;charset=utf-8",
): void {
  const url = URL.createObjectURL(new Blob([content], { type }));
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = name;
  anchor.click();
  window.setTimeout(() => URL.revokeObjectURL(url), 1000);
}
export function csvCell(value: string): string {
  return `"${value.replace(/^[\s\u0000-\u001f]*[=+@-]/, "'$&").replaceAll('"', '""')}"`;
}
