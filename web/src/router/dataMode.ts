export function routeContentEnabled(previewMode: boolean, surface: unknown): boolean {
  return previewMode || surface === 'platform' || surface === 'runtime'
}
